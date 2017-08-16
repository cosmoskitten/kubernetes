/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/version"
)

// applyFlags holds the information about the flags that can be passed to apply
type applyFlags struct {
	nonInteractiveMode, force, dryRun bool
	newK8sVersionStr                  string
	newK8sVersion                     *version.Version
	pullTimeout                       time.Duration
	parent                            *cmdUpgradeFlags
}

// NewCmdApply returns the cobra command for `kubeadm upgrade apply`
func NewCmdApply(parentFlags *cmdUpgradeFlags) *cobra.Command {
	flags := &applyFlags{
		parent:      parentFlags,
		pullTimeout: 15 * time.Minute,
	}

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the upgrade",
		Run: func(cmd *cobra.Command, args []string) {
			kubeadmutil.CheckErr(ValidateFlags(flags))

			// Default the flags
			SetDefaults(flags)

			err := RunApply(flags)
			kubeadmutil.CheckErr(err)
		},
	}

	// Specify the valid flags specific for apply
	cmd.Flags().BoolVarP(&flags.nonInteractiveMode, "yes", "y", flags.nonInteractiveMode, "Perform the upgrade and do not prompt for confirmation (non-interactive mode)")
	cmd.Flags().BoolVarP(&flags.force, "force", "f", flags.force, "Force upgrading although some requirements might not be met. This also implies non-interactive mode")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", flags.dryRun, "Do not change any state, just output what actions would be applied. This also implies non-interactive mode")
	cmd.Flags().StringVar(&flags.newK8sVersionStr, "version", flags.newK8sVersionStr, "Selected version to upgrade to")
	cmd.Flags().DurationVar(&flags.pullTimeout, "pull-timeout", flags.pullTimeout, "The maximum amount of time to wait for the control plane pods to be downloaded")

	return cmd
}

// RunApply takes care of the actual upgrade functionality
func RunApply(flags *applyFlags) error {

	// Start with the basics, verify that the cluster is healthy and get the configuration from the cluster (using the ConfigMap)
	client, cfg, err := EnforceRequirements(flags.parent.kubeConfigPath, flags.parent.cfgPath, flags.parent.printConfig, flags.nonInteractiveMode)
	if err != nil {
		return err
	}

	// Set the upgraded version on the config object now
	cfg.KubernetesVersion = flags.newK8sVersionStr

	// Grab the external, versioned configuration and convert it to the internal type for usage here later
	internalcfg := &kubeadmapi.MasterConfiguration{}
	api.Scheme.Convert(cfg, internalcfg, nil)

	// Enforce all the version skew policies there are
	if err := EnforceVersionPolicies(flags, client); err != nil {
		return err
	}

	// If interactive mode is set (--yes was not specified), ask ther user whether they really want to upgrade
	// If --yes is specified; just proceed and do it
	if !flags.nonInteractiveMode {
		if err := upgrade.InteractivelyConfirmUpgrade("Are you sure you want to proceed with the upgrade"); err != nil {
			return err
		}
	}

	// Use a prepuller implementation based on creating DaemonSets
	// and block until all DaemonSets are ready; then we know for sure that all control plane images are cached locally
	prepuller := upgrade.NewDaemonSetPrepuller(client, internalcfg)
	upgrade.PrepullImagesInParallel(prepuller, flags.pullTimeout)

	// Now; perform the upgrade procedure
	if err := PerformUpgrade(flags, client, internalcfg); err != nil {
		return err
	}

	// Upgrade RBAC rules and addons. Optionally, if needed, perform some extra task for a specific version
	if err := upgrade.PerformPostUpgradeTasks(client, internalcfg, flags.newK8sVersion); err != nil {
		return err
	}

	fmt.Println("")
	fmt.Printf("[upgrade/successful] SUCCESS! Your cluster was upgraded to %q. Enjoy!\n", flags.newK8sVersionStr)
	fmt.Println("")
	fmt.Println("[upgrade/kubelet] Now that your control plane is upgraded, please proceed with upgrading your kubelets in turn.")

	return nil
}

// ValidateFlags makes sure the arguments given to apply are valid
// TODO: Add unit tests for this and negative cmd functional tests
func ValidateFlags(flags *applyFlags) error {
	if len(flags.newK8sVersionStr) == 0 {
		return fmt.Errorf("--version is a required parameter. Please specify the version you want to upgrade to")
	}
	var err error
	flags.newK8sVersion, err = version.ParseSemantic(flags.newK8sVersionStr)
	if err != nil {
		return fmt.Errorf("couldn't parse version %q as a semantic version", flags.newK8sVersionStr)
	}
	return nil
}

// SetDefaults handles dynamically defaulting flags based on each other's value
func SetDefaults(flags *applyFlags) {
	// If we are in dry-run or force mode; we should automatically execute this command non-interactively
	if flags.dryRun || flags.force {
		flags.nonInteractiveMode = true
	}
}

// EnforceVersionPolicies makes sure that the version the user specified is valid to upgrade to
// There are both fatal and skippable (with --force) errors
func EnforceVersionPolicies(flags *applyFlags, client clientset.Interface) error {
	fmt.Printf("[upgrade/version] You have chosen to upgrade to version %q\n", flags.newK8sVersionStr)
	versionGetterImpl := upgrade.NewKubeVersionGetter(client, os.Stdout)
	versionSkewErrs := upgrade.EnforceVersionPolicies(versionGetterImpl, flags.newK8sVersionStr, flags.newK8sVersion, flags.parent.allowExperimentalUpgrades, flags.parent.allowRCUpgrades)
	if versionSkewErrs != nil {

		if len(versionSkewErrs.Mandatory) > 0 {
			fmt.Printf("[upgrade/version] FATAL: The --version argument is invalid due to these fatal errors: %v.\n", versionSkewErrs.Mandatory)
			return fmt.Errorf("The --version argument is invalid")
		}

		if len(versionSkewErrs.Skippable) > 0 {
			fmt.Printf("[upgrade/version] ERROR: The --version argument is invalid due to these fatal errors: %v.\n", versionSkewErrs.Skippable)
			fmt.Println("[upgrade/version] Can be bypassed if you pass the --force flag")

			if !flags.force {
				return fmt.Errorf("The --version argument is invalid")
			}
			fmt.Printf("[upgrade/version] Found %d potential compatibility errors but skipping since the --force flag is set\n", len(versionSkewErrs.Skippable))
		}
	}
	return nil
}

// PerformUpgrade actually performs the upgrade procedure for the cluster of your type (self-hosted or static-pod-hosted)
func PerformUpgrade(flags *applyFlags, client clientset.Interface, internalcfg *kubeadmapi.MasterConfiguration) error {

	// Check if the cluster is self-hosted and act accordingly
	if upgrade.IsControlPlaneSelfHosted(client) {
		fmt.Printf("[upgrade/apply] Upgrading your Self-Hosted control plane to version %q...\n", flags.newK8sVersionStr)

		// Upgrade a self-hosted cluster
		// TODO(luxas): Implement this later when we have the new upgrade strategy
		return fmt.Errorf("not implemented")
	}

	// OK, the cluster is hosted using static pods. Upgrade a static-pod hosted cluster
	fmt.Printf("[upgrade/apply] Upgrading your Static Pod-hosted control plane to version %q...\n", flags.newK8sVersionStr)

	if err := upgrade.UpgradeStaticPodControlPlane(client, internalcfg, flags.newK8sVersion); err != nil {
		return err
	}
	return nil
}
