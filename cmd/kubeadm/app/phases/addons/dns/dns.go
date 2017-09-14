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

package dns

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"strconv"
	"strings"

	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/api/rbac/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/version"
)

const (
	// KubeDNSServiceAccountName describes the name of the ServiceAccount for the kube-dns addon
	KubeDNSServiceAccountName = "kube-dns"
)

// EnsureDNSAddon creates the kube-dns addon
func EnsureDNSAddon(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	k8sVersion, err := version.ParseSemantic(cfg.KubernetesVersion)
	if err != nil {
		return fmt.Errorf("couldn't parse kubernetes version %q: %v", cfg.KubernetesVersion, err)
	}

	if err := CreateServiceAccount(client); err != nil {
		return err
	}

	// Get the YAML manifest conditionally based on the k8s version
	kubeDNSDeploymentBytes := GetKubeDNSManifest(k8sVersion)
	dnsDeploymentBytes, err := kubeadmutil.ParseTemplate(kubeDNSDeploymentBytes, struct{ ImageRepository, Arch, Version, DNSDomain, MasterTaintKey string }{
		ImageRepository: cfg.ImageRepository,
		Arch:            runtime.GOARCH,
		// Get the kube-dns version conditionally based on the k8s version
		Version:        GetKubeDNSVersion(k8sVersion),
		DNSDomain:      cfg.Networking.DNSDomain,
		MasterTaintKey: kubeadmconstants.LabelNodeRoleMaster,
	})
	if err != nil {
		return fmt.Errorf("error when parsing kube-dns deployment template: %v", err)
	}

	dnsip, err := getDNSIP(client)
	if err != nil {
		return err
	}

	dnsServiceBytes, err := kubeadmutil.ParseTemplate(KubeDNSService, struct{ DNSIP string }{
		DNSIP: dnsip.String(),
	})
	if err != nil {
		return fmt.Errorf("error when parsing kube-proxy configmap template: %v", err)
	}

	if err := createKubeDNSAddon(dnsDeploymentBytes, dnsServiceBytes, client); err != nil {
		return err
	}
	fmt.Println("[addons] Applied essential addon: kube-dns")
	return nil
}

// CreateServiceAccount creates the necessary serviceaccounts that kubeadm uses/might use, if they don't already exist.
func CreateServiceAccount(client clientset.Interface) error {

	return apiclient.CreateOrUpdateServiceAccount(client, &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KubeDNSServiceAccountName,
			Namespace: metav1.NamespaceSystem,
		},
	})
}

func createKubeDNSAddon(deploymentBytes, serviceBytes []byte, client clientset.Interface) error {
	kubednsDeployment := &extensions.Deployment{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), deploymentBytes, kubednsDeployment); err != nil {
		return fmt.Errorf("unable to decode kube-dns deployment %v", err)
	}

	// Create the Deployment for kube-dns or update it in case it already exists
	if err := apiclient.CreateOrUpdateDeployment(client, kubednsDeployment); err != nil {
		return err
	}

	kubednsService := &v1.Service{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), serviceBytes, kubednsService); err != nil {
		return fmt.Errorf("unable to decode kube-dns service %v", err)
	}

	// Can't use a generic apiclient helper func here as we have to tolerate more than AlreadyExists.
	if _, err := client.CoreV1().Services(metav1.NamespaceSystem).Create(kubednsService); err != nil {
		// Ignore if the Service is invalid with this error message:
		// 	Service "kube-dns" is invalid: spec.clusterIP: Invalid value: "10.96.0.10": provided IP is already allocated

		if !apierrors.IsAlreadyExists(err) && !apierrors.IsInvalid(err) {
			return fmt.Errorf("unable to create a new kube-dns service: %v", err)
		}

		if _, err := client.CoreV1().Services(metav1.NamespaceSystem).Update(kubednsService); err != nil {
			return fmt.Errorf("unable to create/update the kube-dns service: %v", err)
		}
	}
	return nil
}

func EnsureCoreDNSAddon(cfg *kubeadmapi.MasterConfiguration, client clientset.Interface) error {
	k8sVersion, err := version.ParseSemantic(cfg.KubernetesVersion)
	if err != nil {
		return fmt.Errorf("couldn't parse kubernetes version %q: %v", cfg.KubernetesVersion, err)
	}
	// Get the YAML manifest conditionally based on the k8s version
	dnsDeploymentBytes := GetCoreDNSManifest(k8sVersion)
	coreDNSDeploymentBytes, err := kubeadmutil.ParseTemplate(dnsDeploymentBytes, struct{ MasterTaintKey, Version string }{
		MasterTaintKey: kubeadmconstants.LabelNodeRoleMaster,
		Version:        GetCoreDNSVersion(k8sVersion),
	})
	if err != nil {
		return fmt.Errorf("error when parsing CoreDNS deployment template: %v", err)
	}

	// Get the config file for CoreDNS
	coreDNSConfigMapBytes, err := kubeadmutil.ParseTemplate(CoreDNSConfigMap, struct{ DNSDomain, Servicecidr string }{
		Servicecidr: convSubnet(cfg.Networking.ServiceSubnet),
		DNSDomain:   cfg.Networking.DNSDomain,
	})
	if err != nil {
		return fmt.Errorf("error when parsing CoreDNS configMap template: %v", err)
	}

	// Get the ClusterRole file for CoreDNS
	coreDNSClusterRoleBytes, err := kubeadmutil.ParseTemplate(CoreDNSClusterRole, "")
	if err != nil {
		return fmt.Errorf("error when parsing CoreDNS ClusterRole template: %v", err)
	}

	// Get the ClusterRoleBinding file for CoreDNS
	coreDNSClusterRoleBindingBytes, err := kubeadmutil.ParseTemplate(CoreDNSClusterRoleBinding, "")
	if err != nil {
		return fmt.Errorf("error when parsing CoreDNS ClusterRole template: %v", err)
	}

	coreDNSServiceAccountBytes, err := kubeadmutil.ParseTemplate(CoreDNSServiceAccount, "")
	if err != nil {
		return fmt.Errorf("error when parsing CoreDNS ServiceAccount template: %v", err)
	}

	dnsip, err := getDNSIP(client)
	if err != nil {
		return err
	}

	coreDNSServiceBytes, err := kubeadmutil.ParseTemplate(CoreDNSService, struct{ DNSIP string }{
		DNSIP: dnsip.String(),
	})

	if err != nil {
		return fmt.Errorf("error when parsing CoreDNS service template: %v", err)
	}

	if err := createCoreDNSAddon(coreDNSDeploymentBytes, coreDNSServiceBytes, coreDNSConfigMapBytes, coreDNSClusterRoleBytes, coreDNSClusterRoleBindingBytes, coreDNSServiceAccountBytes, client); err != nil {
		return err
	}
	fmt.Println("[addons] Applied essential addon: CoreDNS")
	return nil
}

func createCoreDNSAddon(deploymentBytes, serviceBytes, configBytes, clusterroleBytes, clusterrolebindingBytes, serviceAccountBytes []byte, client clientset.Interface) error {
	coreDNSConfigMap := &v1.ConfigMap{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), configBytes, coreDNSConfigMap); err != nil {
		return fmt.Errorf("unable to decode CoreDNS configmap %v", err)
	}

	// Create the ConfigMap for CoreDNS or update it in case it already exists
	if err := apiclient.CreateOrUpdateConfigMap(client, coreDNSConfigMap); err != nil {
		return err
	}

	coreDNSClusterRoles := &v1beta1.ClusterRole{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), clusterroleBytes, coreDNSClusterRoles); err != nil {
		return fmt.Errorf("unable to decode CoreDNS clusterroles %v", err)
	}

	// Create the Clusterroles for CoreDNS or update it in case it already exists
	if err := apiclient.CreateOrUpdateClusterRole(client, coreDNSClusterRoles); err != nil {
		return err
	}

	coreDNSClusterRolesBinding := &v1beta1.ClusterRoleBinding{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), clusterrolebindingBytes, coreDNSClusterRolesBinding); err != nil {
		return fmt.Errorf("unable to decode CoreDNS clusterrolebindings %v", err)
	}

	// Create the Clusterrolebindings for CoreDNS or update it in case it already exists
	if err := apiclient.CreateOrUpdateClusterRoleBinding(client, coreDNSClusterRolesBinding); err != nil {
		return err
	}

	coreDNSServiceAccount := &v1.ServiceAccount{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), serviceAccountBytes, coreDNSServiceAccount); err != nil {
		return fmt.Errorf("unable to decode CoreDNS configmap %v", err)
	}

	// Create the ConfigMap for CoreDNS or update it in case it already exists
	if err := apiclient.CreateOrUpdateServiceAccount(client, coreDNSServiceAccount); err != nil {
		return err
	}

	coreDNSDeployment := &extensions.Deployment{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), deploymentBytes, coreDNSDeployment); err != nil {
		return fmt.Errorf("unable to decode CoreDNS deployment %v", err)
	}

	// Create the Deployment for CoreDNS or update it in case it already exists
	if err := apiclient.CreateOrUpdateDeployment(client, coreDNSDeployment); err != nil {
		return err
	}

	coreDNSService := &v1.Service{}
	if err := kuberuntime.DecodeInto(api.Codecs.UniversalDecoder(), serviceBytes, coreDNSService); err != nil {
		return fmt.Errorf("unable to decode coreDNS service %v", err)
	}

	// Can't use a generic apiclient helper func here as we have to tolerate more than AlreadyExists.
	if _, err := client.CoreV1().Services(metav1.NamespaceSystem).Create(coreDNSService); err != nil {
		// Ignore if the Service is invalid with this error message:
		// 	Service "CoreDNS" is invalid: spec.clusterIP: Invalid value: "10.96.0.10": provided IP is already allocated

		if !apierrors.IsAlreadyExists(err) && !apierrors.IsInvalid(err) {
			return fmt.Errorf("unable to create a new CoreDNS service: %v", err)
		}

		if _, err := client.CoreV1().Services(metav1.NamespaceSystem).Update(coreDNSService); err != nil {
			return fmt.Errorf("unable to create/update the CoreDNS service: %v", err)
		}
	}
	return nil
}

//convSubnet fetches the servicecidr and modifies the mask to the nearest class
//CoreDNS requires CIDR notations for reverse zones as classful.
func convSubnet(cidr string) (servicecidr string) {
	var newMask int
	mask := strings.Split(cidr, "/")
	i, _ := strconv.Atoi(mask[1])

	if i > 8 {
		newMask = i - (i % 8)
	} else {
		newMask = 8
	}
	cidr = mask[0] + "/" + strconv.Itoa(newMask)
	_, ipv4Net, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Fatal(err)
	}
	servicecidr = ipv4Net.String()

	return servicecidr
}

// getDNSIP fetches the kubernetes service's ClusterIP and appends a "0" to it in order to get the DNS IP
func getDNSIP(client clientset.Interface) (net.IP, error) {
	k8ssvc, err := client.CoreV1().Services(metav1.NamespaceDefault).Get("kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch information about the kubernetes service: %v", err)
	}

	if len(k8ssvc.Spec.ClusterIP) == 0 {
		return nil, fmt.Errorf("couldn't fetch a valid clusterIP from the kubernetes service")
	}

	// Build an IP by taking the kubernetes service's clusterIP and appending a "0" and checking that it's valid
	dnsIP := net.ParseIP(fmt.Sprintf("%s0", k8ssvc.Spec.ClusterIP))
	if dnsIP == nil {
		return nil, fmt.Errorf("could not parse dns ip %q: %v", dnsIP, err)
	}
	return dnsIP, nil
}
