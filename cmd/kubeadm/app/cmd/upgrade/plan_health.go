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
	"net/http"

	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


type healthCheck struct {
	description, okMessage, failMessage string
	f func(clientset.Interface) error
}

// checkClusterReady makes sure:
// - the API /healthz endpoint is healthy
// - there are DaemonSets with at least one Pod for all control plane components
// - all Nodes are Ready
func checkClusterReady(client clientset.Interface) bool {
	fmt.Println("--> Making sure the cluster is healthy:")

	healthChecks := []healthCheck{
		{
			description: "API Server health",
			okMessage: "Healthy",
			failMessage: "Unhealthy",
			f: apiServerHealthy,
		},
		{
			description: "Node health",
			okMessage: "All Nodes are healthy",
			failMessage: "More than one Node unhealthy",
			f: nodesHealthy,
		},
		{
			description: "Control plane DaemonSet health",
			okMessage: "All control plane DaemonSets are healthy",
			failMessage: "More than one Node unhealthy",
			f: controlPlaneHealth,
		},
	}
	for _, check := range healthChecks {
		fmt.Printf("---> Checking %s: ", check.description)
		err := check.f(client)
		if err == nil {
			fmt.Println(check.okMessage)
		} else {
			fmt.Println(check.failMessage)
			fmt.Println("----> kubeadm can't upgrade your cluster")
			fmt.Printf("----> Reason: %s\n", err)
			return false
		}
	}
	return true
}

func apiServerHealthy(client clientset.Interface) error {
	healthStatus := 0
	client.Discovery().RESTClient().Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
	if healthStatus != http.StatusOK {
		return fmt.Errorf("the API Server is unhealthy; /healthz didn't return %q", "ok")
	}
	return nil
}

func nodesHealthy(client clientset.Interface) error {
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("couldn't list all nodes in cluster")
	}

	notReadyNodes := []string{}
	for _, node := range nodes.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
				notReadyNodes = append(notReadyNodes, node.ObjectMeta.Name)
			}
		}
	}
	if len(notReadyNodes) != 0 {
		return fmt.Errorf("there are NotReady Nodes in the cluster: %v", notReadyNodes)
	}
	return nil
}

func controlPlaneHealth(client clientset.Interface) error {
	masterComponents := []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler"}
	notReadyDaemonSets := []error{}
	for _, component := range masterComponents {
		dsName := "self-hosted-" + component
		ds, err := client.ExtensionsV1beta1().DaemonSets(metav1.NamespaceSystem).Get(dsName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("couldn't list nodes in cluster")
		}
		
		if err := daemonSetHealth(&ds.Status); err != nil {
			notReadyDaemonSets = append(notReadyDaemonSets, fmt.Errorf("DaemonSet %q not healthy: %v", dsName, err))
		}
	}
	if len(notReadyDaemonSets) == len(masterComponents) {
		return fmt.Errorf("the control plane isn't self-hosted: %v", notReadyDaemonSets)
	}
	if len(notReadyDaemonSets) != 0 {
		return fmt.Errorf("there are control plane DaemonSets in the cluster that are not ready: %v", notReadyDaemonSets)
	}
	return nil
}

func daemonSetHealth(dsStatus *extensions.DaemonSetStatus) error {
	if dsStatus.CurrentNumberScheduled != dsStatus.DesiredNumberScheduled {
		return fmt.Errorf("current number of scheduled Pods ('%d') doesn't match the amount of desired Pods ('%d')", dsStatus.CurrentNumberScheduled, dsStatus.DesiredNumberScheduled)
	}
	if dsStatus.NumberAvailable == 0 {
		return fmt.Errorf("no available Pods for DaemonSet")
	}
	if dsStatus.NumberReady == 0 {
		return fmt.Errorf("no ready Pods for DaemonSet")
	}
	return nil
}
