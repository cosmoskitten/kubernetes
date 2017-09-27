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

package storage

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/vsphere"
	"k8s.io/kubernetes/test/e2e/framework"
)

/*
	Test performs following operations

	Steps
	1. Create a storage class with thin diskformat.
	2. Create nginx service.
	3. Create nginx statefulsets with 3 replicas.
	4. Wait until all Pods are ready and PVCs are bounded with PV.
	5. Verify volumes are accessible in all statefulsets pods with creating empty file.
	6. Scale down statefulsets to 2 replicas.
	7. Scale up statefulsets to 4 replicas.
	8. Scale down statefulsets to 0 replicas and delete all pods.
	9. Delete all PVCs from the test namespace.
	10. Delete the storage class.
*/

const (
	manifestPath     = "test/e2e/testing-manifests/statefulset/nginx"
	mountPath        = "/usr/share/nginx/html"
	storageclassname = "nginx-sc"
)

var _ = SIGDescribe("vsphere statefulsets", func() {
	f := framework.NewDefaultFramework("vsphere-statefulsets")
	var (
		namespace string
		client    clientset.Interface
	)
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("vsphere")
		namespace = f.Namespace.Name
		client = f.ClientSet
	})
	AfterEach(func() {
		framework.Logf("Deleting all statefulset in namespace: %v", namespace)
		framework.DeleteAllStatefulSets(client, namespace)
	})

	It("vsphere statefulsets testing", func() {
		By("Creating StorageClass for Statefulsets")
		scParameters := make(map[string]string)
		scParameters["diskformat"] = "thin"
		scSpec := getVSphereStorageClassSpec(storageclassname, scParameters)
		sc, err := client.StorageV1().StorageClasses().Create(scSpec)
		Expect(err).NotTo(HaveOccurred())
		defer client.StorageV1().StorageClasses().Delete(sc.Name, nil)

		By("Creating statefulsets with number of Replica: 3")
		statefulsetsTester := framework.NewStatefulSetTester(client)
		statefulsets := statefulsetsTester.CreateStatefulSet(manifestPath, namespace)
		// Waiting for pods status to be Ready
		ssPods := statefulsetsTester.GetPodList(statefulsets)
		Expect(ssPods.Items).NotTo(BeEmpty(), fmt.Sprintf("Unable to get list of Pods from the Statefulset: %v", statefulsets.Name))
		statefulsetsTester.WaitForStatusReadyReplicas(statefulsets, 3)
		Expect(statefulsetsTester.CheckMount(statefulsets, mountPath)).NotTo(HaveOccurred())

		By("Scaling down statefulsets to number of Replica: 2")
		_, scaledownErr := statefulsetsTester.Scale(statefulsets, 2)
		Expect(scaledownErr).NotTo(HaveOccurred())
		statefulsetsTester.WaitForStatusReadyReplicas(statefulsets, 2)

		vsp, err := vsphere.GetVSphere()
		Expect(err).NotTo(HaveOccurred())

		// After scale down, verify vsphere volumes are detached from deleted pods
		By("Verify Volumes are detached from Nodes after Statefulsets is scaled down")
		for _, sspod := range ssPods.Items {
			_, err := client.CoreV1().Pods(namespace).Get(sspod.Name, metav1.GetOptions{})
			if err != nil {
				Expect(apierrs.IsNotFound(err), BeTrue())
				for _, volumespec := range sspod.Spec.Volumes {
					if volumespec.PersistentVolumeClaim != nil {
						vSpherediskPath := getvSphereVolumePathFromClaim(client, statefulsets.Namespace, volumespec.PersistentVolumeClaim.ClaimName)
						framework.Logf("Waiting for Volume: %q to detach from Node: %q", vSpherediskPath, sspod.Spec.NodeName)
						Expect(waitForVSphereDiskToDetach(vsp, vSpherediskPath, types.NodeName(sspod.Spec.NodeName))).NotTo(HaveOccurred())
					}
				}
			}
		}

		By("Scaling up statefulsets to number of Replica: 4")
		_, scaleupErr := statefulsetsTester.Scale(statefulsets, 4)
		Expect(scaleupErr).NotTo(HaveOccurred())
		statefulsetsTester.WaitForStatusReplicas(statefulsets, 4)
		statefulsetsTester.WaitForStatusReadyReplicas(statefulsets, 4)

		ssPodsAfterScaleUp := statefulsetsTester.GetPodList(statefulsets)
		Expect(ssPodsAfterScaleUp.Items).NotTo(BeEmpty(), fmt.Sprintf("Unable to get list of Pods from the Statefulset: %v", statefulsets.Name))

		// After scale up, verify all vsphere volumes are attached to node VMs.
		By("Verify all volumes are attached to Nodes after Statefulsets is scaled up")
		for _, sspod := range ssPodsAfterScaleUp.Items {
			err := framework.WaitForPodsReady(client, statefulsets.Namespace, sspod.Name, 0)
			Expect(err).NotTo(HaveOccurred())
			pod, err := client.CoreV1().Pods(namespace).Get(sspod.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			for _, volumespec := range pod.Spec.Volumes {
				if volumespec.PersistentVolumeClaim != nil {
					vSpherediskPath := getvSphereVolumePathFromClaim(client, statefulsets.Namespace, volumespec.PersistentVolumeClaim.ClaimName)
					framework.Logf("Verify Volume: %q is attached to the Node: %q", vSpherediskPath, sspod.Spec.NodeName)
					isVolumeAttached, verifyDiskAttachedError := verifyVSphereDiskAttached(vsp, vSpherediskPath, types.NodeName(sspod.Spec.NodeName))
					Expect(isVolumeAttached).To(BeTrue())
					Expect(verifyDiskAttachedError).NotTo(HaveOccurred())
				}
			}
		}
	})
})
