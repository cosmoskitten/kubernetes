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

package e2e_node

import (
	"fmt"
	"os/exec"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
)

const (
	testPodNamePrefix = "nvidia-gpu-"
	// Nvidia driver installation can take upwards of 5 minutes.
	driverInstallTimeout = 10 * time.Minute
)

var _ = framework.KubeDescribe("[Feature:GPUDevicePlugin]", func() {
	f := framework.NewDefaultFramework("device-plugin-gpus-errors")

	// Install NVIDIA driver if cluster is running COS
	BeforeEach(func() {
		if !framework.IsNodeRunningCOS(getLocalNode(f)) {
			Skip("NVIDIA GPU tests are supported only on Container Optimized OS image currently")
		}
		if !checkIfNvidiaGPUsExistOnNode() {
			Skip("Nvidia GPUs do not exist on the node. Skipping test.")
		}

		framework.Logf("Cluster is running on COS. Proceeding with test")

		if framework.NumberOfNVIDIAGPUs(getLocalNode(f)) == 0 {
			return
		}

		installDriver(f)

		// Wait for Nvidia GPUs to be available on nodes
		Eventually(func() bool { return framework.NumberOfNVIDIAGPUs(getLocalNode(f)) > 0 },
			driverInstallTimeout*2, time.Second).Should(BeTrue())
	})

	It("checks that when Kubelet restarts exclusive GPU assignation to pods is kept.", func() {
		By("Creating one GPU pod on a node with at least two GPUs")
		n := getLocalNode(f)

		if framework.NumberOfNVIDIAGPUs(getLocalNode(f)) < 2 {
			Skip("Not enough GPUs to execute this test (at least two needed)")
		}

		p1 := f.PodClient().CreateSync(makeCudaPauseImage())

		By("Restarting Kubelet and creating another GPU pod")
		restartKubelet(f)

		p2 := f.PodClient().CreateSync(makeCudaPauseImage())

		cmd := fmt.Sprintf("exec %s %s nvidia-smi -L", n.Name, p1.Spec.Containers[0].Name)
		uuid1, _ := framework.RunKubectl(cmd)

		cmd = fmt.Sprintf("exec %s %s nvidia-smi -L", n.Name, p2.Spec.Containers[0].Name)
		uuid2, _ := framework.RunKubectl(cmd)

		By("Checking that pods got a different GPU")
		Expect(uuid1).To(Not(Equal(uuid2)))

		f.PodClient().DeleteSync(p1.Name, &metav1.DeleteOptions{}, framework.DefaultPodDeletionTimeout)
		f.PodClient().DeleteSync(p2.Name, &metav1.DeleteOptions{}, framework.DefaultPodDeletionTimeout)
	})
})

func installDriver(f *framework.Framework) {
	dsYamlUrl := "https://raw.githubusercontent.com/GoogleCloudPlatform/container-engine-accelerators/master/device-plugin-daemonset.yaml"

	ds := framework.DsFromManifest(dsYamlUrl)
	p := &v1.Pod{
		Spec: ds.Spec.Template.Spec,
	}

	f.PodClient().CreateSync(p)
	framework.Logf("Successfully created NVIDIA driver pod installer. Waiting for drivers to be installed and GPUs to be available in Node Capacity...")
}

func makeCudaPauseImage() *v1.Pod {
	podName := testPodNamePrefix + string(uuid.NewUUID())

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{{
				Name:    "cuda-pause",
				Image:   "nvidia/cuda",
				Command: []string{"sleep", "infinity"},

				Resources: v1.ResourceRequirements{
					Limits:   newDecimalResourceList(framework.NVIDIAGPUResourceName, 1),
					Requests: newDecimalResourceList(framework.NVIDIAGPUResourceName, 1),
				},
			}},
		},
	}
}

func newDecimalResourceList(name v1.ResourceName, quantity int64) v1.ResourceList {
	return v1.ResourceList{name: *resource.NewQuantity(quantity, resource.DecimalSI)}
}

// TODO: Find a uniform way to deal with systemctl/initctl/service operations. #34494
func restartKubelet(f *framework.Framework) {
	stdout1, err1 := exec.Command("sudo", "systemctl", "restart", "kubelet").CombinedOutput()
	if err1 == nil {
		return
	}

	stdout2, err2 := exec.Command("sudo", "/etc/init.d/kubelet", "restart").CombinedOutput()
	if err2 == nil {
		return
	}

	stdout3, err3 := exec.Command("sudo", "service", "kubelet", "restart").CombinedOutput()
	if err3 == nil {
		return
	}

	framework.Failf("Failed to trigger kubelet restart with systemctl/initctl/service operations:"+
		"\nsystemclt: %v, %v"+
		"\ninitctl:   %v, %v"+
		"\nservice:   %v, %v", err1, stdout1, err2, stdout2, err3, stdout3)
}
