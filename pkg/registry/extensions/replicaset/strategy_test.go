/*
Copyright 2016 The Kubernetes Authors.

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
	"net/http"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

func TestReplicaSetStrategy(t *testing.T) {
	ctx := genericapirequest.NewDefaultContext()
	if !Strategy.NamespaceScoped() {
		t.Errorf("ReplicaSet must be namespace scoped")
	}
	if Strategy.AllowCreateOnUpdate() {
		t.Errorf("ReplicaSet should not allow create on update")
	}

	validSelector := map[string]string{"a": "b"}
	validPodTemplate := api.PodTemplate{
		Template: api.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: validSelector,
			},
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyAlways,
				DNSPolicy:     api.DNSClusterFirst,
				Containers:    []api.Container{{Name: "abc", Image: "image", ImagePullPolicy: "IfNotPresent", TerminationMessagePolicy: api.TerminationMessageReadFile}},
			},
		},
	}
	rs := &extensions.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "abc", Namespace: metav1.NamespaceDefault},
		Spec: extensions.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: validSelector},
			Template: validPodTemplate.Template,
		},
		Status: extensions.ReplicaSetStatus{
			Replicas:           1,
			ObservedGeneration: int64(10),
		},
	}

	Strategy.PrepareForCreate(ctx, rs)
	if rs.Status.Replicas != 0 {
		t.Error("ReplicaSet should not allow setting status.replicas on create")
	}
	if rs.Status.ObservedGeneration != int64(0) {
		t.Error("ReplicaSet should not allow setting status.observedGeneration on create")
	}
	errs := Strategy.Validate(ctx, rs)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	invalidRc := &extensions.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "bar", ResourceVersion: "4"},
	}
	Strategy.PrepareForUpdate(ctx, invalidRc, rs)
	errs = Strategy.ValidateUpdate(ctx, invalidRc, rs)
	if len(errs) == 0 {
		t.Errorf("Expected a validation error")
	}
	if invalidRc.ResourceVersion != "4" {
		t.Errorf("Incoming resource version on update should not be mutated")
	}
}

func TestReplicaSetStatusStrategy(t *testing.T) {
	ctx := genericapirequest.NewDefaultContext()
	if !StatusStrategy.NamespaceScoped() {
		t.Errorf("ReplicaSet must be namespace scoped")
	}
	if StatusStrategy.AllowCreateOnUpdate() {
		t.Errorf("ReplicaSet should not allow create on update")
	}
	validSelector := map[string]string{"a": "b"}
	validPodTemplate := api.PodTemplate{
		Template: api.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: validSelector,
			},
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyAlways,
				DNSPolicy:     api.DNSClusterFirst,
				Containers:    []api.Container{{Name: "abc", Image: "image", ImagePullPolicy: "IfNotPresent"}},
			},
		},
	}
	oldRS := &extensions.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "abc", Namespace: metav1.NamespaceDefault, ResourceVersion: "10"},
		Spec: extensions.ReplicaSetSpec{
			Replicas: 3,
			Selector: &metav1.LabelSelector{MatchLabels: validSelector},
			Template: validPodTemplate.Template,
		},
		Status: extensions.ReplicaSetStatus{
			Replicas:           1,
			ObservedGeneration: int64(10),
		},
	}
	newRS := &extensions.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "abc", Namespace: metav1.NamespaceDefault, ResourceVersion: "9"},
		Spec: extensions.ReplicaSetSpec{
			Replicas: 1,
			Selector: &metav1.LabelSelector{MatchLabels: validSelector},
			Template: validPodTemplate.Template,
		},
		Status: extensions.ReplicaSetStatus{
			Replicas:           3,
			ObservedGeneration: int64(11),
		},
	}
	StatusStrategy.PrepareForUpdate(ctx, newRS, oldRS)
	if newRS.Status.Replicas != 3 {
		t.Errorf("ReplicaSet status updates should allow change of replicas: %v", newRS.Status.Replicas)
	}
	if newRS.Spec.Replicas != 3 {
		t.Errorf("PrepareForUpdate should have preferred spec")
	}
	errs := StatusStrategy.ValidateUpdate(ctx, newRS, oldRS)
	if len(errs) != 0 {
		t.Errorf("Unexpected error %v", errs)
	}
}

func TestSelectorImmutability(t *testing.T) {
	tests := []struct {
		method                 string
		url                    string
		oldSelectorLabels      map[string]string
		newSelectorLabels      map[string]string
		expectedAPIVersion     string
		expectedSelectorLabels map[string]string
	}{
		{"GET", "/api/v1beta2/namespaces", map[string]string{"a": "b"}, map[string]string{"c": "d"}, "v1beta2", map[string]string{"a": "b"}},
		{"GET", "/api/v1beta1/namespaces", map[string]string{"a": "b"}, map[string]string{"c": "d"}, "v1beta1", map[string]string{"c": "d"}},
	}

	resolver := newTestRequestInfoResolver()

	for _, test := range tests {
		req, _ := http.NewRequest(test.method, test.url, nil)

		apiRequestInfo, err := resolver.NewRequestInfo(req)
		if err != nil {
			t.Errorf("Unexpected error for url: %s %v", test.url, err)
		}
		if !apiRequestInfo.IsResourceRequest {
			t.Errorf("Expected resource request")
		}
		if test.expectedAPIVersion != apiRequestInfo.APIVersion {
			t.Errorf("Unexpected apiVersion for url: %s, expected: %s, actual: %s", test.url, test.expectedAPIVersion, apiRequestInfo.APIVersion)
		}

		oldReplicaSet := newReplicaSetWithSelectorLabels(&test.oldSelectorLabels)
		newReplicaSet := newReplicaSetWithSelectorLabels(&test.newSelectorLabels)

		context := genericapirequest.NewContext()
		context = genericapirequest.WithRequestInfo(context, apiRequestInfo)

		rsStrategy{}.PrepareForUpdate(context, newReplicaSet, oldReplicaSet)

		if !reflect.DeepEqual(test.expectedSelectorLabels, newReplicaSet.Spec.Selector.MatchLabels) {
			t.Errorf("Unexpected Spec.Selector, expected: %v, actual: %v", test.expectedSelectorLabels, newReplicaSet.Spec.Selector.MatchLabels)
		}
	}
}

func newTestRequestInfoResolver() *genericapirequest.RequestInfoFactory {
	return &genericapirequest.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
}

func newReplicaSetWithSelectorLabels(selectorLabels *map[string]string) *extensions.ReplicaSet {
	return &extensions.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: extensions.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels:      *selectorLabels,
				MatchExpressions: []metav1.LabelSelectorRequirement{},
			},
		},
	}
}
