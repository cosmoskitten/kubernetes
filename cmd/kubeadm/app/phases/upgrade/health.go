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
	"os"

	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

// healthCheck is a helper struct for easily performing healthchecks against the cluster and printing the output
type healthCheck struct {
	description, okMessage, failMessage string
	// f is invoked with a k8s client passed to it. Should return an optional warning and/or an error
	f func(clientset.Interface) error
}

// VerifyClusterHealth makes sure:
// - the API /healthz endpoint is healthy
// - all Nodes are Ready
// - (if self-hosted) that there are DaemonSets with at least one Pod for all control plane components
// - (if static pod-hosted) that all required Static Pod manifests exist on disk
func VerifyClusterHealth(client clientset.Interface) bool {
	fmt.Println("[upgrade] Making sure the cluster is healthy:")

	defaultHealthChecks := []healthCheck{
		{
			description: "API Server health",
			okMessage:   "Healthy",
			failMessage: "Unhealthy",
			f:           apiServerHealthy,
		},
		{
			description: "Node health",
			okMessage:   "All Nodes are healthy",
			failMessage: "More than one Node unhealthy",
			f:           nodesHealthy,
		},
		// TODO: Add a check for ComponentStatuses here?
	}

	success := runHealthChecks(client, defaultHealthChecks)
	if !success {
		return false
	}

	// If the control plane is self-hosted, we should run more health checks
	fmt.Printf("[upgrade/health] Checking if control plane is Static Pod-hosted or Self-Hosted: ")
	if IsControlPlaneSelfHosted(client) {

		fmt.Println("Self-Hosted.")

		// Run an extra pair of health checks to make sure the self-hosted control plane is healthy
		success = runHealthChecks(client, []healthCheck{
			{
				description: "Control plane DaemonSet health",
				okMessage:   "All control plane DaemonSets are healthy",
				failMessage: "More than one control plane DaemonSet unhealthy",
				f:           controlPlaneHealth,
			},
		})
		if !success {
			return false
		}
	} else {
		fmt.Println("Static Pod-hosted.")
		fmt.Println("[upgrade/health] NOTE: kubeadm will upgrade your Static Pod-hosted control plane to a Self-Hosted one when upgrading if --feature-gates=SelfHosting=true is set (which is the default)")
		fmt.Println("[upgrade/health] If you strictly want to continue using a Static Pod-hosted control plane, set --feature-gates=SelfHosting=true when running 'kubeadm upgrade apply'")

		success = runHealthChecks(client, []healthCheck{
			{
				description: "Static Pod manifests exists on disk",
				okMessage:   "All required Static Pod manifests exist on disk",
				failMessage: "Not all required Static Pod manifests exist on disk",
				f:           staticPodManifestHealth,
			},
		})
		if !success {
			return false
		}
	}

	return true
}

func runHealthChecks(client clientset.Interface, healthChecks []healthCheck) bool {

	for _, check := range healthChecks {
		fmt.Printf("[upgrade/health] Checking %s: ", check.description)
		err := check.f(client)
		if err != nil {
			fmt.Println(check.failMessage)
			fmt.Println("[upgrade/health] FATAL: kubeadm can't upgrade your cluster")
			fmt.Printf("[upgrade/health] Reason: %s\n", err)
			return false
		}

		fmt.Println(check.okMessage)
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

	notReadyNodes := getNotReadyNodes(nodes.Items)
	if len(notReadyNodes) != 0 {
		return fmt.Errorf("there are NotReady Nodes in the cluster: %v", notReadyNodes)
	}
	return nil
}

func controlPlaneHealth(client clientset.Interface) error {
	notReadyDaemonSets, err := getNotReadyDaemonSets(client)
	if err != nil {
		return err
	}

	if len(notReadyDaemonSets) != 0 {
		return fmt.Errorf("there are control plane DaemonSets in the cluster that are not ready: %v", notReadyDaemonSets)
	}
	return nil
}

func staticPodManifestHealth(_ clientset.Interface) error {
	nonExistentManifests := []string{}
	for _, component := range constants.MasterComponents {
		manifestFile := constants.GetStaticPodFilepath(component, constants.GetStaticPodDirectory())
		if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
			nonExistentManifests = append(nonExistentManifests, manifestFile)
		}
	}
	if len(nonExistentManifests) == 0 {
		return nil
	}
	return fmt.Errorf("The control plane seems to be Static Pod-hosted, but some of the manifests don't seem to exist on disk. This probably means you're running 'kubeadm upgrade' on a remote machine, which is not supported for a Static Pod-hosted cluster. Manifest files not found: %v", nonExistentManifests)
}

// IsControlPlaneSelfHosted returns whether the control plane is self hosted or not
func IsControlPlaneSelfHosted(client clientset.Interface) bool {
	notReadyDaemonSets, err := getNotReadyDaemonSets(client)
	if err != nil {
		return false
	}

	// If there are no NotReady DaemonSets, we are using self-hosting
	return len(notReadyDaemonSets) == 0
}

func getNotReadyDaemonSets(client clientset.Interface) ([]error, error) {
	notReadyDaemonSets := []error{}
	for _, component := range constants.MasterComponents {
		dsName := constants.AddSelfHostedPrefix(component)
		ds, err := client.ExtensionsV1beta1().DaemonSets(metav1.NamespaceSystem).Get(dsName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("couldn't get daemonset %q in the %s namespace", dsName, metav1.NamespaceSystem)
		}

		if err := daemonSetHealth(&ds.Status); err != nil {
			notReadyDaemonSets = append(notReadyDaemonSets, fmt.Errorf("DaemonSet %q not healthy: %v", dsName, err))
		}
	}
	return notReadyDaemonSets, nil
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

func getNotReadyNodes(nodes []v1.Node) []string {
	notReadyNodes := []string{}
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
				notReadyNodes = append(notReadyNodes, node.ObjectMeta.Name)
			}
		}
	}
	return notReadyNodes
}
