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

package master

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"k8s.io/kubernetes/cmd/kubeadm/app/master/spec"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	ext "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	rbac "k8s.io/client-go/pkg/apis/rbac/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/images"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/certs/pkiutil"
)

const (
	peerSecret     = "etcd-server-peer-tls"
	clientSecret   = "etcd-server-client-tls"
	operatorSecret = "operator-etcd-client-tls"
)

func launchEtcdOperator(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	start := time.Now()

	clusterRole := getEtcdClusterRole()
	if _, err := client.RbacV1beta1().ClusterRoles().Create(&clusterRole); err != nil {
		return fmt.Errorf("[self-hosted] Failed to create etcd-operator ClusterRole [%v]", err)
	}

	serviceAccount := getEtcdServiceAccount()
	if _, err := client.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Create(&serviceAccount); err != nil {
		return fmt.Errorf("[self-hosted] Failed to create etcd-operator ServiceAccount [%v]", err)
	}

	clusterRoleBinding := getEtcdClusterRoleBinding()
	if _, err := client.RbacV1beta1().ClusterRoleBindings().Create(&clusterRoleBinding); err != nil {
		return fmt.Errorf("[self-hosted] Failed to create etcd-operator ClusterRoleBinding [%v]", err)
	}

	etcdOperatorDep := getEtcdOperatorDeployment(cfg)
	if _, err := client.Extensions().Deployments(metav1.NamespaceSystem).Create(&etcdOperatorDep); err != nil {
		return fmt.Errorf("[self-hosted] Failed to create etcd-operator deployment [%v]", err)
	}

	waitForPodsWithLabel(client, etcdOperator, true)
	fmt.Printf("[self-hosted] etcd-operator deployment ready after %f seconds\n", time.Since(start).Seconds())

	return nil
}

func CreateSelfHostedEtcdCluster(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	start := time.Now()

	// setup TPR client
	restClient, err := getEtcdTPRClient()
	if err != nil {
		return err
	}

	fmt.Println("[self-hosted] Waiting for etcd ThirdPartyResource to exist")
	waitEtcdTPRReady(restClient, time.Second*5, time.Minute*1, metav1.NamespaceSystem)

	seedPodIP, err := getBootEtcdPodIP(client)
	if err != nil {
		return err
	}
	fmt.Printf("[self-hosted] Boot IP for etcd is %s\n", seedPodIP)

	if err := createTLSAssets(client); err != nil {
		return err
	}

	clusterData := getEtcdClusterData(cfg, seedPodIP)
	fmt.Println("[self-hosted] Sending TPR cluster data")
	err = restClient.Post().
		Resource(spec.TPRKindPlural).
		Namespace(metav1.NamespaceSystem).
		Body(clusterData).
		Do().Error()
	if err != nil {
		return fmt.Errorf("[self-hosted] API server rejected TPR call: %v\n", err)
	}

	fmt.Println("[self-hosted] Verifying TPR data exists")
	err = wait.Poll(kubeadmconstants.DiscoveryRetryInterval, 5*time.Minute, func() (bool, error) {
		cluster := &spec.Cluster{}
		err := restClient.Get().
			Resource(spec.TPRKindPlural).
			Namespace(metav1.NamespaceSystem).
			Name(etcdCluster).
			Do().Into(cluster)
		if err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Println("[self-hosted] TPR does not exist yet. Retrying...")
				return false, nil
			}
			fmt.Printf("[self-hosted] Error retrieving etcd cluster: %v\n", err)
			return false, err
		}

		switch cluster.Status.Phase {
		case spec.ClusterPhaseRunning:
			return true, nil
		case spec.ClusterPhaseFailed:
			return false, errors.New("[self-hosted] Failed to create etcd cluster")
		default:
			return false, nil
		}
	})
	if err != nil {
		return err
	}

	fmt.Println("[self-hosted] Waiting for etcd to remove seed member from cluster")
	err = waitBootEtcdRemoved(cfg.Etcd.Cluster.ServiceIP)
	if err != nil {
		return err
	}

	etcdStaticManifestPath := buildStaticManifestFilepath(etcd)
	if err := os.RemoveAll(etcdStaticManifestPath); err != nil {
		return fmt.Errorf("[self-hosted] Unable to delete seed etcd manifest [%v]", err)
	}

	fmt.Println("[self-hosted] Waiting for seed etcd pod to be deleted from kubernetes")
	err = wait.PollInfinite(kubeadmconstants.DiscoveryRetryInterval, func() (bool, error) {
		_, err := client.Core().Pods(metav1.NamespaceSystem).Get(etcd, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("[self-hosted] Unable to delete seed etcd pod: %v", err)
	}

	fmt.Printf("[self-hosted] Self-hosted etcd ready after %f seconds\n", time.Since(start).Seconds())
	return nil
}

func createTLSAssets(client *clientset.Clientset) error {
	if err := createTLSSecret(client, operatorSecret, "etcd", []string{}); err != nil {
		return err
	}

	peerDNSNames := []string{
		fmt.Sprintf("*.%s.%s.svc.cluster.local", etcdCluster, metav1.NamespaceSystem),
	}
	if err := createTLSSecret(client, peerSecret, "peer", peerDNSNames); err != nil {
		return err
	}

	clientDNSNames := []string{
		fmt.Sprintf("*.%s.%s.svc.cluster.local", etcdCluster, metav1.NamespaceSystem),
		fmt.Sprintf("*.%s-client.%s.svc.cluster.local", etcdCluster, metav1.NamespaceSystem),
	}
	if err := createTLSSecret(client, clientSecret, "client", clientDNSNames); err != nil {
		return err
	}

	return nil
}

func createTLSSecret(client *clientset.Clientset, name, prefix string, dnsNames []string) error {
	caCert, caKey, err := pkiutil.NewCertificateAuthority()
	if err != nil {
		return err
	}
	config := certutil.Config{
		CommonName: name,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if len(dnsNames) > 0 {
		config.AltNames = certutil.AltNames{DNSNames: dnsNames}
	}
	cert, key, err := pkiutil.NewCertAndKey(caCert, caKey, config)
	if err != nil {
		return err
	}

	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string][]byte{
			fmt.Sprintf("%s-crt.pem", prefix):    certutil.EncodeCertPEM(cert),
			fmt.Sprintf("%s-key.pem", prefix):    certutil.EncodePrivateKeyPEM(key),
			fmt.Sprintf("%s-ca-crt.pem", prefix): certutil.EncodeCertPEM(caCert),
		},
	}

	if _, err := client.CoreV1().Secrets(metav1.NamespaceSystem).Create(secret); err != nil {
		return fmt.Errorf("Could not create %s secret: %#v", name, err)
	}

	return nil
}

func getEtcdClusterData(cfg *kubeadmapi.MasterConfiguration, seedPodIP string) *spec.Cluster {
	return &spec.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", spec.TPRGroup, spec.TPRVersion),
			Kind:       strings.Title(spec.TPRKind),
		},
		Metadata: metav1.ObjectMeta{
			Name:      etcdCluster,
			Namespace: metav1.NamespaceSystem,
		},
		Spec: spec.ClusterSpec{
			Size:    cfg.Etcd.Cluster.Size,
			Version: cfg.Etcd.Cluster.Version,
			SelfHosted: &spec.SelfHostedPolicy{
				BootMemberClientEndpoint: fmt.Sprintf("http://%s:12379", seedPodIP),
			},
			Pod: &spec.PodPolicy{
				NodeSelector: map[string]string{kubeadmconstants.LabelNodeRoleMaster: ""},
				Tolerations:  []v1.Toleration{kubeadmconstants.MasterToleration},
			},
			TLS: &spec.TLSPolicy{
				Static: &spec.StaticTLS{
					OperatorSecret: operatorSecret,
					Member: &spec.MemberSecret{
						PeerSecret:   peerSecret,
						ClientSecret: clientSecret,
					},
				},
			},
		},
	}
}

func getEtcdTPRClient() (*rest.RESTClient, error) {
	kubeConfigPath := path.Join(kubeadmapi.GlobalEnvParams.KubernetesDir, kubeadmconstants.AdminKubeConfigFileName)
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()

	config.GroupVersion = &schema.GroupVersion{
		Group:   spec.TPRGroup,
		Version: spec.TPRVersion,
	}
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}

	restcli, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}
	return restcli, nil
}

func getBootEtcdPodIP(kubecli *clientset.Clientset) (string, error) {
	var ip string
	interval := 5
	err := wait.Poll(time.Duration(interval)*time.Second, 60*time.Second, func() (bool, error) {
		podList, err := kubecli.CoreV1().Pods(metav1.NamespaceSystem).List(metav1.ListOptions{
			LabelSelector: "component=" + bootEtcd,
		})
		if err != nil {
			fmt.Printf("[self-hosted] Failed to list pods with component=%s selector: %v\n", bootEtcd, err)
			return false, err
		}
		if len(podList.Items) < 1 {
			fmt.Printf("[self-hosted] No %s pod found, retrying after %ds...\n", bootEtcd, interval)
			return false, nil
		}
		pod := podList.Items[0]
		switch pod.Status.Phase {
		case v1.PodRunning:
			ip = pod.Status.PodIP
			return true, nil
		default:
			fmt.Println("[self-hosted] Boot etcd pod not running. Could not extract IP.")
			return false, nil
		}
	})
	return ip, err
}

func waitBootEtcdRemoved(etcdServiceIP string) error {
	err := wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
		etcdcli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{fmt.Sprintf("http://%s:2379", etcdServiceIP)},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			fmt.Printf("[self-hosted] Failed to create etcd client, will retry. Error: %v\n", err)
			return false, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		memberList, err := etcdcli.MemberList(ctx)
		cancel()
		etcdcli.Close()

		if err != nil {
			fmt.Printf("[self-hosted] Failed to list etcd members, will retry. Error: %v\n", err)
			return false, nil
		}

		if len(memberList.Members) != 1 {
			fmt.Println("[self-hosted] Still waiting for boot-etcd to be deleted...")
			return false, nil
		}

		return true, nil
	})
	return err
}

func createEtcdService(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	etcdService := getEtcdService(cfg)
	if _, err := client.Core().Services(metav1.NamespaceSystem).Create(&etcdService); err != nil {
		return fmt.Errorf("[self-hosted] Failed to create self-hosted etcd service: %v", err)
	}
	return nil
}

func getEtcdService(cfg *kubeadmapi.MasterConfiguration) v1.Service {
	return v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdService,
			Namespace: metav1.NamespaceSystem,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app":          "etcd",
				"etcd_cluster": etcdCluster,
			},
			ClusterIP: cfg.Etcd.Cluster.ServiceIP,
			Ports: []v1.ServicePort{
				{Name: "client", Port: 2379, Protocol: "TCP"},
			},
		},
	}
}

func getEtcdClusterRoleBinding() rbac.ClusterRoleBinding {
	return rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1beta1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdOperator,
			Namespace: metav1.NamespaceSystem,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     etcdOperator,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      etcdOperator,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}
}

func getEtcdServiceAccount() v1.ServiceAccount {
	return v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdOperator,
			Namespace: metav1.NamespaceSystem,
		},
	}
}

func getEtcdClusterRole() rbac.ClusterRole {
	return rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1beta1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: etcdOperator,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"etcd.coreos.com"},
				Resources: []string{"clusters"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"thirdpartyresources"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"replicasets", "deployments"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "secrets", "endpoints", "persistentvolumeclaims"},
				Verbs:     []string{"*"},
			},
		},
	}
}

func getEtcdOperatorDeployment(cfg *kubeadmapi.MasterConfiguration) ext.Deployment {
	replicas := int32(1)
	return ext.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdOperator,
			Namespace: metav1.NamespaceSystem,
			Labels:    map[string]string{"k8s-app": etcdOperator},
		},
		Spec: ext.DeploymentSpec{
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": etcdOperator,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  etcdOperator,
							Image: images.GetCoreImage(images.EtcdOperatorImage, cfg, ""),
							Env: []v1.EnvVar{
								getFieldEnv("MY_POD_NAMESPACE", "metadata.namespace"),
								getFieldEnv("MY_POD_NAME", "metadata.name"),
							},
						},
					},
					Tolerations:        []v1.Toleration{kubeadmconstants.MasterToleration},
					ServiceAccountName: etcdOperator,
				},
			},
		},
	}
}

func getFieldEnv(name, fieldPath string) v1.EnvVar {
	return v1.EnvVar{
		Name: name,
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: fieldPath,
			},
		},
	}
}

// TODO: Remove when etcd-operator can be vendored.
func retry(interval time.Duration, maxRetries int, f func() (bool, error)) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for i := 0; ; i++ {
		ok, err := f()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if i+1 == maxRetries {
			break
		}
		<-tick.C
	}
	return fmt.Errorf("Failed retrying after %d retries", maxRetries)
}

// TODO: Remove when etcd-operator can be vendored.
func waitEtcdTPRReady(restcli rest.Interface, interval, timeout time.Duration, ns string) error {
	listClustersURI := fmt.Sprintf("/apis/%s/%s/namespaces/%s/clusters", spec.TPRGroup, spec.TPRVersion, ns)
	return retry(interval, int(timeout/interval), func() (bool, error) {
		_, err := restcli.Get().RequestURI(listClustersURI).DoRaw()
		if err != nil {
			if apierrors.IsNotFound(err) { // not set up yet. wait more.
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}
