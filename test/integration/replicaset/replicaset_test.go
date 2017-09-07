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
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/controller/replicaset"
	"k8s.io/kubernetes/test/integration/framework"
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
			UID:       uuid.NewUUID(),
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

func createRSsPods(t *testing.T, clientSet clientset.Interface, rss []*v1beta1.ReplicaSet, pods []*v1.Pod, ns string) {
	rsClient := clientSet.Extensions().ReplicaSets(ns)
	podClient := clientSet.Core().Pods(ns)
	for _, rs := range rss {
		if _, err := rsClient.Create(rs); err != nil {
			t.Fatalf("Failed to create replica set %s: %v", rs.Name, err)
		}
	}
	for _, pod := range pods {
		if _, err := podClient.Create(pod); err != nil {
			t.Fatalf("Failed to create pod %s: %v", pod.Name, err)
		}
	}
}

func waitRSStable(t *testing.T, clientSet clientset.Interface, rs *v1beta1.ReplicaSet, ns string) {
	rsClient := clientSet.Extensions().ReplicaSets(ns)
	if err := wait.Poll(10*time.Second, 60*time.Second, func() (bool, error) {
		updatedRS, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if updatedRS.Status.Replicas != *rs.Spec.Replicas {
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		t.Fatal(err)
	}
}

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

// create a pod with the given phase for the given rs (same selectors and namespace)
func newPod(name string, rs *v1beta1.ReplicaSet, status v1.PodPhase, lastTransitionTime *metav1.Time, properlyOwned bool) *v1.Pod {
	var conditions []v1.PodCondition
	if status == v1.PodRunning {
		condition := v1.PodCondition{Type: v1.PodReady, Status: v1.ConditionTrue}
		if lastTransitionTime != nil {
			condition.LastTransitionTime = *lastTransitionTime
		}
		conditions = append(conditions, condition)
	}
	var controllerReference metav1.OwnerReference
	if properlyOwned {
		var trueVar = true
		controllerReference = metav1.OwnerReference{UID: rs.UID, APIVersion: "v1beta1", Kind: "ReplicaSet", Name: rs.Name, Controller: &trueVar}
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       rs.Namespace,
			Labels:          rs.Spec.Selector.MatchLabels,
			OwnerReferences: []metav1.OwnerReference{controllerReference},
		},
		Status: v1.PodStatus{Phase: status, Conditions: conditions},
	}
}

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
		rs := newRS(fmt.Sprintf("rs-%d", i+1), ns.Name, i+1)
		rs.Spec.Selector.MatchLabels = labelMap
		rs.Spec.Template.Labels = labelMap

		// One RS has 1 replica, and another has 2 replicas
		podCount := i + 1

		var podList []*v1.Pod
		for j := 0; j < podCount; j++ {
			pod := newMatchingPod(fmt.Sprintf("pod-%d-%d", i+1, j+1), ns.Name)
			pod.Labels = labelMap
			podList = append(podList, pod)
		}
		createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, podList, ns.Name)
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
	pod := newMatchingPod("pod", ns.Name)
	pod.Labels = labelMap
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{pod}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	rs, err := c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 1 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 1, saw %d", rs.Status.Replicas)
	}

	// Orphaning: RS should remove OwnerReference from a pod when the pod's labels change to not match its labels
	// Although we can access the pod directly, we should still access it via selector to ensure the selector is working
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

	pod = &pods.Items[0]
	newLabelMap := map[string]string{"new-foo": "new-bar"}
	pod.Labels = newLabelMap
	pod, err = c.Core().Pods(ns.Name).Update(pod)
	if err != nil {
		t.Fatalf("failed to update pod %s: %v", pod.Name, err)
	}
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	// The RS should still have 1 replica (the RS should create another new replica)
	if rs.Status.Replicas != 1 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 1, saw %d", rs.Status.Replicas)
	}

	pod, err = c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	if !reflect.DeepEqual(pod.Labels, newLabelMap) {
		t.Fatalf("failed to update labels for pod %s: Expected %v, saw %v", pod.Name, newLabelMap, pod.Labels)
	}
	if len(pod.OwnerReferences) != 0 {
		t.Fatalf("failed to remove controllerReference for pod %s after changing its labels", pod.Name)
	}

	// Adoption: RS should add OwnerReference to a pod when the pod's labels change to match its labels
	podSelector = labels.Set{"new-foo": "new-bar"}.AsSelector()
	options = metav1.ListOptions{LabelSelector: podSelector.String()}
	pods, err = c.Core().Pods(ns.Name).List(options)
	if err != nil {
		t.Fatalf("Failed obtaining a list of pods that match the pod labels %v: %v", newLabelMap, err)
	}
	if pods == nil {
		t.Fatalf("Obtained a nil list of pods")
	}
	if len(pods.Items) != 1 {
		t.Fatalf("len(pods) = %v, want %v", len(pods.Items), 1)
	}

	// Revert to original labels so that the RS can adopt the pod again
	pod = &pods.Items[0]
	pod.Labels = labelMap
	pod, err = c.Core().Pods(ns.Name).Update(pod)
	if err != nil {
		t.Fatalf("failed to update pod %s: %v", pod.Name, err)
	}

	// Have to GET the RS again as the pod (and thus the RS) has been modified
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	// The RS should still have 1 replica (original replica should replace the newly created replica)
	if rs.Status.Replicas != 1 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 1, saw %d", rs.Status.Replicas)
	}

	pod, err = c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	if !reflect.DeepEqual(pod.Labels, labelMap) {
		t.Fatalf("failed to update labels for pod %s: Expected %v, saw %v", pod.Name, labelMap, pod.Labels)
	}
	// Perhaps a better way is to compare the OwnerReferences using reflect.DeepEqual (this comment will be removed afterwards)
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("failed to add controllerReference for pod %s after changing its labels", pod.Name)
	}
}

func TestPodOrphaningAndAdoptionWhenSelectorChanges(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-replicaset-basics", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	labelMap := map[string]string{"foo": "bar"}
	rs := newRS("rs", ns.Name, 1)
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	pod := newMatchingPod("pod", ns.Name)
	pod.Labels = labelMap
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{pod}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	rs, err := c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 1 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 1, saw %d", rs.Status.Replicas)
	}

	// Orphaning: RS should remove OwnerReference from a pod when the pod's selector changes to not match its selector
	newLabelMap := map[string]string{"new-foo": "new-bar"}
	rs.Spec.Selector.MatchLabels = newLabelMap
	rs.Spec.Template.Labels = newLabelMap
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if !reflect.DeepEqual(rs.Spec.Selector.MatchLabels, newLabelMap) {
		t.Fatalf("failed to update selector for RS %s: Expected %v, saw %v", rs.Name, newLabelMap, rs.Spec.Selector.MatchLabels)
	}
	// The RS should still have 1 replica (the RS should create another new replica)
	if rs.Status.Replicas != 1 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 1, saw %d", rs.Status.Replicas)
	}

	pod, err = c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	if len(pod.OwnerReferences) != 0 {
		t.Fatalf("failed to remove controllerReference for pod %s after changing the selector of owning RS %s", pod.Name, rs.Name)
	}

	// Adoption: RS should add OwnerReference to a pod when the pod's selector changes to match its selector
	// Revert to original selector so that the RS can adopt the pod again
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if !reflect.DeepEqual(rs.Spec.Selector.MatchLabels, labelMap) {
		t.Fatalf("failed to update selector for RS %s: Expected %v, saw %v", rs.Name, labelMap, rs.Spec.Selector.MatchLabels)
	}
	// The RS should still have 1 replica (original replica should replace the newly created replica)
	if rs.Status.Replicas != 1 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 1, saw %d", rs.Status.Replicas)
	}

	pod, err = c.Core().Pods(ns.Name).Update(pod)
	if err != nil {
		t.Fatalf("failed to update pod %s: %v", pod.Name, err)
	}
	pod, err = c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	// Verify the RS does not adopt the pod automatically after reverting back the selector
	if len(pod.OwnerReferences) != 0 {
		t.Fatalf("pod %s was not expected to be adopted by RS %s", pod.Name, rs.Name)
	}

	// We have to set OwnerReferences of the pod manually for it to be adopted by the RS
	var trueVar = true
	controllerReference := metav1.OwnerReference{UID: rs.UID, APIVersion: "v1beta1", Kind: "ReplicaSet", Name: rs.Name, Controller: &trueVar}
	pod.OwnerReferences = []metav1.OwnerReference{controllerReference}

	// Have to GET the pod again as the pod has been modified
	pod, err = c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	pod, err = c.Core().Pods(ns.Name).Update(pod)
	if err != nil {
		t.Fatalf("failed to update pod %s: %v", pod.Name, err)
	}
	pod, err = c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	// Perhaps a better way is to compare the OwnerReferences using reflect.DeepEqual (this comment will be removed afterwards)
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("failed to add controllerReference for pod %s after explicitly setting its OwnerReferences to RS %s", pod.Name, rs.Name)
	}
}

func TestPatchPod(t *testing.T) {
	s, closeFn, rm, informers, c := rmSetup(t)
	defer closeFn()
	ns := framework.CreateTestingNamespace("test-replicaset-basics", s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)
	labelMap := map[string]string{"foo": "bar"}
	rs := newRS("rs", ns.Name, 1)
	rs.Spec.Selector.MatchLabels = labelMap
	rs.Spec.Template.Labels = labelMap
	pod := newMatchingPod("pod", ns.Name)
	pod.Labels = labelMap
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{pod}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	var trueVar = true
	var falseVar = false
	expectedControllerReference := metav1.OwnerReference{UID: rs.UID, APIVersion: "extensions/v1beta1", Kind: "ReplicaSet", Name: rs.Name, Controller: &trueVar}
	expectedOwnerReferences := []metav1.OwnerReference{expectedControllerReference}

	// When the only OwnerReference of the pod points to another type of API object such as statefulset
	// with Controller=false, the RS should add a second OwnerReference pointing to itself
	// with Controller=true
	controllerReference := metav1.OwnerReference{UID: uuid.NewUUID(), APIVersion: "apps/v1beta1", Kind: "StatefulSet", Name: rs.Name, Controller: &falseVar}
	pod.OwnerReferences = []metav1.OwnerReference{controllerReference}
	pod, err := c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	pod, err = c.Core().Pods(ns.Name).Update(pod)
	if err != nil {
		t.Fatalf("failed to update pod %s: %v", pod.Name, err)
	}

	// We have to test each field individually as address fields
	// such as Controller and BlockOwnerDeletion will be different
	if pod.OwnerReferences[0].APIVersion != expectedControllerReference.APIVersion ||
		pod.OwnerReferences[0].Kind != expectedControllerReference.Kind ||
		pod.OwnerReferences[0].Name != expectedControllerReference.Name ||
		pod.OwnerReferences[0].Controller == &falseVar {
		t.Fatalf("Unexpected ControllerReference for pod %s: Expected %v, saw %v", pod.Name, expectedOwnerReferences, pod.OwnerReferences)
	}

	// When the only OwnerReference of the pod points to the RS, but Controller=false
	controllerReference = expectedControllerReference
	controllerReference.Controller = &falseVar
	pod.OwnerReferences = []metav1.OwnerReference{controllerReference}
	pod, err = c.Core().Pods(ns.Name).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod.Name, err)
	}
	pod, err = c.Core().Pods(ns.Name).Update(pod)
	if err != nil {
		t.Fatalf("failed to update pod %s: %v", pod.Name, err)
	}
	if pod.OwnerReferences[0].APIVersion != expectedControllerReference.APIVersion ||
		pod.OwnerReferences[0].Kind != expectedControllerReference.Kind ||
		pod.OwnerReferences[0].Name != expectedControllerReference.Name ||
		pod.OwnerReferences[0].Controller == &falseVar {
		t.Fatalf("Unexpected ControllerReference for pod %s: Expected %v, saw %v", pod.Name, expectedOwnerReferences, pod.OwnerReferences)
	}

	// When the pod is extra
	pod2 := newMatchingPod("pod-2", ns.Name)
	pod2.Labels = labelMap
	createRSsPods(t, c, []*v1beta1.ReplicaSet{}, []*v1.Pod{pod2}, ns.Name)
	controllerReference = metav1.OwnerReference{UID: uuid.NewUUID(), APIVersion: "apps/v1beta1", Kind: "StatefulSet", Name: rs.Name, Controller: &falseVar}
	pod2.OwnerReferences = []metav1.OwnerReference{controllerReference}
	pod2, err = c.Core().Pods(ns.Name).Get(pod2.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get pod %s: %v", pod2.Name, err)
	}
	pod2, err = c.Core().Pods(ns.Name).Update(pod2)
	if err != nil {
		t.Fatalf("failed to update pod %s: %v", pod2.Name, err)
	}
	// Verify PATCH erases OwnerReferences of pod2 as the RS can only have 1 pod
	if pod2.OwnerReferences != nil {
		t.Fatalf("Unexpected ControllerReference for pod %s: Expected nil, saw %v", pod2.Name, pod.OwnerReferences)
	}

	// Should we test when the pod belongs to another RS? It seems irrelevant.
}

// addPodConditionReady sets given pod status to ready at given time
func addPodConditionReady(pod *v1.Pod, time metav1.Time) {
	pod.Status = v1.PodStatus{
		Phase: v1.PodRunning,
		Conditions: []v1.PodCondition{
			{
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				LastTransitionTime: time,
			},
		},
	}
}

func markAllPodsReady(t *testing.T, clientSet clientset.Interface, rs *v1beta1.ReplicaSet, ns string) {
	selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
	if err != nil {
		t.Fatalf("failed to parse ReplicaSet selector: %v", err)
	}
	var readyPods int32
	err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
		readyPods = 0
		pods, err := clientSet.Core().Pods(ns).List(metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			t.Logf("failed to list ReplicaSet pods, will retry later: %v", err)
			return false, nil
		}
		for i := range pods.Items {
			pod := pods.Items[i]
			if podutil.IsPodReady(&pod) {
				readyPods++
				continue
			}
			addPodConditionReady(&pod, metav1.Now())
			if _, err = clientSet.Core().Pods(ns).UpdateStatus(&pod); err != nil {
				t.Logf("failed to update ReplicaSet pod %s, will retry later: %v", pod.Name, err)
			} else {
				readyPods++
			}
		}
		if readyPods >= *rs.Spec.Replicas {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("failed to mark all ReplicaSet pods to ready: %v", err)
	}
	//fmt.Println("readyPods: %d, rs.spec.replicas: %d", readyPods, *rs.Spec.Replicas)
}

func waitAllPodsReady(t *testing.T, clientSet clientset.Interface, rs *v1beta1.ReplicaSet, ns string) {
	rsClient := clientSet.Extensions().ReplicaSets(ns)
	if err := wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		updatedRS, err := rsClient.Get(rs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if updatedRS.Status.ReadyReplicas != *rs.Spec.Replicas {
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
	// Make .spec.minReadySeconds long enough so that we can test the replicas are unavailable initially
	rs.Spec.MinReadySeconds = 3600

	// Create 3 pods
	var podList []*v1.Pod
	moment := metav1.Now()
	for i := 0; i < 3; i++ {
		// When creating the pod, manually set pod status as there is no kubelet in integration test env
		pod := newPod(fmt.Sprintf("pod-%d", i+1), rs, v1.PodRunning, &moment, true)
		pod.Labels = labelMap
		pod.Spec.Containers = []v1.Container{
			{
				Name:  "nginx",
				Image: "gcr.io/google_containers/nginx-slim:0.8",
			},
		}
		podList = append(podList, pod)
	}
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, podList, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	markAllPodsReady(t, c, rs, ns.Name)
	waitAllPodsReady(t, c, rs, ns.Name)
	waitRSStable(t, c, rs, ns.Name)

	rs, err := c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 3 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 3, saw %d\n", rs.Status.Replicas)
	}
	if rs.Status.ReadyReplicas != 3 {
		t.Fatalf("Unexpected .Status.ReadyReplicas: Expected 3, saw %d\n", rs.Status.ReadyReplicas)
	}
	//Verify all pods are not available manually
	podSelector := labels.Set{"foo": "bar"}.AsSelector()
	options := metav1.ListOptions{LabelSelector: podSelector.String()}

	// Make .spec.minReadySeconds = 0 so that we can test the replicas are available
	rs.Spec.MinReadySeconds = 0
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	// After updating the RS and waiting RS to be stable, we still have to GET the RS
	// for the RS to reflect newest .Status
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 3 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 3, saw %d\n", rs.Status.Replicas)
	}
	if rs.Status.ReadyReplicas != 3 {
		t.Fatalf("Unexpected .Status.ReadyReplicas: Expected 3, saw %d\n", rs.Status.ReadyReplicas)
	}
	// Verify all pods are available manually
	pods, err := c.Core().Pods(ns.Name).List(options)
	if err != nil {
		t.Fatalf("Failed obtaining a list of pods that match the pod labels %v: %v", labelMap, err)
	}
	if pods == nil {
		t.Fatalf("Obtained a nil list of pods")
	}
	for _, pod := range pods.Items {
		if podutil.IsPodAvailable(&pod, rs.Spec.MinReadySeconds, metav1.Now()) {
			t.Fatalf("Pod %s is not available after .spec.minReadySeconds of RS %s changed to %d", pod.Name, rs.Name, rs.Spec.MinReadySeconds)
		}
	}
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

	// Create 2 pods (one is failed, one is being deleted)
	// Ensure both are being created again in the end
	failedPod := newMatchingPod("failed-pod", ns.Name)
	failedPod.Labels = labelMap
	failedPod.Status.Phase = v1.PodFailed
	deletedPod := newMatchingPod("deleted-pod", ns.Name)
	deletedPod.Labels = labelMap
	deletedPod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	createRSsPods(t, c, []*v1beta1.ReplicaSet{rs}, []*v1.Pod{failedPod, deletedPod}, ns.Name)

	stopCh := make(chan struct{})
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	defer close(stopCh)
	waitRSStable(t, c, rs, ns.Name)

	rs, err := c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 2 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 2, saw %d\n", rs.Status.Replicas)
	}

	// Update .Spec.Replicas to 3 > 2
	*(rs.Spec.Replicas) = 3
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	// After updating the RS and waiting RS to be stable, we still have to GET the RS
	// for the RS to reflect newest .Status
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 3 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 3, saw %d\n", rs.Status.Replicas)
	}

	// Update .Spec.Replicas to 2 < 3
	*(rs.Spec.Replicas) = 2
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 2 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 2, saw %d\n", rs.Status.Replicas)
	}

	// Update .Spec.Replicas to 0
	*(rs.Spec.Replicas) = 0
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.Replicas != 0 {
		t.Fatalf("Unexpected .Status.Replicas: Expected 0, saw %d\n", rs.Status.Replicas)
	}

	// RS should reconcile conflicting attributes of .spec and .status
	*(rs.Spec.Replicas) = 1
	rs.Status.FullyLabeledReplicas = 10
	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Update(rs)
	if err != nil {
		t.Fatalf("failed to update replicaset %s: %v", rs.Name, err)
	}
	waitRSStable(t, c, rs, ns.Name)

	rs, err = c.ExtensionsV1beta1().ReplicaSets(ns.Name).Get(rs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get replicaset %s: %v", rs.Name, err)
	}
	if rs.Status.FullyLabeledReplicas != 1 {
		t.Fatalf("Unexpected .Status.FullyLabeledReplicas: Expected 1, saw %d\n", rs.Status.FullyLabeledReplicas)
	}
}
