package addons

import (
	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	dnsaddon "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/dns"
	proxyaddon "k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/proxy"
)

// EnsureAllAddons install all addons to a Kubernetes cluster.
func EnsureAllAddons(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {

	addonActions := []func(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error{
		dnsaddon.EnsureDNSAddon,
		proxyaddon.EnsureProxyAddon,
	}

	for _, action := range addonActions {
		err := action(cfg, client)
		if err != nil {
			return err
		}
	}

	return nil
}
