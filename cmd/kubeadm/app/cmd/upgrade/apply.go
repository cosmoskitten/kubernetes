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
	nonInteractiveMode bool
	force              bool
	dryRun             bool
	newK8sVersionStr   string
	newK8sVersion      *version.Version
	pullTimeout        time.Duration
	parent             *cmdUpgradeFlags
}

// SessionIsInteractive returns true if the session is of an interactive type (the default, can be opted out of with -y, -f or --dry-run)
func (f *applyFlags) SessionIsInteractive() bool {
	return !f.nonInteractiveMode
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

			// Default the flags dynamically, based on each others' value
			SetImplicitFlags(flags)

			err := RunApply(flags)
			kubeadmutil.CheckErr(err)
		},
	}

	// Specify the valid flags specific for apply
	cmd.Flags().BoolVarP(&flags.nonInteractiveMode, "yes", "y", flags.nonInteractiveMode, "Perform the upgrade and do not prompt for confirmation (non-interactive mode).")
	cmd.Flags().BoolVarP(&flags.force, "force", "f", flags.force, "Force upgrading although some requirements might not be met. This also implies non-interactive mode.")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", flags.dryRun, "Do not change any state, just output what actions would be applied. This also implies non-interactive mode.")
	cmd.Flags().StringVar(&flags.newK8sVersionStr, "version", flags.newK8sVersionStr, "Selected version to upgrade to.")
	cmd.Flags().DurationVar(&flags.pullTimeout, "pull-timeout", flags.pullTimeout, "The maximum amount of time to wait for the control plane pods to be downloaded.")

	return cmd
}

// RunApply takes care of the actual upgrade functionality
func RunApply(flags *applyFlags) error {

	// Start with the basics, verify that the cluster is healthy and get the configuration from the cluster (using the ConfigMap)
	upgradeVars, err := EnforceRequirements(flags.parent.kubeConfigPath, flags.parent.cfgPath, flags.parent.printConfig, flags.nonInteractiveMode)
	if err != nil {
		return err
	}

	// Set the upgraded version on the external config object now
	upgradeVars.cfg.KubernetesVersion = flags.newK8sVersionStr

	// Grab the external, versioned configuration and convert it to the internal type for usage here later
	internalcfg := &kubeadmapi.MasterConfiguration{}
	api.Scheme.Convert(upgradeVars.cfg, internalcfg, nil)

	// Enforce the version skew policies
	if err := EnforceVersionPolicies(flags, upgradeVars.versionGetter); err != nil {
		return fmt.Errorf("[upgrade/version] FATAL: %v", err)
	}

	// If the current session is interactive, ask the user whether they really want to upgrade
	if flags.SessionIsInteractive() {
		if err := upgrade.InteractivelyConfirmUpgrade("Are you sure you want to proceed with the upgrade"); err != nil {
			return err
		}
	}

	// TODO: Implement a prepulling mechanism here

	// Now; perform the upgrade procedure
	if err := UpgradeControlPlaneComponents(flags, upgradeVars.client, internalcfg); err != nil {
		return err
	}

	// Upgrade RBAC rules and addons. Optionally, if needed, perform some extra task for a specific version
	if err := upgrade.PerformPostUpgradeTasks(upgradeVars.client, internalcfg, flags.newK8sVersion); err != nil {
		return err
	}

	fmt.Println("")
	fmt.Printf("[upgrade/successful] SUCCESS! Your cluster was upgraded to %q. Enjoy!\n", flags.newK8sVersionStr)
	fmt.Println("")
	fmt.Println("[upgrade/kubelet] Now that your control plane is upgraded, please proceed with upgrading your kubelets in turn.")

	return nil
}

// ValidateFlags makes sure the arguments given to apply are valid
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

// SetImplicitFlags handles dynamically defaulting flags based on each other's value
func SetImplicitFlags(flags *applyFlags) {
	// If we are in dry-run or force mode; we should automatically execute this command non-interactively
	if flags.dryRun || flags.force {
		flags.nonInteractiveMode = true
	}
}

// EnforceVersionPolicies makes sure that the version the user specified is valid to upgrade to
// There are both fatal and skippable (with --force) errors
func EnforceVersionPolicies(flags *applyFlags, versionGetter upgrade.VersionGetter) error {
	fmt.Printf("[upgrade/version] You have chosen to upgrade to version %q\n", flags.newK8sVersionStr)

	versionSkewErrs := upgrade.EnforceVersionPolicies(versionGetter, flags.newK8sVersionStr, flags.newK8sVersion, flags.parent.allowExperimentalUpgrades, flags.parent.allowRCUpgrades)
	if versionSkewErrs != nil {

		if len(versionSkewErrs.Mandatory) > 0 {
			return fmt.Errorf("The --version argument is invalid due to these fatal errors: %v", versionSkewErrs.Mandatory)
		}

		if len(versionSkewErrs.Skippable) > 0 {
			// Return the error if the user hasn't specified the --force flag
			if !flags.force {
				return fmt.Errorf("The --version argument is invalid due to these errors: %v. Can be bypassed if you pass the --force flag", versionSkewErrs.Mandatory)
			}
			// Soft errors found, but --force was specified
			fmt.Printf("[upgrade/version] Found %d potential version compatibility errors but skipping since the --force flag is set: %v\n", len(versionSkewErrs.Skippable), versionSkewErrs.Skippable)
		}
	}
	return nil
}

// UpgradeControlPlaneComponents actually performs the upgrade procedure for the cluster of your type (self-hosted or static-pod-hosted)
func UpgradeControlPlaneComponents(flags *applyFlags, client clientset.Interface, internalcfg *kubeadmapi.MasterConfiguration) error {

	// Check if the cluster is self-hosted and act accordingly
	if upgrade.IsControlPlaneSelfHosted(client) {
		fmt.Printf("[upgrade/apply] Upgrading your Self-Hosted control plane to version %q...\n", flags.newK8sVersionStr)

		// Upgrade a self-hosted cluster
		// TODO(luxas): Implement this later when we have the new upgrade strategy
		return fmt.Errorf("not implemented")
	}

	// OK, the cluster is hosted using static pods. Upgrade a static-pod hosted cluster
	fmt.Printf("[upgrade/apply] Upgrading your Static Pod-hosted control plane to version %q...\n", flags.newK8sVersionStr)

	if err := upgrade.StaticPodControlPlane(client, internalcfg, flags.newK8sVersion); err != nil {
		return err
	}
	return nil
}
