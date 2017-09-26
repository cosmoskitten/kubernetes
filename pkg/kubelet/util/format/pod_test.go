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

package format

import (
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func fakeCreatePod(name, namespace string, uid types.UID) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       uid,
		},
	}
}

func fakeCreatePodWithDeletionTimestamp(name, namespace string, uid types.UID, deletionTimestamp *metav1.Time) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               uid,
			DeletionTimestamp: deletionTimestamp,
		},
	}
}

func TestPodAndPodDesc(t *testing.T) {
	testCases := []struct {
		caseName      string
		podName       string
		podNamesapce  string
		podUID        types.UID
		expectedValue string
	}{
		{"field_empty_but_not_should_be_exist_case", "", "", "", "_()"},
		{"field_normal_all_case", "test-pod", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75", "test-pod_default(551f5a43-9f2f-11e7-a589-fa163e148d75)"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			// Create test Ojbect
			fakePod := fakeCreatePod(testCase.podName, testCase.podNamesapce, testCase.podUID)

			realPod := Pod(fakePod)
			if realPod != testCase.expectedValue {
				t.Fatalf("Failed to format pod: Get %s, but want %s", realPod, testCase.expectedValue)
			}

			realPodDesc := PodDesc(fakePod.Name, fakePod.Namespace, fakePod.UID)
			if realPodDesc != testCase.expectedValue {
				t.Fatalf("Failed to format pod(PodDesc): Get %s, but want %s", realPodDesc, testCase.expectedValue)
			}

		})
	}
}

func TestPodWithDeletionTimestamp(t *testing.T) {
	normalDeletionTime := metav1.Date(2017, time.September, 26, 14, 37, 50, 00, time.UTC)

	testCases := []struct {
		caseName               string
		isdeletionTimestampNil bool
		deletionTimestamp      metav1.Time
		expectedValue          string
	}{
		{"field_timestamp_is_nil_case", true, normalDeletionTime, "test-pod_default(551f5a43-9f2f-11e7-a589-fa163e148d75)"},
		{"field_normal_case", false, normalDeletionTime, "test-pod_default(551f5a43-9f2f-11e7-a589-fa163e148d75):DeletionTimestamp=2017-09-26T14:37:50Z"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			fakePod := fakeCreatePodWithDeletionTimestamp("test-pod", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75", &testCase.deletionTimestamp)

			if testCase.isdeletionTimestampNil {
				fakePod.SetDeletionTimestamp(nil)
			}

			realPodWithDeletionTimestamp := PodWithDeletionTimestamp(fakePod)
			if realPodWithDeletionTimestamp != testCase.expectedValue {
				t.Errorf("Failed to format pod(PodWithDeletionTimestamp): Get %s, but want %s", realPodWithDeletionTimestamp, testCase.expectedValue)
			}

		})
	}
}

func TestPods(t *testing.T) {
	pod1 := fakeCreatePod("pod1", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75")
	pod2 := fakeCreatePod("pod2", metav1.NamespaceDefault, "e84a99bf-d1f9-43c2-9fa5-044ac85f794b")

	testCases := []struct {
		caseName      string
		pods          []*v1.Pod
		expectedValue string
	}{
		{"nil_case", nil, ""},
		{"empty_case", []*v1.Pod{}, ""},
		{"only_one_case", []*v1.Pod{pod1}, "pod1_default(551f5a43-9f2f-11e7-a589-fa163e148d75)"},
		{"more_than_one_case", []*v1.Pod{pod1, pod2}, "pod1_default(551f5a43-9f2f-11e7-a589-fa163e148d75), pod2_default(e84a99bf-d1f9-43c2-9fa5-044ac85f794b)"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			realPods := Pods(testCase.pods)
			if realPods != testCase.expectedValue {
				t.Errorf("Failed to format pods: Get %s, but want %s", realPods, testCase.expectedValue)
			}

		})
	}
}

func TestPodsWithDeletiontimestamps(t *testing.T) {
	normalDeletionTime := metav1.Date(2017, time.September, 26, 14, 37, 50, 00, time.UTC)
	pod1 := fakeCreatePodWithDeletionTimestamp("pod1", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75", &normalDeletionTime)
	pod2 := fakeCreatePodWithDeletionTimestamp("pod2", metav1.NamespaceDefault, "e84a99bf-d1f9-43c2-9fa5-044ac85f794b", &normalDeletionTime)

	testCases := []struct {
		caseName      string
		pods          []*v1.Pod
		expectedValue string
	}{
		{"nil_case", nil, ""},
		{"empty_case", []*v1.Pod{}, ""},
		{"one_case", []*v1.Pod{pod1}, "pod1_default(551f5a43-9f2f-11e7-a589-fa163e148d75):DeletionTimestamp=2017-09-26T14:37:50Z"},
		{"more_than_one_case", []*v1.Pod{pod1, pod2}, "pod1_default(551f5a43-9f2f-11e7-a589-fa163e148d75):DeletionTimestamp=2017-09-26T14:37:50Z, pod2_default(e84a99bf-d1f9-43c2-9fa5-044ac85f794b):DeletionTimestamp=2017-09-26T14:37:50Z"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			realPods := PodsWithDeletiontimestamps(testCase.pods)
			if realPods != testCase.expectedValue {
				t.Errorf("Failed to format pods(PodsWithDeletiontimestamps): Get %s, but want %s", realPods, testCase.expectedValue)
			}

		})
	}
}
