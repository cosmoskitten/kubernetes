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

package scheduling

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = SIGDescribe("[Feature:GPU]", func() {
	f := framework.NewDefaultFramework("device-plugin-gpus-errors")

	BeforeEach(func() {
		if !isClusterRunningCOS(f) {
			Skip("Nvidia GPU tests are supproted only on Container Optimized OS image currently")
		}

		framework.Logf("Cluster is running on COS. Proceeding with test")

		if !areGPUsAvailableOnAllSchedulableNodes(f) {
			installDriver(f)
		}
	})

	It("checks that when Kubelet restarts exclusive GPU assignation to pods is kept. Runs on Container Optimized OS only", func() {
		By("Creating one GPU pod on a node with at least two GPUs")
		n := getNodeWithAtLeastTwoGPUs(f)
		Expect(n).To(Not(Equal(nil)))

		spec := makeCudaAdditionDevicePluginTestPod()
		spec.Spec.NodeName = n.Name

		p1 := f.PodClient().Create(spec)
		f.PodClient().WaitForSuccess(p1.Name, 5*time.Minute)

		By("Restarting Kubelet and creating another GPU pod")
		framework.RestartKubelet(n.Name)

		spec = makeCudaAdditionDevicePluginTestPod()
		spec.Spec.NodeName = n.Name

		p2 := f.PodClient().Create(spec)
		f.PodClient().WaitForSuccess(p2.Name, 5*time.Minute)

		cmd := fmt.Sprintf("exec %s %s nvidia-smi -L", n.Name, p1.Spec.Containers[0].Name)
		uuid1, _ := framework.RunKubectl(cmd)

		cmd = fmt.Sprintf("exec %s %s nvidia-smi -L", n.Name, p2.Spec.Containers[0].Name)
		uuid2, _ := framework.RunKubectl(cmd)

		By("Checking that pods got a different GPU")
		Expect(uuid1).To(Not(Equal(uuid2)))
	})
})

func installDriver(f *framework.Framework) {
	dsYamlUrl := "https://raw.githubusercontent.com/ContainerEngine/accelerators/master/cos-nvidia-gpu-installer/daemonset.yaml"

	ds := dsFromManifest(dsYamlUrl)
	ds.Namespace = f.Namespace.Name

	_, err := f.ClientSet.Extensions().DaemonSets(f.Namespace.Name).Create(ds)
	framework.ExpectNoError(err, "failed to create daemonset")

	framework.Logf("Successfully created daemonset to install Nvidia drivers. Waiting for drivers to be installed and GPUs to be available in Node Capacity...")

	// Wait for Nvidia GPUs to be available on nodes
	Eventually(func() bool { return areGPUsAvailableOnAllSchedulableNodes(f) },
		driverInstallTimeout, time.Second).Should(BeTrue())
}

func getNodeWithAtLeastTwoGPUs(f *framework.Framework) *v1.Node {
	nodeList, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	framework.ExpectNoError(err, "getting node list")

	for _, node := range nodeList.Items {
		if node.Spec.Unschedulable {
			continue
		}

		if val, ok := node.Status.Capacity[gpuResourceName]; !ok || val.Value() <= 1 {
			continue
		}

		return &node
	}

	return nil
}
