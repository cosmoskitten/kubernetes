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

package phases

import (
	"github.com/spf13/cobra"

	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	dnsaddon "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/dns"
	proxyaddon "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/proxy"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	"k8s.io/kubernetes/pkg/api"
)

// NewCmdAddon returns the addon Cobra command
func NewCmdAddon() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "addon <addon-name>",
		Aliases: []string{"addons"},
		Short:   "Install an addon to a Kubernetes cluster.",
		RunE:    subCmdRunE("addon"),
	}

	cmd.AddCommand(getAddonsSubCommands()...)
	return cmd
}

func EnsureAllAddons(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	return nil
}

// getAddonsSubCommands returns sub commands for addons phase
func getAddonsSubCommands() []*cobra.Command {
	cfg := &kubeadmapiext.MasterConfiguration{}
	// Default values for the cobra help text
	api.Scheme.Default(cfg)

	var cfgPath, kubeConfigFile string
	var subCmds []*cobra.Command

	subCmdProperties := []struct {
		use     string
		short   string
		cmdFunc func(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error
	}{
		{
			use:     "all",
			short:   "Install all addons to a Kubernetes cluster",
			cmdFunc: EnsureAllAddons,
		},
		{
			use:     "kube-dns",
			short:   "Install kube-dns addon to a Kubernetes cluster.",
			cmdFunc: dnsaddon.EnsureDNSAddon,
		},
		{
			use:     "kube-proxy",
			short:   "Install kube-proxy addon to a Kubernetes cluster.",
			cmdFunc: proxyaddon.EnsureProxyAddon,
		},
	}

	for _, properties := range subCmdProperties {
		// Creates the UX Command
		cmd := &cobra.Command{
			Use:   properties.use,
			Short: properties.short,
			Run:   runAddonsCmdFunc(properties.cmdFunc, cfg),
		}

		// Add flags to the command
		cmd.Flags().StringVar(&kubeConfigFile, "kubeconfig", "/etc/kubernetes/admin.conf", "The KubeConfig file to use for talking to the cluster")
		cmd.Flags().StringVar(&cfgPath, "config", cfgPath, "Path to kubeadm config file (WARNING: Usage of a configuration file is experimental)")

		subCmds = append(subCmds, cmd)
	}

	return subCmds
}

// runAddonsCmdFunc creates a cobra.Command Run function, by composing the call to the given cmdFunc with necessary additional steps (e.g preparation of input parameters)
func runAddonsCmdFunc(cmdFunc func(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error, cfg *kubeadmapiext.MasterConfiguration) func(cmd *cobra.Command, args []string) {

	// the following statement build a clousure that wraps a call to a cmdFunc, binding
	// the function itself with the specific parameters of each sub command.
	// Please note that specific parameter should be passed as value, while other parameters - passed as reference -
	// are shared between sub commands and gets access to current value e.g. flags value.

	return func(cmd *cobra.Command, args []string) {
		if err := validation.ValidateMixedArguments(cmd.Flags()); err != nil {
			kubeadmutil.CheckErr(err)
		}

		var kubeConfigFile string
		internalcfg := &kubeadmapi.MasterConfiguration{}
		api.Scheme.Convert(cfg, internalcfg, nil)
		client, err := kubeconfigutil.ClientSetFromFile(kubeConfigFile)
		kubeadmutil.CheckErr(err)
		// internalcfg, err := configutil.ConfigFileAndDefaultsToInternalConfig(*cfgPath, cfg)
		kubeadmutil.CheckErr(err)

		// Execute the cmdFunc
		err = cmdFunc(internalcfg, client)
		kubeadmutil.CheckErr(err)
	}
}
