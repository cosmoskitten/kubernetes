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
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/api/scheduling/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	_ "github.com/stretchr/testify/assert"
)

var _ = SIGDescribe("SchedulerPreemption [Serial]", func() {
	var cs clientset.Interface
	var nodeList *v1.NodeList
	var ns string
	f := framework.NewDefaultFramework("sched-preemption")

	lowPriority, highPriority := int32(100), int32(1000)
	lowPriorityClassName := f.BaseName + "-low-priority"
	highPriorityClassName := f.BaseName + "-high-priority"

	AfterEach(func() {
	})

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace.Name
		nodeList = &v1.NodeList{}

		pcs, err := cs.SchedulingV1alpha1().PriorityClasses().List(metav1.ListOptions{})
		framework.ExpectNoError(err)
		if len(pcs.Items) == 0 {
			_, err := f.ClientSet.SchedulingV1alpha1().PriorityClasses().Create(&v1alpha1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: highPriorityClassName}, Value: highPriority})
			framework.ExpectNoError(err)
			_, err = f.ClientSet.SchedulingV1alpha1().PriorityClasses().Create(&v1alpha1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: lowPriorityClassName}, Value: lowPriority})
			framework.ExpectNoError(err)
		}

		framework.WaitForAllNodesHealthy(cs, time.Minute)
		masterNodes, nodeList = framework.GetMasterAndWorkerNodesOrDie(cs)

		err = framework.CheckTestingNSDeletedExcept(cs, ns)
		framework.ExpectNoError(err)
	})

	// This test verifies that when a higher priority pod is created and no node with
	// enough resources is found, scheduler preempts a lower priority pod to schedule
	// the high priority pod.
	It("validates basic preemption works", func() {
		var podRes v1.ResourceList
		// Create one pod per node that uses a lot of the node's resources.
		By("Starting pods that use 60% of node resources.")
		for _, node := range nodeList.Items {
			cpuAllocatable, found := node.Status.Allocatable["cpu"]
			Expect(found).To(Equal(true))
			milliCPU := cpuAllocatable.MilliValue() * 40 / 100
			memAllocatable, found := node.Status.Allocatable["memory"]
			Expect(found).To(Equal(true))
			memory := memAllocatable.Value() * 60 / 100
			podRes = v1.ResourceList{}
			podRes[v1.ResourceCPU] = *resource.NewMilliQuantity(int64(milliCPU), resource.DecimalSI)
			podRes[v1.ResourceMemory] = *resource.NewQuantity(int64(memory), resource.BinarySI)

			createPausePod(f, pausePodConfig{
				Name:              node.Name + "-pod",
				PriorityClassName: lowPriorityClassName,
				Resources: &v1.ResourceRequirements{
					Requests: podRes,
				},
			})
			framework.Logf("Node: %v", node)
		}
		currentlyScheduledPods := framework.WaitForStableCluster(cs, masterNodes)
		Expect(currentlyScheduledPods).To(Equal(len(nodeList.Items)))

		By("Starting a high priority pod that use 60% of a node resources.")
		// Create a high priority pod and make sure it is scheduled.
		runPausePod(f, pausePodConfig{
			Name:              "preemptor-pod",
			PriorityClassName: highPriorityClassName,
			Resources: &v1.ResourceRequirements{
				Requests: podRes,
			},
		})
	})

})
