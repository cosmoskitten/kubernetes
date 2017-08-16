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
	"io"

	"github.com/spf13/cobra"
)

type cmdUpgradeFlags struct {
	kubeConfigPath, cfgPath                                 string
	allowExperimentalUpgrades, allowRCUpgrades, printConfig bool
}

// NewCmdPlan returns the cobra command for `kubeadm upgrade`
func NewCmdUpgrade(out io.Writer) *cobra.Command {
	flags := &cmdUpgradeFlags{
		kubeConfigPath:            "/etc/kubernetes/admin.conf",
		cfgPath:                   "",
		allowExperimentalUpgrades: false,
		allowRCUpgrades:           false,
		printConfig:               false,
	}

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade your cluster smoothly to a newer version with this command.",
		RunE:  subCmdRunE("upgrade"),
	}

	cmd.PersistentFlags().StringVar(&flags.kubeConfigPath, "kubeconfig", flags.kubeConfigPath, "The KubeConfig file to use for talking to the cluster")
	cmd.PersistentFlags().StringVar(&flags.cfgPath, "config", flags.cfgPath, "Path to kubeadm config file (WARNING: Usage of a configuration file is experimental)")
	cmd.PersistentFlags().BoolVar(&flags.allowExperimentalUpgrades, "allow-experimental-upgrades", flags.allowExperimentalUpgrades, "Show unstable versions of Kubernetes as an upgrade alternative and allow upgrading to an alpha/beta/release candidate versions of Kubernetes")
	cmd.PersistentFlags().BoolVar(&flags.allowRCUpgrades, "allow-release-candidate-upgrades", flags.allowRCUpgrades, "Show release candidate versions of Kubernetes as an upgrade alternative and allow upgrading to a release candidate versions of Kubernetes")
	cmd.PersistentFlags().BoolVar(&flags.printConfig, "print-config", flags.printConfig, "Whether the configuration file that will be used in the upgrade should be printed or not")

	cmd.AddCommand(NewCmdApply(flags))
	cmd.AddCommand(NewCmdPlan(flags))

	return cmd
}

// subCmdRunE returns a function that handles a case where a subcommand must be specified
// Without this callback, if a user runs just the command without a subcommand,
// or with an invalid subcommand, cobra will print usage information, but still exit cleanly.
// We want to return an error code in these cases so that the
// user knows that their command was invalid.
func subCmdRunE(name string) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing subcommand; %q is not meant to be run on its own", name)
		}

		return fmt.Errorf("invalid subcommand: %q", args[0])
	}
}
