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

package v1

import (
	//"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api"
)

func TestPodToSelectableFields(t *testing.T) {
	expectedStr := "metadata.name=foo,metadata.namespace=bar,metadata.uid=baz,spec.nodeName=node1,spec.restartPolicy=Always,spec.serviceAccountName=svc1,status.hostIP=1.2.3.4,status.phase=ph1,status.podIP=4.5.6.7"
	pod := api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
			UID:       types.UID("baz"),
		},
		Spec: api.PodSpec{
			NodeName:           "node1",
			RestartPolicy:      api.RestartPolicyAlways,
			ServiceAccountName: "svc1",
		},
		Status: api.PodStatus{
			HostIP: "1.2.3.4",
			PodIP:  "4.5.6.7",
			Phase:  api.PodPhase("ph1"),
		},
	}

	podFieldsSet := PodToSelectableFields(&pod)
	if podFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for Pod", podFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "metadata.namespace",
			ExpectedValue: "bar",
		},
		{
			ExpectedKey:   "metadata.uid",
			ExpectedValue: "baz",
		},
		{
			ExpectedKey:   "spec.nodeName",
			ExpectedValue: "node1",
		},
		{
			ExpectedKey:   "spec.restartPolicy",
			ExpectedValue: "Always",
		},
		{
			ExpectedKey:   "spec.serviceAccountName",
			ExpectedValue: "svc1",
		},
		{
			ExpectedKey:   "status.hostIP",
			ExpectedValue: "1.2.3.4",
		},
		{
			ExpectedKey:   "status.phase",
			ExpectedValue: "ph1",
		},
		{
			ExpectedKey:   "status.podIP",
			ExpectedValue: "4.5.6.7",
		},
	}

	for _, tc := range testcases {
		if !podFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing Pod fieldSelector %q", tc.ExpectedKey)
		}
		if podFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("Pod filedSelector %q has got unexpected value %q", tc.ExpectedKey, podFieldsSet.Get(tc.ExpectedKey))
		}
	}
}

func TestNodeToSelectableFields(t *testing.T) {
	expectedStr := "metadata.name=foo,spec.unschedulable=false"
	node := api.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: api.NodeSpec{
			Unschedulable: false,
		},
	}

	nodeFieldsSet := NodeToSelectableFields(&node)
	if nodeFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for Node", nodeFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "spec.unschedulable",
			ExpectedValue: "false",
		},
	}

	for _, tc := range testcases {
		if !nodeFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing Node fieldSelector %q", tc.ExpectedKey)
		}
		if nodeFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("Node filedSelector %q has got unexpected value %q", tc.ExpectedKey, nodeFieldsSet.Get(tc.ExpectedKey))
		}
	}
}

func TestControllerToSelectableFields(t *testing.T) {
	expectedStr := "metadata.name=foo,metadata.namespace=bar,status.replicas=1"
	rc := api.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Status: api.ReplicationControllerStatus{
			Replicas: 1,
		},
	}

	rcFieldsSet := ControllerToSelectableFields(&rc)
	if rcFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for ReplicationController", rcFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "metadata.namespace",
			ExpectedValue: "bar",
		},
		{
			ExpectedKey:   "status.replicas",
			ExpectedValue: "1",
		},
	}

	for _, tc := range testcases {
		if !rcFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing ReplicationController fieldSelector %q", tc.ExpectedKey)
		}
		if rcFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("ReplicationController filedSelector %q has got unexpected value %q", tc.ExpectedKey, rcFieldsSet.Get(tc.ExpectedKey))
		}
	}
}

func TestPersistentVolumeToSelectableFields(t *testing.T) {
	expectedStr := "metadata.name=foo,name=foo"
	pv := api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	pvFieldsSet := PersistentVolumeToSelectableFields(&pv)
	if pvFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for PersistentVolume", pvFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "name",
			ExpectedValue: "foo",
		},
	}

	for _, tc := range testcases {
		if !pvFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing PersistentVolume fieldSelector %q", tc.ExpectedKey)
		}
		if pvFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("PersistentVolume filedSelector %q has got unexpected value %q", tc.ExpectedKey, pvFieldsSet.Get(tc.ExpectedKey))
		}
	}
}

func TestEventToSelectableFields(t *testing.T) {
	expectedStr := "involvedObject.apiVersion=foo,involvedObject.fieldPath=bar,involvedObject.kind=baz,involvedObject.name=name1,involvedObject.namespace=ns1,involvedObject.resourceVersion=1,involvedObject.uid=uid1,metadata.name=foo,metadata.namespace=ns1,reason=reason1,type=type1"
	event := api.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "ns1",
		},
		InvolvedObject: api.ObjectReference{
			APIVersion:      "foo",
			FieldPath:       "bar",
			Kind:            "baz",
			Name:            "name1",
			Namespace:       "ns1",
			ResourceVersion: "1",
			UID:             types.UID("uid1"),
		},
		Reason: "reason1",
		Type:   "type1",
	}

	eventFieldsSet := EventToSelectableFields(&event)
	if eventFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for Event", eventFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "involvedObject.apiVersion",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "involvedObject.fieldPath",
			ExpectedValue: "bar",
		},
		{
			ExpectedKey:   "involvedObject.kind",
			ExpectedValue: "baz",
		},
		{
			ExpectedKey:   "involvedObject.name",
			ExpectedValue: "name1",
		},
		{
			ExpectedKey:   "involvedObject.namespace",
			ExpectedValue: "ns1",
		},
		{
			ExpectedKey:   "involvedObject.resourceVersion",
			ExpectedValue: "1",
		},
		{
			ExpectedKey:   "involvedObject.uid",
			ExpectedValue: "uid1",
		},
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "metadata.namespace",
			ExpectedValue: "ns1",
		},
		{
			ExpectedKey:   "reason",
			ExpectedValue: "reason1",
		},
		{
			ExpectedKey:   "type",
			ExpectedValue: "type1",
		},
	}

	for _, tc := range testcases {
		if !eventFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing Event fieldSelector %q", tc.ExpectedKey)
		}
		if eventFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("Event filedSelector %q has got unexpected value %q", tc.ExpectedKey, eventFieldsSet.Get(tc.ExpectedKey))
		}
	}
}

func TestNamespaceToSelectableFields(t *testing.T) {
	expectedStr := "metadata.name=foo,name=foo,status.phase=ph1"
	ns := api.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Status: api.NamespaceStatus{
			Phase: api.NamespacePhase("ph1"),
		},
	}

	nsFieldsSet := NamespaceToSelectableFields(&ns)
	if nsFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for Namespace", nsFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "status.phase",
			ExpectedValue: "ph1",
		},
	}

	for _, tc := range testcases {
		if !nsFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing Namespace fieldSelector %q", tc.ExpectedKey)
		}
		if nsFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("Namespace filedSelector %q has got unexpected value %q", tc.ExpectedKey, nsFieldsSet.Get(tc.ExpectedKey))
		}
	}
}

func TestSecretToSelectableFields(t *testing.T) {
	expectedStr := "metadata.name=foo,type=type1"
	secret := api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Type: api.SecretType("type1"),
	}

	secretFieldsSet := SecretToSelectableFields(&secret)
	if secretFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for Secret", secretFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "type",
			ExpectedValue: "type1",
		},
	}

	for _, tc := range testcases {
		if !secretFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing Secret fieldSelector %q", tc.ExpectedKey)
		}
		if secretFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("Secret filedSelector %q has got unexpected value %q", tc.ExpectedKey, secretFieldsSet.Get(tc.ExpectedKey))
		}
	}
}

func TestPersistentVolumeClaimToSelectableFields(t *testing.T) {
	expectedStr := "metadata.name=foo,name=foo"
	pvc := api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	pvcFieldsSet := PersistentVolumeClaimToSelectableFields(&pvc)
	if pvcFieldsSet.String() != expectedStr {
		t.Errorf("unexpected fieldSelector %q for PersistentVolumeClaim", pvcFieldsSet.String())
	}

	testcases := []struct {
		ExpectedKey   string
		ExpectedValue string
	}{
		{
			ExpectedKey:   "metadata.name",
			ExpectedValue: "foo",
		},
		{
			ExpectedKey:   "name",
			ExpectedValue: "foo",
		},
	}

	for _, tc := range testcases {
		if !pvcFieldsSet.Has(tc.ExpectedKey) {
			t.Errorf("missing PersistentVolumeClaim fieldSelector %q", tc.ExpectedKey)
		}
		if pvcFieldsSet.Get(tc.ExpectedKey) != tc.ExpectedValue {
			t.Errorf("PersistentVolumeClaim filedSelector %q has got unexpected value %q", tc.ExpectedKey, pvcFieldsSet.Get(tc.ExpectedKey))
		}
	}
}
