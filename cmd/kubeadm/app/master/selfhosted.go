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
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/k8sutil"
	"github.com/coreos/etcd/clientv3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	ext "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	rbac "k8s.io/client-go/pkg/apis/rbac/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/images"
	"k8s.io/kubernetes/pkg/util/version"
)

var (
	// maximum unavailable and surge instances per self-hosted component deployment
	maxUnavailable = intstr.FromInt(0)
	maxSurge       = intstr.FromInt(1)
)

func CreateSelfHostedControlPlane(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	volumes := []v1.Volume{k8sVolume()}
	volumeMounts := []v1.VolumeMount{k8sVolumeMount()}
	if isCertsVolumeMountNeeded() {
		volumes = append(volumes, certsVolume(cfg))
		volumeMounts = append(volumeMounts, certsVolumeMount())
	}

	if isPkiVolumeMountNeeded() {
		volumes = append(volumes, pkiVolume())
		volumeMounts = append(volumeMounts, pkiVolumeMount())
	}

	// Need lock for self-hosted
	volumes = append(volumes, flockVolume())
	volumeMounts = append(volumeMounts, flockVolumeMount())

	// create etcd service which fans out to eventual etcd cluster
	if err := createEtcdService(cfg, client); err != nil {
		return err
	}

	if err := launchSelfHostedAPIServer(cfg, client, volumes, volumeMounts); err != nil {
		return err
	}

	if err := launchSelfHostedScheduler(cfg, client, volumes, volumeMounts); err != nil {
		return err
	}

	if err := launchSelfHostedControllerManager(cfg, client, volumes, volumeMounts); err != nil {
		return err
	}

	if err := launchSelfHostedProxy(cfg, client); err != nil {
		return err
	}

	if err := launchEtcdOperator(cfg, client); err != nil {
		return err
	}

	return nil
}

func getEtcdTPRClient() (*rest.RESTClient, error) {
	kubeConfigPath := path.Join(kubeadmapi.GlobalEnvParams.KubernetesDir, kubeadmconstants.AdminKubeConfigFileName)
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}

	config.GroupVersion = &schema.GroupVersion{
		Group:   spec.TPRGroup,
		Version: spec.TPRVersion,
	}
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	restcli, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}
	return restcli, nil
}

func getBootEtcdPodIP(kubecli *clientset.Clientset) (string, error) {
	var ip string
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		podList, err := kubecli.CoreV1().Pods(api.NamespaceSystem).List(metav1.ListOptions{
			LabelSelector: "component=boot-etcd",
		})
		if err != nil {
			fmt.Printf("failed to list 'boot-etcd' pod: %v\n", err)
			return false, err
		}
		if len(podList.Items) < 1 {
			fmt.Printf("no 'boot-etcd' pod found, retrying after 5s...\n")
			return false, nil
		}

		pod := podList.Items[0]
		ip = pod.Status.PodIP
		if len(ip) == 0 {
			return false, nil
		}
		return true, nil
	})
	return ip, err
}

func CreateEtcdCluster(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	start := time.Now()

	// setup TPR client
	restClient, err := getEtcdTPRClient()
	if err != nil {
		return err
	}

	fmt.Println("Wait for TPR to exist")
	k8sutil.WaitEtcdTPRReady(restClient, time.Second*5, time.Minute*1, "kube-system")

	seedPodIP, err := getBootEtcdPodIP(client)
	if err != nil {
		return err
	}
	fmt.Printf("Boot IP is %s\n", seedPodIP)

	// create TPR data
	cluster := &spec.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "etcd.coreos.com/v1beta1",
			Kind:       "Cluster",
		},
		Metadata: metav1.ObjectMeta{
			Name:      etcdCluster,
			Namespace: metav1.NamespaceSystem,
		},
		Spec: spec.ClusterSpec{
			Size:    1,
			Version: "v3.1.6",
			SelfHosted: &spec.SelfHostedPolicy{
				BootMemberClientEndpoint: fmt.Sprintf("http://%s:12379", seedPodIP),
			},
			Pod: &spec.PodPolicy{
				NodeSelector: map[string]string{"node-role.kubernetes.io/master": ""},
				Tolerations: []v1.Toleration{
					v1.Toleration{
						Key:      "node-role.kubernetes.io/master",
						Operator: "Exists",
						Effect:   v1.TaintEffectNoSchedule,
					},
				},
			},
		},
	}

	fmt.Println("Sending TPR cluster data")
	err = restClient.Post().
		Resource(spec.TPRKindPlural).
		Namespace(metav1.NamespaceSystem).
		Body(cluster).
		Do().Error()
	if err != nil {
		return err
	}

	fmt.Println("Waiting for 30s")
	time.Sleep(30 * time.Second)

	fmt.Println("Waiting for TPR data to exist")
	err = wait.Poll(kubeadmconstants.DiscoveryRetryInterval, 5*time.Minute, func() (bool, error) {
		cluster := &spec.Cluster{}

		err := restClient.Get().
			Resource(spec.TPRKindPlural).
			Namespace(metav1.NamespaceSystem).
			Name(etcdCluster).
			Do().Into(cluster)

		if err != nil {
			fmt.Printf("[self-hosted] Error retrieving etcd cluster: %v\n", err)
			return false, nil
		}

		switch cluster.Status.Phase {
		case spec.ClusterPhaseRunning:
			return true, nil
		case spec.ClusterPhaseFailed:
			return false, errors.New("failed to create etcd cluster")
		default:
			return false, nil
		}
	})
	if err != nil {
		return err
	}

	fmt.Println("Waiting for etcd to remove peer")
	err = waitBootEtcdRemoved(cfg.Etcd.Cluster.ServiceIP)
	if err != nil {
		return err
	}

	fmt.Println("Removing seed etcd")
	// remove seed etcd pod
	etcdStaticManifestPath := buildStaticManifestFilepath(etcd)
	if err := os.RemoveAll(etcdStaticManifestPath); err != nil {
		return fmt.Errorf("unable to delete seed etcd manifest [%v]", err)
	}

	fmt.Println("Waiting for seed etcd pod to delete")

	//wait for seed etcd to disappear
	wait.PollInfinite(kubeadmconstants.DiscoveryRetryInterval, func() (bool, error) {
		_, err := client.Core().Pods(metav1.NamespaceSystem).Get(etcd, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})

	fmt.Printf("[self-hosted] self-hosted etcd ready after %f seconds\n", time.Since(start).Seconds())
	return nil
}

func waitBootEtcdRemoved(etcdServiceIP string) error {
	err := wait.Poll(10*time.Second, 5*time.Minute, func() (bool, error) {
		cfg := clientv3.Config{
			Endpoints:   []string{fmt.Sprintf("http://%s:2379", etcdServiceIP)},
			DialTimeout: 5 * time.Second,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			fmt.Printf("failed to create etcd client, will retry: %v", err)
			return false, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		m, err := etcdcli.MemberList(ctx)
		cancel()
		etcdcli.Close()
		if err != nil {
			fmt.Printf("failed to list etcd members, will retry: %v", err)
			return false, nil
		}

		if len(m.Members) != 1 {
			fmt.Println("still waiting for boot-etcd to be deleted...")
			return false, nil
		}
		return true, nil
	})
	return err
}

func createEtcdService(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	etcdService := getEtcdService(cfg)
	if _, err := client.Core().Services(metav1.NamespaceSystem).Create(&etcdService); err != nil {
		return fmt.Errorf("failed to create self-hosted etcd service [%v]", err)
	}
	return nil
}

func launchSelfHostedProxy(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	sa := getKubeProxyServiceAccount()
	if _, err := client.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Create(&sa); err != nil {
		return fmt.Errorf("failed to create self-hosted kube-proxy serviceaccount [%v]", err)
	}

	crb := getKubeProxyClusterRoleBinding()
	if _, err := client.RbacV1beta1().ClusterRoleBindings().Create(&crb); err != nil {
		return fmt.Errorf("failed to create self-hosted kube-proxy clusterrolebinding [%v]", err)
	}

	cm := getKubeProxyConfigMap(cfg)
	if _, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Create(&cm); err != nil {
		return fmt.Errorf("failed to create self-hosted kube-proxy configmap [%v]", err)
	}

	ds := getKubeProxyDS(cfg)
	if _, err := client.ExtensionsV1beta1().DaemonSets(metav1.NamespaceSystem).Create(&ds); err != nil {
		return fmt.Errorf("failed to create self-hosted kube-proxy daemonset [%v]", err)
	}

	return nil
}

func getKubeProxyServiceAccount() v1.ServiceAccount {
	return v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadmconstants.KubeProxyServiceAccountName,
			Namespace: metav1.NamespaceSystem,
		},
	}
}

func getKubeProxyClusterRoleBinding() rbac.ClusterRoleBinding {
	return rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:node-proxier",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:node-proxier",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      kubeadmconstants.KubeProxyServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	}
}

func launchSelfHostedAPIServer(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset, volumes []v1.Volume, volumeMounts []v1.VolumeMount) error {
	start := time.Now()

	kubeVersion, err := version.ParseSemantic(cfg.KubernetesVersion)
	if err != nil {
		return err
	}
	apiServer := getAPIServerDS(cfg, volumes, volumeMounts, kubeVersion)
	if _, err := client.Extensions().DaemonSets(metav1.NamespaceSystem).Create(&apiServer); err != nil {
		return fmt.Errorf("failed to create self-hosted %q daemon set [%v]", kubeAPIServer, err)
	}

	wait.PollInfinite(kubeadmconstants.APICallRetryInterval, func() (bool, error) {
		// TODO: This might be pointless, checking the pods is probably enough.
		// It does however get us a count of how many there should be which may be useful
		// with HA.
		apiDS, err := client.DaemonSets(metav1.NamespaceSystem).Get("self-hosted-"+kubeAPIServer,
			metav1.GetOptions{})
		if err != nil {
			fmt.Println("[self-hosted] error getting apiserver DaemonSet:", err)
			return false, nil
		}
		fmt.Printf("[self-hosted] %s DaemonSet current=%d, desired=%d\n",
			kubeAPIServer,
			apiDS.Status.CurrentNumberScheduled,
			apiDS.Status.DesiredNumberScheduled)

		if apiDS.Status.CurrentNumberScheduled != apiDS.Status.DesiredNumberScheduled {
			return false, nil
		}

		return true, nil
	})

	// Wait for self-hosted API server to take ownership
	waitForPodsWithLabel(client, "self-hosted-"+kubeAPIServer, true)

	// Remove temporary API server
	apiServerStaticManifestPath := buildStaticManifestFilepath(kubeAPIServer)
	if err := os.RemoveAll(apiServerStaticManifestPath); err != nil {
		return fmt.Errorf("unable to delete temporary API server manifest [%v]", err)
	}

	WaitForAPI(client)

	fmt.Printf("[self-hosted] self-hosted kube-apiserver ready after %f seconds\n", time.Since(start).Seconds())
	return nil
}

func launchSelfHostedControllerManager(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset, volumes []v1.Volume, volumeMounts []v1.VolumeMount) error {
	start := time.Now()

	ctrlMgr := getControllerManagerDeployment(cfg, volumes, volumeMounts)
	if _, err := client.Extensions().Deployments(metav1.NamespaceSystem).Create(&ctrlMgr); err != nil {
		return fmt.Errorf("failed to create self-hosted %q deployment [%v]", kubeControllerManager, err)
	}

	waitForPodsWithLabel(client, "self-hosted-"+kubeControllerManager, true)

	ctrlMgrStaticManifestPath := buildStaticManifestFilepath(kubeControllerManager)
	if err := os.RemoveAll(ctrlMgrStaticManifestPath); err != nil {
		return fmt.Errorf("unable to delete temporary controller manager manifest [%v]", err)
	}

	fmt.Printf("[self-hosted] self-hosted kube-controller-manager ready after %f seconds\n", time.Since(start).Seconds())
	return nil

}

func launchSelfHostedScheduler(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset, volumes []v1.Volume, volumeMounts []v1.VolumeMount) error {
	start := time.Now()
	scheduler := getSchedulerDeployment(cfg, volumes, volumeMounts)
	if _, err := client.Extensions().Deployments(metav1.NamespaceSystem).Create(&scheduler); err != nil {
		return fmt.Errorf("failed to create self-hosted %q deployment [%v]", kubeScheduler, err)
	}

	waitForPodsWithLabel(client, "self-hosted-"+kubeScheduler, true)

	schedulerStaticManifestPath := buildStaticManifestFilepath(kubeScheduler)
	if err := os.RemoveAll(schedulerStaticManifestPath); err != nil {
		return fmt.Errorf("unable to delete temporary scheduler manifest [%v]", err)
	}

	fmt.Printf("[self-hosted] self-hosted kube-scheduler ready after %f seconds\n", time.Since(start).Seconds())
	return nil
}

func launchEtcdOperator(cfg *kubeadmapi.MasterConfiguration, client *clientset.Clientset) error {
	start := time.Now()

	clusterRole := getEtcdClusterRole()
	if _, err := client.RbacV1beta1().ClusterRoles().Create(&clusterRole); err != nil {
		return fmt.Errorf("failed to create etcd-operator ClusterRole [%v]", err)
	}

	serviceAccount := getEtcdServiceAccount()
	if _, err := client.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Create(&serviceAccount); err != nil {
		return fmt.Errorf("failed to create etcd-operator ServiceAccount [%v]", err)
	}

	clusterRoleBinding := getEtcdClusterRoleBinding()
	if _, err := client.RbacV1beta1().ClusterRoleBindings().Create(&clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create etcd-operator ClusterRoleBinding [%v]", err)
	}

	etcdOperatorDep := getEtcdOperatorDeployment(cfg)
	if _, err := client.Extensions().Deployments(metav1.NamespaceSystem).Create(&etcdOperatorDep); err != nil {
		return fmt.Errorf("failed to create etcd-operator deployment [%v]", err)
	}

	waitForPodsWithLabel(client, etcdOperator, true)
	fmt.Printf("[self-hosted] etcd-operator deployment ready after %f seconds\n", time.Since(start).Seconds())

	return nil
}

// waitForPodsWithLabel will lookup pods with the given label and wait until they are all
// reporting status as running.
func waitForPodsWithLabel(client *clientset.Clientset, label string, mustBeRunning bool) {
	wait.PollInfinite(kubeadmconstants.APICallRetryInterval, func() (bool, error) {
		// TODO: Do we need a stronger label link than this?
		listOpts := metav1.ListOptions{LabelSelector: fmt.Sprintf("k8s-app=%s", label)}
		apiPods, err := client.Pods(metav1.NamespaceSystem).List(listOpts)
		if err != nil {
			fmt.Printf("[self-hosted] error getting %s pods [%v]\n", label, err)
			return false, nil
		}
		fmt.Printf("[self-hosted] Found %d %s pods\n", len(apiPods.Items), label)

		// TODO: HA
		if int32(len(apiPods.Items)) != 1 {
			return false, nil
		}
		for _, pod := range apiPods.Items {
			fmt.Printf("[self-hosted] Pod %s status: %s\n", pod.Name, pod.Status.Phase)
			if mustBeRunning && pod.Status.Phase != "Running" {
				return false, nil
			}
		}

		return true, nil
	})
}

func getKubeProxyDS(cfg *kubeadmapi.MasterConfiguration) ext.DaemonSet {
	privileged := true
	return ext.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			Labels:    map[string]string{"app": "kube-proxy"},
		},
		Spec: ext.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"k8s-app": "kube-proxy"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"k8s-app": "kube-proxy"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:            "kube-proxy",
							Image:           images.GetCoreImage(images.KubeProxyImage, cfg, kubeadmapi.GlobalEnvParams.HyperkubeImage),
							ImagePullPolicy: "IfNotPresent",
							Command: []string{
								"/usr/local/bin/kube-proxy",
								"--kubeconfig=/var/lib/kube-proxy/kubeconfig.conf",
								getClusterCIDR(cfg.Networking.PodSubnet),
							},
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []v1.VolumeMount{
								v1.VolumeMount{
									MountPath: "/var/lib/kube-proxy",
									Name:      "kube-proxy",
								},
							},
						},
					},
					HostNetwork:        true,
					ServiceAccountName: "kube-proxy",
					Tolerations:        []v1.Toleration{kubeadmconstants.MasterToleration},
					Volumes: []v1.Volume{
						v1.Volume{
							Name: "kube-proxy",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: "kube-proxy"},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getClusterCIDR(podsubnet string) string {
	if len(podsubnet) == 0 {
		return ""
	}
	return "- --cluster-cidr=" + podsubnet
}

func getKubeProxyConfigMap(cfg *kubeadmapi.MasterConfiguration) v1.ConfigMap {
	kubeConfig := fmt.Sprintf(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    server: https://%s:%d
  name: default
contexts:
- context:
    cluster: default
    namespace: default
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
`, cfg.API.AdvertiseAddress, cfg.API.BindPort)
	return v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			Labels:    map[string]string{"app": "kube-proxy"},
		},
		Data: map[string]string{"kubeconfig.conf": kubeConfig},
	}
}

func getEtcdService(cfg *kubeadmapi.MasterConfiguration) v1.Service {
	return v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-service",
			Namespace: "kube-system",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app":          "etcd",
				"etcd_cluster": etcdCluster,
			},
			ClusterIP: cfg.Etcd.Cluster.ServiceIP,
			Ports: []v1.ServicePort{
				v1.ServicePort{Name: "client", Port: 2379, Protocol: "TCP"},
			},
		},
	}
}

func getEtcdClusterTPR(cfg *kubeadmapi.MasterConfiguration) ext.ThirdPartyResource {
	return ext.ThirdPartyResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: spec.TPRName(),
		},
		Versions: []ext.APIVersion{
			{Name: spec.TPRVersion},
		},
		Description: spec.TPRDescription,
	}
}

// Sources from bootkube templates.go
func getAPIServerDS(cfg *kubeadmapi.MasterConfiguration, volumes []v1.Volume, volumeMounts []v1.VolumeMount, kubeVersion *version.Version) ext.DaemonSet {
	ds := ext.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "self-hosted-" + kubeAPIServer,
			Namespace: "kube-system",
			Labels:    map[string]string{"k8s-app": "self-hosted-" + kubeAPIServer},
		},
		Spec: ext.DaemonSetSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app":   "self-hosted-" + kubeAPIServer,
						"component": kubeAPIServer,
						"tier":      "control-plane",
					},
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{kubeadmconstants.LabelNodeRoleMaster: ""},
					HostNetwork:  true,
					Volumes:      volumes,
					Containers: []v1.Container{
						{
							Name:  "self-hosted-" + kubeAPIServer,
							Image: images.GetCoreImage(images.KubeAPIServerImage, cfg, kubeadmapi.GlobalEnvParams.HyperkubeImage),
							// Need to append etcd service IP
							Command:       getAPIServerCommand(cfg, true, kubeVersion, true),
							Env:           getSelfHostedAPIServerEnv(),
							VolumeMounts:  volumeMounts,
							LivenessProbe: componentProbe(6443, "/healthz", v1.URISchemeHTTPS),
							Resources:     componentResources("250m"),
						},
					},
					Tolerations: []v1.Toleration{kubeadmconstants.MasterToleration},
					DNSPolicy:   v1.DNSClusterFirstWithHostNet,
				},
			},
		},
	}
	return ds
}

func getControllerManagerDeployment(cfg *kubeadmapi.MasterConfiguration, volumes []v1.Volume, volumeMounts []v1.VolumeMount) ext.Deployment {
	d := ext.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "self-hosted-" + kubeControllerManager,
			Namespace: "kube-system",
			Labels:    map[string]string{"k8s-app": "self-hosted-" + kubeControllerManager},
		},
		Spec: ext.DeploymentSpec{
			// TODO bootkube uses 2 replicas
			Strategy: ext.DeploymentStrategy{
				Type: ext.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &ext.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app":   "self-hosted-" + kubeControllerManager,
						"component": kubeControllerManager,
						"tier":      "control-plane",
					},
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{kubeadmconstants.LabelNodeRoleMaster: ""},
					HostNetwork:  true,
					Volumes:      volumes,
					Containers: []v1.Container{
						{
							Name:          "self-hosted-" + kubeControllerManager,
							Image:         images.GetCoreImage(images.KubeControllerManagerImage, cfg, kubeadmapi.GlobalEnvParams.HyperkubeImage),
							Command:       getControllerManagerCommand(cfg, true),
							VolumeMounts:  volumeMounts,
							LivenessProbe: componentProbe(10252, "/healthz", v1.URISchemeHTTP),
							Resources:     componentResources("200m"),
							Env:           getProxyEnvVars(),
						},
					},
					Tolerations: []v1.Toleration{kubeadmconstants.MasterToleration},
					DNSPolicy:   v1.DNSClusterFirstWithHostNet,
				},
			},
		},
	}
	return d
}

func getSchedulerDeployment(cfg *kubeadmapi.MasterConfiguration, volumes []v1.Volume, volumeMounts []v1.VolumeMount) ext.Deployment {
	d := ext.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "self-hosted-" + kubeScheduler,
			Namespace: "kube-system",
			Labels:    map[string]string{"k8s-app": "self-hosted-" + kubeScheduler},
		},
		Spec: ext.DeploymentSpec{
			// TODO bootkube uses 2 replicas
			Strategy: ext.DeploymentStrategy{
				Type: ext.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &ext.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app":   "self-hosted-" + kubeScheduler,
						"component": kubeScheduler,
						"tier":      "control-plane",
					},
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{kubeadmconstants.LabelNodeRoleMaster: ""},
					HostNetwork:  true,
					Volumes:      volumes,
					Containers: []v1.Container{
						{
							Name:          "self-hosted-" + kubeScheduler,
							Image:         images.GetCoreImage(images.KubeSchedulerImage, cfg, kubeadmapi.GlobalEnvParams.HyperkubeImage),
							Command:       getSchedulerCommand(cfg, true),
							VolumeMounts:  volumeMounts,
							LivenessProbe: componentProbe(10251, "/healthz", v1.URISchemeHTTP),
							Resources:     componentResources("100m"),
							Env:           getProxyEnvVars(),
						},
					},
					Tolerations: []v1.Toleration{kubeadmconstants.MasterToleration},
					DNSPolicy:   v1.DNSClusterFirstWithHostNet,
				},
			},
		},
	}

	return d
}

func getEtcdClusterRoleBinding() rbac.ClusterRoleBinding {
	// return rbac.ClusterRoleBinding{
	// 	TypeMeta: metav1.TypeMeta{
	// 		APIVersion: "rbac.authorization.k8s.io/v1beta1",
	// 		Kind:       "ClusterRoleBinding",
	// 	},
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: "system:default-sa",
	// 	},
	// 	Subjects: []rbac.Subject{
	// 		rbac.Subject{
	// 			Kind:      "ServiceAccount",
	// 			Name:      "default",
	// 			Namespace: metav1.NamespaceSystem,
	// 		},
	// 	},
	// 	RoleRef: rbac.RoleRef{
	// 		Kind:     "ClusterRole",
	// 		Name:     "cluster-admin",
	// 		APIGroup: "rbac.authorization.k8s.io",
	// 	},
	// }
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
			rbac.Subject{
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
			rbac.PolicyRule{
				APIGroups: []string{"etcd.coreos.com"},
				Resources: []string{"clusters"},
				Verbs:     []string{"*"},
			},
			rbac.PolicyRule{
				APIGroups: []string{"extensions"},
				Resources: []string{"thirdpartyresources"},
				Verbs:     []string{"create"},
			},
			rbac.PolicyRule{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"create"},
			},
			rbac.PolicyRule{
				APIGroups: []string{"extensions"},
				Resources: []string{"replicasets", "deployments"},
				Verbs:     []string{"*"},
			},
			rbac.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "endpoints", "persistentvolumeclaims"},
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
			Namespace: "kube-system",
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
					Tolerations: []v1.Toleration{
						v1.Toleration{
							Key:      "node-role.kubernetes.io/master",
							Operator: "Exists",
							Effect:   "NoSchedule",
						},
					},
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

func buildStaticManifestFilepath(name string) string {
	return path.Join(kubeadmapi.GlobalEnvParams.KubernetesDir, "manifests", name+".yaml")
}
