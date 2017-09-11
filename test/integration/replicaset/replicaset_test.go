/*
Copyright 2015 The Kubernetes Authors.

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

package replicaset

import (
	"fmt"
	"net/http/httptest"
	//"reflect"
	//"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	//"k8s.io/apimachinery/pkg/types"
	//"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	// api "k8s.io/kubernetes/pkg/api"
	// podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	// apiextensions "k8s.io/kubernetes/pkg/apis/extensions"
	// "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller/replicaset"
	"k8s.io/kubernetes/test/integration/framework"
)

const (
	interval = 100 * time.Millisecond
	timeout  = 1 * time.Second
)

func testLabels() map[string]string {
	return map[string]string{"name": "test"}
}

func newRS(name, namespace string, replicas int) *v1beta1.ReplicaSet {
	replicasCopy := int32(replicas)
	return &v1beta1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			//UID:       uuid.NewUUID(),
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1beta1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: testLabels(),
			},
			Replicas: &replicasCopy,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: testLabels(),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "fake-name",
							Image: "fakeimage",
						},
					},
				},
			},
		},
	}
}

func newMatchingPod(podName, namespace string) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    testLabels(),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "fake-name",
					Image: "fakeimage",
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}
}

// verifyRemainingObjects verifies if the number of the remaining replica
// sets and pods are rsNum and podNum. It returns error if the
// communication with the API server fails.
func verifyRemainingObjects(t *testing.T, clientSet clientset.Interface, namespace string, rsNum, podNum int) (bool, error) {
	rsClient := clientSet.Extensions().ReplicaSets(namespace)
	podClient := clientSet.Core().Pods(namespace)
	pods, err := podClient.List(metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("Failed to list pods: %v", err)
	}
	var ret = true
	if len(pods.Items) != podNum {
		ret = false
		t.Logf("expect %d pods, got %d pods", podNum, len(pods.Items))
	}
	rss, err := rsClient.List(metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("Failed to list replica sets: %v", err)
	}
	if len(rss.Items) != rsNum {
		ret = false
		t.Logf("expect %d RSs, got %d RSs", rsNum, len(rss.Items))
	}
	return ret, nil
}

func rmSetup(t *testing.T) (*httptest.Server, framework.CloseFunc, *replicaset.ReplicaSetController, informers.SharedInformerFactory, clientset.Interface) {
	masterConfig := framework.NewIntegrationTestMasterConfig()
	_, s, closeFn := framework.RunAMaster(masterConfig)

	config := restclient.Config{Host: s.URL}
	clientSet, err := clientset.NewForConfig(&config)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}
	resyncPeriod := 12 * time.Hour
	informers := informers.NewSharedInformerFactory(clientset.NewForConfigOrDie(restclient.AddUserAgent(&config, "rs-informers")), resyncPeriod)

	rm := replicaset.NewReplicaSetController(
		informers.Extensions().V1beta1().ReplicaSets(),
		informers.Core().V1().Pods(),
		clientset.NewForConfigOrDie(restclient.AddUserAgent(&config, "replicaset-controller")),
		replicaset.BurstReplicas,
	)

	if err != nil {
		t.Fatalf("Failed to create replicaset controller")
	}
	return s, closeFn, rm, informers, clientSet
}

func rmSimpleSetup(t *testing.T) (*httptest.Server, framework.CloseFunc, clientset.Interface) {
	masterConfig := framework.NewIntegrationTestMasterConfig()
	_, s, closeFn := framework.RunAMaster(masterConfig)

	config := restclient.Config{Host: s.URL}
	clientSet, err := clientset.NewForConfig(&config)
	if err != nil {
		t.Fatalf("Error in create clientset: %v", err)
	}
	return s, closeFn, clientSet
}

// wait for the podInformer to observe the pods. Call this function before
// running the RS controller to prevent the rc manager from creating new pods
// rather than adopting the existing ones.
func waitToObservePods(t *testing.T, podInformer cache.SharedIndexInformer, podNum int) {
	if err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		objects := podInformer.GetIndexer().List()
		if len(objects) == podNum {
			return true, nil
		} else {
			return false, nil
		}
	}); err != nil {
		t.Fatal(err)
	}
}

/*
func TestAdoption(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	testCases := []struct {
		name                    string
		existingOwnerReferences func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference
		expectedOwnerReferences func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference
	}{
		{
			"pod refers rs as an owner, not a controller",
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{{UID: rs.UID, Name: rs.Name, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet"}}
			},
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{{UID: rs.UID, Name: rs.Name, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Controller: boolPtr(true), BlockOwnerDeletion: boolPtr(true)}}
			},
		},
		{
			"pod doesn't have owner references",
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{}
			},
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{{UID: rs.UID, Name: rs.Name, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Controller: boolPtr(true), BlockOwnerDeletion: boolPtr(true)}}
			},
		},
		{
			"pod refers rs as a controller",
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{{UID: rs.UID, Name: rs.Name, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Controller: boolPtr(true)}}
			},
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{{UID: rs.UID, Name: rs.Name, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Controller: boolPtr(true)}}
			},
		},
		{
			"pod refers other rs as the controller, refers the rs as an owner",
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{
					{UID: "1", Name: "anotherRS", APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Controller: boolPtr(true)},
					{UID: rs.UID, Name: rs.Name, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet"},
				}
			},
			func(rs *v1beta1.ReplicaSet) []metav1.OwnerReference {
				return []metav1.OwnerReference{
					{UID: "1", Name: "anotherRS", APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Controller: boolPtr(true)},
					{UID: rs.UID, Name: rs.Name, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet"},
				}
			},
		},
	}
	for i, tc := range testCases {
		s, closeFn, rm, informers, clientSet := rmSetup(t)
		defer closeFn()
		podInformer := informers.Core().V1().Pods().Informer()
		ns := framework.CreateTestingNamespace(fmt.Sprintf("rs-adoption-%d", i), s, t)
		defer framework.DeleteTestingNamespace(ns, s, t)

		rsClient := clientSet.Extensions().ReplicaSets(ns.Name)
		podClient := clientSet.Core().Pods(ns.Name)
		const rsName = "rs"
		rs, err := rsClient.Create(newRS(rsName, ns.Name, 1))
		if err != nil {
			t.Fatalf("Failed to create replica set: %v", err)
		}
		podName := fmt.Sprintf("pod%d", i)
		pod := newMatchingPod(podName, ns.Name)
		pod.OwnerReferences = tc.existingOwnerReferences(rs)
		_, err = podClient.Create(pod)
		if err != nil {
			t.Fatalf("Failed to create Pod: %v", err)
		}

		stopCh := make(chan struct{})
		informers.Start(stopCh)
		waitToObservePods(t, podInformer, 1)
		go rm.Run(5, stopCh)
		if err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
			updatedPod, err := podClient.Get(pod.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if e, a := tc.expectedOwnerReferences(rs), updatedPod.OwnerReferences; reflect.DeepEqual(e, a) {
				return true, nil
			} else {
				t.Logf("ownerReferences don't match, expect %v, got %v", e, a)
				return false, nil
			}
		}); err != nil {
			t.Fatalf("test %q failed: %v", tc.name, err)
		}
		close(stopCh)
	}
}
*/
func createRSsPods(t *testing.T, clientSet clientset.Interface, rss []*v1beta1.ReplicaSet, pods []*v1.Pod, ns string) ([]*v1beta1.ReplicaSet, []*v1.Pod) {
	rsClient := clientSet.Extensions().ReplicaSets(ns)
	podClient := clientSet.Core().Pods(ns)
	var createdRSs []*v1beta1.ReplicaSet
	var createdPods []*v1.Pod
	for _, rs := range rss {
		createdRS, err := rsClient.Create(rs)
		if err != nil {
			t.Fatalf("Failed to create replica set %s: %v", rs.Name, err)
		}
		createdRSs = append(createdRSs, createdRS)
	}
	for _, pod := range pods {
		createdPod, err := podClient.Create(pod)
		if err != nil {
			t.Fatalf("Failed to create pod %s: %v", pod.Name, err)
		}
		createdPods = append(createdPods, createdPod)
	}

	return createdRSs, createdPods
}

func waitRSStable(t *testing.T, clientSet clientset.Interface, rs *v1beta1.ReplicaSet, ns string) {
	rsClient := clientSet.Extensions().ReplicaSets(ns)
	if err := wait.Poll(10*time.Second, 15*time.Second, func() (bool, error) {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if rs.Status.Replicas != *rs.Spec.Replicas {
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		t.Fatal(err, rs.Status.Replicas, *rs.Spec.Replicas)
	}
}

/*
func TestUpdateSelectorToAdopt(t *testing.T) {
	// We have pod1, pod2 and rs. rs.spec.replicas=1. At first rs.Selector
	// matches pod1 only; change the selector to match pod2 as well. Verify
	// there is only one pod left.
	s, closeFn, rm, informers, clientSet := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("rs-update-selector-to-adopt", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	rs := newRS("rs", ns.Name, 1)
	// let rs's selector only match pod1
	rs.Spec.Selector.MatchLabels["uniqueKey"] = "1"
	rs.Spec.Template.Labels["uniqueKey"] = "1"
	pod1 := newMatchingPod("pod1", ns.Name)
	pod1.Labels["uniqueKey"] = "1"
	pod2 := newMatchingPod("pod2", ns.Name)
	pod2.Labels["uniqueKey"] = "2"
	createRSsPods(t, clientSet, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{pod1, pod2}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	waitRSStable(t, clientSet, rs, ns.Name)

	// change the rs's selector to match both pods
	patch := `{"spec":{"selector":{"matchLabels": {"uniqueKey":null}}}}`
	rsClient := clientSet.Extensions().ReplicaSets(ns.Name)
	rs, err := rsClient.Patch(rs.Name, types.StrategicMergePatchType, []byte(patch))
	if err != nil {
		t.Fatalf("Failed to patch replica set: %v", err)
	}
	t.Logf("patched rs = %#v", rs)
	// wait for the rs select both pods and delete one of them
	if err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		return verifyRemainingObjects(t, clientSet, ns.Name, 1, 1)
	}); err != nil {
		t.Fatal(err)
	}
	close(stopCh)
}

func TestUpdateSelectorToRemoveControllerRef(t *testing.T) {
	// We have pod1, pod2 and rs. rs.spec.replicas=2. At first rs.Selector
	// matches pod1 and pod2; change the selector to match only pod1. Verify
	// that rs creates one more pod, so there are 3 pods. Also verify that
	// pod2's controllerRef is cleared.
	s, closeFn, rm, informers, clientSet := rmSetup(t)
	defer closeFn()
	podInformer := informers.Core().V1().Pods().Informer()
	ns := framework.CreateTestingNamespace("rs-update-selector-to-remove-controllerref", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	rs := newRS("rs", ns.Name, 2)
	pod1 := newMatchingPod("pod1", ns.Name)
	pod1.Labels["uniqueKey"] = "1"
	pod2 := newMatchingPod("pod2", ns.Name)
	pod2.Labels["uniqueKey"] = "2"
	createRSsPods(t, clientSet, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{pod1, pod2}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	waitToObservePods(t, podInformer, 2)
	go rm.Run(5, stopCh)
	waitRSStable(t, clientSet, rs, ns.Name)

	// change the rs's selector to match both pods
	patch := `{"spec":{"selector":{"matchLabels": {"uniqueKey":"1"}},"template":{"metadata":{"labels":{"uniqueKey":"1"}}}}}`
	rsClient := clientSet.Extensions().ReplicaSets(ns.Name)
	rs, err := rsClient.Patch(rs.Name, types.StrategicMergePatchType, []byte(patch))
	if err != nil {
		t.Fatalf("Failed to patch replica set: %v", err)
	}
	t.Logf("patched rs = %#v", rs)
	// wait for the rs to create one more pod
	if err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		return verifyRemainingObjects(t, clientSet, ns.Name, 1, 3)
	}); err != nil {
		t.Fatal(err)
	}
	podClient := clientSet.Core().Pods(ns.Name)
	pod2, err = podClient.Get(pod2.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get pod2: %v", err)
	}
	if len(pod2.OwnerReferences) != 0 {
		t.Fatalf("ownerReferences of pod2 is not cleared, got %#v", pod2.OwnerReferences)
	}
	close(stopCh)
}

func TestUpdateLabelToRemoveControllerRef(t *testing.T) {
	// We have pod1, pod2 and rs. rs.spec.replicas=2. At first rs.Selector
	// matches pod1 and pod2; change pod2's labels to non-matching. Verify
	// that rs creates one more pod, so there are 3 pods. Also verify that
	// pod2's controllerRef is cleared.
	s, closeFn, rm, informers, clientSet := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("rs-update-label-to-remove-controllerref", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	rs := newRS("rs", ns.Name, 2)
	pod1 := newMatchingPod("pod1", ns.Name)
	pod2 := newMatchingPod("pod2", ns.Name)
	createRSsPods(t, clientSet, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{pod1, pod2}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	waitRSStable(t, clientSet, rs, ns.Name)

	// change the rs's selector to match both pods
	patch := `{"metadata":{"labels":{"name":null}}}`
	podClient := clientSet.Core().Pods(ns.Name)
	pod2, err := podClient.Patch(pod2.Name, types.StrategicMergePatchType, []byte(patch))
	if err != nil {
		t.Fatalf("Failed to patch pod2: %v", err)
	}
	t.Logf("patched pod2 = %#v", pod2)
	// wait for the rs to create one more pod
	if err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		return verifyRemainingObjects(t, clientSet, ns.Name, 1, 3)
	}); err != nil {
		t.Fatal(err)
	}
	pod2, err = podClient.Get(pod2.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get pod2: %v", err)
	}
	if len(pod2.OwnerReferences) != 0 {
		t.Fatalf("ownerReferences of pod2 is not cleared, got %#v", pod2.OwnerReferences)
	}
	close(stopCh)
}

func TestUpdateLabelToBeAdopted(t *testing.T) {
	// We have pod1, pod2 and rs. rs.spec.replicas=1. At first rs.Selector
	// matches pod1 only; change pod2's labels to be matching. Verify the RS
	// controller adopts pod2 and delete one of them, so there is only 1 pod
	// left.
	s, closeFn, rm, informers, clientSet := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("rs-update-label-to-be-adopted", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	rs := newRS("rs", ns.Name, 1)
	// let rs's selector only matches pod1
	rs.Spec.Selector.MatchLabels["uniqueKey"] = "1"
	rs.Spec.Template.Labels["uniqueKey"] = "1"
	pod1 := newMatchingPod("pod1", ns.Name)
	pod1.Labels["uniqueKey"] = "1"
	pod2 := newMatchingPod("pod2", ns.Name)
	pod2.Labels["uniqueKey"] = "2"
	createRSsPods(t, clientSet, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{pod1, pod2}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	waitRSStable(t, clientSet, rs, ns.Name)

	// change the rs's selector to match both pods
	patch := `{"metadata":{"labels":{"uniqueKey":"1"}}}`
	podClient := clientSet.Core().Pods(ns.Name)
	pod2, err := podClient.Patch(pod2.Name, types.StrategicMergePatchType, []byte(patch))
	if err != nil {
		t.Fatalf("Failed to patch pod2: %v", err)
	}
	t.Logf("patched pod2 = %#v", pod2)
	// wait for the rs to select both pods and delete one of them
	if err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		return verifyRemainingObjects(t, clientSet, ns.Name, 1, 1)
	}); err != nil {
		t.Fatal(err)
	}
	close(stopCh)
}

// selectors are IMMUTABLE for all API versions except extensions/v1beta1
func TestRSSelectorImmutability(t *testing.T) {
	s, closeFn, clientSet := rmSimpleSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("rs-selector-immutability", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	rs := newRS("rs", ns.Name, 0)
	createRSsPods(t, clientSet, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{}, ns.Name)

	// test to ensure extensions/v1beta1 selector is mutable
	newSelectorLabels := map[string]string{"changed_name_extensions_v1beta1": "changed_test_extensions_v1beta1"}
	rs.Spec.Selector.MatchLabels = newSelectorLabels
	rs.Spec.Template.Labels = newSelectorLabels
	replicaset, err := clientSet.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update extensions/v1beta1 replicaset %s: %v", replicaset.Name, err)
	}
	if !reflect.DeepEqual(replicaset.Spec.Selector.MatchLabels, newSelectorLabels) {
		t.Errorf("selector should be changed for extensions/v1beta1, expected: %v, got: %v", newSelectorLabels, replicaset.Spec.Selector.MatchLabels)
	}

	// test to ensure apps/v1beta2 selector is immutable
	rsV1beta2, err := clientSet.AppsV1beta2().ReplicaSets(ns.Name).Get(replicaset.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get apps/v1beta2 replicaset %s: %v", replicaset.Name, err)
	}
	newSelectorLabels = map[string]string{"changed_name_apps_v1beta2": "changed_test_apps_v1beta2"}
	rsV1beta2.Spec.Selector.MatchLabels = newSelectorLabels
	rsV1beta2.Spec.Template.Labels = newSelectorLabels
	_, err = clientSet.AppsV1beta2().ReplicaSets(ns.Name).Update(rsV1beta2)
	if err == nil {
		t.Fatalf("failed to provide validation error when changing immutable selector when updating apps/v1beta2 replicaset %s", rsV1beta2.Name)
	}
	expectedErrType := "Invalid value"
	expectedErrDetail := "field is immutable"
	if !strings.Contains(err.Error(), expectedErrType) || !strings.Contains(err.Error(), expectedErrDetail) {
		t.Errorf("error message does not match, expected type: %s, expected detail: %s, got: %s", expectedErrType, expectedErrDetail, err.Error())
	}
}
*/
func TestOverlappingRSs(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-overlapping-rss", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)

	// Create 2 RS with identical selectors
	labelMap := map[string]string{"foo": "bar"}
	for i := 0; i < 2; i++ {
		// One RS has 1 replica, and another has 2 replicas
		rs := newRS(fmt.Sprintf("rs-%d", i+1), ns.Name, i+1)
		rs.Spec.Selector.MatchLabels = labelMap
		rs.Spec.Template.Labels = labelMap
		createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{}, ns.Name)
		waitRSStable(t, c, rs, ns.Name)
	}

	// Expect 3 total Pods to be created
	podSelector := labels.Set{"foo": "bar"}.AsSelector()
	options := metav1.ListOptions{LabelSelector: podSelector.String()}
	pods, err := c.Core().Pods(ns.Name).List(options)
	if err != nil {
		t.Fatalf("Failed obtaining a list of pods that match the pod labels %v: %v", labelMap, err)
	}
	if pods == nil {
		t.Fatalf("Obtained a nil list of pods")
	}
	if len(pods.Items) != 3 {
		t.Errorf("len(pods) = %v, want %v", len(pods.Items), 3)
	}

	// Expect both RS have status.replicas = spec.replicas
	for i := 0; i < 2; i++ {
		replicaset, err := c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(fmt.Sprintf("rs-%d", i+1), metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get replicaset rs-%d: %v", i+1, err)
		}
		if replicaset.Status.Replicas != *replicaset.Spec.Replicas {
			t.Fatalf(".status.replicas %d is not equal to .spec.replicas %d", replicaset.Status.Replicas, *replicaset.Spec.Replicas)
		}
	}
}

func TestPodOrphaningAndAdoptionWhenLabelsChange(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-replicaset-basics", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	labelMap := map[string]string{"foo": "bar"}
	rs := newRS("rs", ns.Name, 1)
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	rss, _ := createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	// Orphaning: RS should remove OwnerReference from a pod when the pod's labels change to not match its labels
	// Although we can access the pod directly, we should still access it via selector to ensure the selector is working
	podClient := c.Core().Pods(ns.Name)
	podSelector := labels.Set{"foo": "bar"}.AsSelector()
	options := metav1.ListOptions{LabelSelector: podSelector.String()}
	pods, err := podClient.List(options)
	if err != nil {
		t.Fatalf("Failed obtaining a list of pods that match the pod labels %v: %v", labelMap, err)
	}
	if pods == nil {
		t.Fatalf("Obtained a nil list of pods")
	}
	if len(pods.Items) != 1 {
		t.Fatalf("len(pods) = %v, want %v", len(pods.Items), 1)
	}

	pod := &pods.Items[0]
	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		newPod, err := podClient.Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// Firstly, test ControllerRef for the pod is not nil
		if metav1.GetControllerOf(newPod) != nil {
			pod = newPod
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}

	newLabelMap := map[string]string{"new-foo": "new-bar"}
	if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		pod, err := podClient.Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		pod.Labels = newLabelMap
		pod, err = podClient.Update(pod)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		newPod, err := podClient.Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// Then, test ControllerRef for the pod is nil
		if metav1.GetControllerOf(newPod) == nil {
			pod = newPod
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}

	// Adoption: RS should add ControllerRef to a pod when the pod's labels change to match its labels
	if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		pod, err := podClient.Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// Revert to original labels so that the RS can adopt the pod again
		pod.Labels = labelMap
		pod, err = podClient.Update(pod)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err = wait.PollImmediate(interval, timeout, func() (bool, error) {
		newPod, err := podClient.Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if metav1.GetControllerOf(newPod) != nil {
			pod = newPod
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}

	rs = rss[0]
	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef.UID != rs.UID {
		t.Fatalf("RS owner of the pod %s has a different UID: Expected %v, got %v", pod.Name, rs.UID, controllerRef.UID)
	}
}

/*
// Verify ControllerRef of a RS pod that has incorrect attributes is automatically patched by the RS
func testPodControllerRefPatch(t *testing.T, c clientset.Interface, pod *v1.Pod, ownerReference *metav1.OwnerReference, rs *v1beta1.ReplicaSet, ns string) {
	podClient := c.Core().Pods(ns)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		pod, err := podClient.Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod %s: %v", pod.Name, err)
		}
		pod.OwnerReferences = []metav1.OwnerReference{*ownerReference}
		pod, err = c.Core().Pods(ns).Update(pod)
		if err != nil {
			return fmt.Errorf("failed to update pod %s: %v", pod.Name, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pod, err := podClient.Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if metav1.GetControllerOf(pod) != nil {
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}

	rs, err := c.ExtensionsV1beta1().ReplicaSets(ns).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef.UID != rs.UID {
		t.Fatalf("RS owner of the pod %s has a different UID: Expected %v, got %v", pod.Name, rs.UID, controllerRef.UID)
	}
	ownerReferenceNum := len(pod.GetOwnerReferences())
	if ownerReferenceNum != 1 {
		t.Fatalf("Unexpected number of owner references for pod %s: Expected 1, got %d", pod.Name, ownerReferenceNum)
	}
}

func TestGeneralPodAdoption(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-replicaset-basics", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	labelMap := map[string]string{"foo": "bar"}
	rs := newRS("rs", ns.Name, 1)
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	podSelector := labels.Set{"foo": "bar"}.AsSelector()
	options := metav1.ListOptions{LabelSelector: podSelector.String()}
	pods, err := c.Core().Pods(ns.Name).List(options)
	if err != nil {
		t.Fatalf("Failed obtaining a list of pods that match the pod labels %v: %v", labelMap, err)
	}
	if pods == nil {
		t.Fatalf("Obtained a nil list of pods")
	}
	if len(pods.Items) != 1 {
		t.Fatalf("len(pods) = %v, want %v", len(pods.Items), 1)
	}

	pod := &pods.Items[0]
	var falseVar = false

	// When the only OwnerReference of the pod points to another type of API object such as statefulset
	// with Controller=false, the RS should add a second OwnerReference (ControllerRef) pointing to itself
	// with Controller=true
	ownerReference := metav1.OwnerReference{UID: uuid.NewUUID(), APIVersion: "apps/v1beta1", Kind: "StatefulSet", Name: rs.Name, Controller: &falseVar}
	testPodControllerRefPatch(t, c, pod, ownerReference, rs, ns.Name)

	// When the only OwnerReference of the pod points to the RS, but Controller=false
	ownerReference = metav1.OwnerReference{UID: uuid.NewUUID(), APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Name: rs.Name, Controller: &falseVar}
	testPodControllerRefPatch(t, c, pod, ownerReference, rs, ns.Name)
}

func markAllPodsRunningAndReady(t *testing.T, clientSet clientset.Interface, pods *v1.PodList, ns string, replicas int32) {
	var readyPods int32
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		readyPods = 0
		for i := range pods.Items {
			pod := &pods.Items[i]
			if podutil.IsPodReady(pod) {
				readyPods++
				continue
			}
			pod.Status.Phase = v1.PodRunning
			condition := &v1.PodCondition {
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			}
			if !podutil.UpdatePodCondition(&pod.Status, condition) {
				t.Logf("failed to update condition for pod %s, will retry later", pod.Name)
				continue
			}
			_, err := clientSet.Core().Pods(ns).UpdateStatus(pod)
			if err != nil {
				return false, nil
			}
			readyPods++
		}
		if readyPods >= replicas {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("failed to mark all ReplicaSet pods to ready: %v", err)
	}
}

func waitAllPodsReady(t *testing.T, clientSet clientset.Interface, rs *v1beta1.ReplicaSet, ns string) {
	rsClient := clientSet.Extensions().ReplicaSets(ns)
	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if rs.Status.ReadyReplicas != *rs.Spec.Replicas {
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReadyAndAvailableReplicas(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-ready-and-available-replicas", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)

	labelMap := map[string]string{"foo": "bar"}
	rs := newRS("rs", ns.Name, 3)
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	// Make .spec.minReadySeconds long enough so that we can test the replicas are unavailable
	rs.Spec.MinReadySeconds = 3600
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	podSelector := labels.Set{"foo": "bar"}.AsSelector()
	options := metav1.ListOptions{LabelSelector: podSelector.String()}
	pods, err := c.Core().Pods(ns.Name).List(options)
	if err != nil {
		t.Fatalf("Failed obtaining a list of pods that match the pod labels %v: %v", labelMap, err)
	}
	if pods == nil {
		t.Fatalf("Obtained a nil list of pods")
	}
	if len(pods.Items) != 3 {
		t.Fatalf("len(pods) = %v, want %v", len(pods.Items), 3)
	}

	markAllPodsRunningAndReady(t, c, pods, ns.Name, *rs.Spec.Replicas)
	waitAllPodsReady(t, c, rs, ns.Name)

	if rs.Status.AvailableReplicas != 0 {
		t.Fatalf("Unexpected .Status.AvailableReplicas: Expected 0, saw %d\n", rs.Status.AvailableReplicas)
	}

	rsClient := c.Extensions().ReplicaSets(ns.Name)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get rs %s: %v", rs.Name, err)
		}
		// Make .spec.minReadySeconds = 0 so that we can test the replicas are available
		rs.Spec.MinReadySeconds = 0
		rs, err = rsClient.Update(rs)
		if err != nil {
			return fmt.Errorf("failed to update rs %s: %v", rs.Name, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if rs.Status.AvailableReplicas != *rs.Spec.Replicas {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Update .Spec.Replicas to x and verify .Status.Replicas is changed accordingly
func testStatusReplicas(t *testing.T, c clientset.Interface, rs *v1beta1.ReplicaSet, ns string, x int32) {
	rsClient := c.Extensions().ReplicaSets(ns)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get rs %s: %v", rs.Name, err)
		}
		*(rs.Spec.Replicas) = x
		rs, err = c.Extensions().ReplicaSets(ns).Update(rs)
		if err != nil {
			return fmt.Errorf("failed to update rs %s: %v", rs.Name, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	waitRSStable(t, c, rs, ns)
}

func TestReplicaSetBasics(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-replicaset-basics", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	labelMap := map[string]string{"foo": "bar"}
	rs := newRS("rs", ns.Name, 2)
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	testStatusReplicas(t, c, rs, ns.Name, 3)
	testStatusReplicas(t, c, rs, ns.Name, 0)
	testStatusReplicas(t, c, rs, ns.Name, 2)

	// Update 2 pods that RS created such that one is failed and one is being deleted
	// Ensure both are being created again in the end
	podSelector := labels.Set{"foo": "bar"}.AsSelector()
	options := metav1.ListOptions{LabelSelector: podSelector.String()}
	pods, err := c.Core().Pods(ns.Name).List(options)
	if err != nil {
		t.Fatalf("Failed obtaining a list of pods that match the pod labels %v: %v", labelMap, err)
	}
	if pods == nil {
		t.Fatalf("Obtained a nil list of pods")
	}
	if len(pods.Items) != 2 {
		t.Fatalf("len(pods) = %v, want %v", len(pods.Items), 2)
	}

	failedPod := &pods.Items[0]
	deletingPod := &pods.Items[1]

	podClient := c.Core().Pods(ns.Name)
	if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		failedPod, err := podClient.Get(failedPod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		failedPod.Status.Phase = v1.PodFailed
		failedPod, err = c.Core().Pods(ns.Name).Update(failedPod)
		if err != nil {
			return fmt.Errorf("failed to update pod %s: %v", failedPod.Name, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		deletingPod, err := podClient.Get(deletingPod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		//deletingPod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		err = c.Core().Pods(ns.Name).Delete(deletingPod.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete pod %s: %v", deletingPod.Name, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rsClient := c.Extensions().ReplicaSets(ns.Name)
	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if rs.Status.Replicas != 2 {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	// Update .Generation and verify .Status.ObservedGeneration is changed
	// Not need to use RetryOnConflict() to update .Generation (not a .Spec attibute)
	rs.Generation = 100
	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if rs.Status.ObservedGeneration >= rs.Generation {
			return true, nil
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func testScalingUsingScaleEndpoint(t *testing.T, c clientset.Interface, rs *v1beta1.ReplicaSet, ns string, x int32) {
	kind := "ReplicaSet"
	scaleClient := c.ExtensionsV1beta1().Scales(ns)
	rsClient := c.Extensions().ReplicaSets(ns)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		scale, err := scaleClient.Get(kind, rs.Name)
		if err != nil {
			t.Fatalf("failed to get scale endpoint for RS %s: %v", rs.Name, err)
		}
		scale.Spec.Replicas = x
		scale, err = scaleClient.Update(kind, scale)
		if err != nil {
			t.Fatalf("failed to update scale endpoint for RS %s: %v", rs.Name, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		rs, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if rs.Status.Replicas != x {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRSScaleEndpoint(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-rs-scale-endpoint", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	labelMap := map[string]string{"foo": "bar"}
	rs := newRS("rs", ns.Name, 1)
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	rs, err := c.Extensions().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 1 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 1, saw %d\n", rs.Status.Replicas)
	}

	// Use scale endpoint to scale up .Spec.Replicas to 3
	testScalingUsingScaleEndpoint(t, c, rs, ns.Name, 3)

	// Use the scale endpoint to scale down .Spec.Replicas to 0
	testScalingUsingScaleEndpoint(t, c, rs, ns.Name, 0)
}
*/
