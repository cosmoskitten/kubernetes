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
	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/addons"
	nodebootstraptoken "k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/node"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/apiconfig"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/uploadconfig"
	"k8s.io/kubernetes/pkg/util/version"
)

// PerformPostUpgradeTasks runs nearly the same functions as 'kubeadm init' would do
// Note that the markmaster phase is left out, not needed, and no token is created as that doesn't belong to the upgrade
func PerformPostUpgradeTasks(client clientset.Interface, cfg *kubeadmapi.MasterConfiguration, k8sVersion *version.Version) error {

	// TODO: Is this needed to do here? I think that updating cluster info should probably be separate from a normal upgrade
	// if err := clusterinfo.CreateBootstrapConfigMapIfNotExists(client, adminKubeConfigPath); err != nil {
	// 	return err
	// }

	// TODO: ServiceAccounts should be created by the addon consumers respectively
	if err := apiconfig.CreateServiceAccounts(client); err != nil {
		return err
	}
	// TODO: RBAC rules should be created by the consumers respectively
	if err := apiconfig.CreateRBACRules(client, k8sVersion); err != nil {
		return err
	}
	// TODO: Should be broken into separate kube-proxy/kube-dns
	if err := addons.CreateEssentialAddons(cfg, client, k8sVersion); err != nil {
		return err
	}

	if err := nodebootstraptoken.AllowBootstrapTokensToPostCSRs(client); err != nil {
		return err
	}
	if err := nodebootstraptoken.AutoApproveNodeBootstrapTokens(client, k8sVersion); err != nil {
		return err
	}

	if err := uploadconfig.UploadConfiguration(cfg, client); err != nil {
		return err
	}
	return nil
}
