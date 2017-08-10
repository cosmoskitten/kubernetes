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
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/version"
)

// applyFlags holds the information about the flags that can be passed to apply
type applyFlags struct {
	flagYes, force, dryRun bool
	newK8sVersionStr       string
	newK8sVersion          *version.Version
	parent                 *cmdUpgradeFlags
}

// NewCmdApply returns the cobra command for `kubeadm upgrade apply`
func NewCmdApply(parentFlags *cmdUpgradeFlags) *cobra.Command {
	flags := &applyFlags{
		parent: parentFlags,
	}

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the upgrade",
		Run: func(cmd *cobra.Command, args []string) {
			kubeadmutil.CheckErr(ValidateFlags(flags))

			err := RunApply(flags)
			kubeadmutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVar(&flags.flagYes, "yes", flags.flagYes, "Actually perform the selected upgrade")
	cmd.Flags().BoolVar(&flags.force, "force", flags.force, "Force upgrading although some requirements might not be met")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", flags.dryRun, "Do not change any state, just output what actions would be applied")
	cmd.Flags().StringVar(&flags.newK8sVersionStr, "version", flags.newK8sVersionStr, "Selected version to upgrade to")

	return cmd
}

// RunApply takes care of the actual upgrade functionality
func RunApply(flags *applyFlags) error {

	// Start with the basics, verify that the cluster is healthy and get config
	client, cfg, err := EnforceRequirements(flags.parent.kubeConfigPath, flags.parent.cfgPath, flags.parent.printConfig)
	if err != nil {
		return err
	}

	// Set the version on the config object now
	cfg.KubernetesVersion = flags.newK8sVersionStr

	internalcfg := &kubeadmapi.MasterConfiguration{}
	api.Scheme.Convert(cfg, internalcfg, nil)

	fmt.Printf("[upgrade/version] You have chosen to upgrade to version %q\n", flags.newK8sVersionStr)
	versionGetterImpl := upgrade.NewKubeVersionGetter(client, os.Stdout)
	versionSkewErrs := upgrade.EnforceVersionPolicies(versionGetterImpl, flags.newK8sVersionStr, flags.newK8sVersion, flags.parent.allowExperimentalUpgrades, flags.parent.allowRCUpgrades)
	if versionSkewErrs != nil {

		if len(versionSkewErrs.Mandatory) > 0 {
			fmt.Printf("[upgrade/version] FATAL: The --version argument is invalid: %v.\n[upgrade/version] Can be bypassed if you pass the --force flag\n", versionSkewErrs.Mandatory)
			return fmt.Errorf("The --version argument is invalid")
		}

		if len(versionSkewErrs.Skippable) > 0 {
			fmt.Printf("[upgrade/version] ERROR: The --version argument is invalid: %v.\n[upgrade/version] Can be bypassed if you pass the --force flag\n", versionSkewErrs.Skippable)

			if !flags.force {
				return fmt.Errorf("The --version argument is invalid")
			}
			fmt.Println("[upgrade/version] Continuing since the --force flag is set")
		}
	}

	if !flags.flagYes && !flags.dryRun {

		fmt.Print("[upgrade/confirm] Are you sure you want to proceed with the upgrade? [y/N]: ")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if err = scanner.Err(); err != nil {
			return fmt.Errorf("couldn't read from standard input: %v", err)
		}
		answer := scanner.Text()
		if strings.ToLower(answer) != "y" {
			return fmt.Errorf("won't proceed; the user didn't answer (Y|y) in order to continue")
		}
	}

	// Use a prepuller implementation based on creating DaemonSets
	prepuller := upgrade.NewDaemonSetPrepuller(client, internalcfg)
	// Block until all DaemonSets are ready
	// TODO: Make this timeout configurable?
	upgrade.PrepullImagesInParallell(prepuller, 15*time.Minute)

	if upgrade.IsControlPlaneSelfHosted(client) {
		fmt.Printf("[upgrade/apply] Upgrading your Self-Hosted control plane to version %q...\n", flags.newK8sVersionStr)

		// Upgrade a self-hosted cluster
		return fmt.Errorf("not implemented")
	} else {
		fmt.Printf("[upgrade/apply] Upgrading your Static Pod-hosted control plane to version %q...\n", flags.newK8sVersionStr)

		// Upgrade a static-pod hosted cluster
		if err := upgrade.UpgradeStaticPodControlPlane(client, internalcfg, flags.newK8sVersion); err != nil {
			return err
		}

		// Upgrade the static pod hosted control plane to a self-hosted one
		// TODO: Do this only in case the self-hosting feature gate is enabled
		//if err := selfhosting.CreateSelfHostedControlPlane(internalcfg, client); err != nil {
		//	return err
		//}
	}

	// Upgrade RBAC rules and addons. Optionally, if needed, perform some extra task for a specific version
	if err := upgrade.PerformPostUpgradeTasks(client, internalcfg, flags.newK8sVersion); err != nil {
		return err
	}

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
