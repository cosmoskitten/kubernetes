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

package daemonset

import (
	"net/http"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	_ "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

func TestDefaultGarbageCollectionPolicy(t *testing.T) {
	// Make sure we correctly implement the interface.
	// Otherwise a typo could silently change the default.
	var gcds rest.GarbageCollectionDeleteStrategy = Strategy
	if got, want := gcds.DefaultGarbageCollectionPolicy(), rest.OrphanDependents; got != want {
		t.Errorf("DefaultGarbageCollectionPolicy() = %#v, want %#v", got, want)
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

		oldDaemonSet := newDaemonSetWithSelectorLabels(&test.oldSelectorLabels)
		newDaemonSet := newDaemonSetWithSelectorLabels(&test.newSelectorLabels)

		context := genericapirequest.NewContext()
		context = genericapirequest.WithRequestInfo(context, apiRequestInfo)

		daemonSetStrategy{}.PrepareForUpdate(context, newDaemonSet, oldDaemonSet)

		if !reflect.DeepEqual(test.expectedSelectorLabels, newDaemonSet.Spec.Selector.MatchLabels) {
			t.Errorf("Unexpected Spec.Selector, expected: %v, actual: %v", test.expectedSelectorLabels, newDaemonSet.Spec.Selector.MatchLabels)
		}
	}
}

func newTestRequestInfoResolver() *genericapirequest.RequestInfoFactory {
	return &genericapirequest.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
}

func newDaemonSetWithSelectorLabels(selectorLabels *map[string]string) *extensions.DaemonSet {
	return &extensions.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: extensions.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels:      *selectorLabels,
				MatchExpressions: []metav1.LabelSelectorRequirement{},
			},
		},
	}
}
