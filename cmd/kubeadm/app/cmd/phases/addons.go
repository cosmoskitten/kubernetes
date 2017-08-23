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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	dnsaddon "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/dns"
	proxyaddon "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/proxy"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
	"k8s.io/kubernetes/pkg/api"
)

// NewCmdAddon returns the addon Cobra command
func NewCmdAddon() *cobra.Command {
	var kubeConfigFile string
	cfg := &kubeadmapiext.MasterConfiguration{}
	cmd := &cobra.Command{
		Use:     "addon",
		Aliases: []string{},
		Short:   "Install an addon to a Kubernetes cluster.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				fmt.Printf("An addon must be specified")
				os.Exit(1)
			}
			api.Scheme.Default(cfg)
			internalcfg := &kubeadmapi.MasterConfiguration{}
			api.Scheme.Convert(cfg, internalcfg, nil)
			client, err := kubeconfigutil.ClientSetFromFile(kubeConfigFile)
			kubeadmutil.CheckErr(err)
			addon := args[0]
			switch addon {
			case dnsaddon.KubeDNSServiceAccountName:
				return dnsaddon.EnsureDNSAddon(internalcfg, client)
			case proxyaddon.KubeProxyServiceAccountName:
				return proxyaddon.EnsureProxyAddon(internalcfg, client)
			default:
				fmt.Printf("The addon %q is not a valid addon", addon)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kubeConfigFile, "kubeconfig", "/etc/kubernetes/admin.conf", "The KubeConfig file to use for talking to the cluster")
	return cmd
}
