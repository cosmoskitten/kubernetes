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

package deployment

import (
	"net/http"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

func TestStatusUpdates(t *testing.T) {
	tests := []struct {
		old      runtime.Object
		obj      runtime.Object
		expected runtime.Object
	}{
		{
			old:      newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"}),
			obj:      newDeployment(map[string]string{"test": "label", "sneaky": "label"}, map[string]string{"test": "annotation"}),
			expected: newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"}),
		},
		{
			old:      newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation"}),
			obj:      newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation", "sneaky": "annotation"}),
			expected: newDeployment(map[string]string{"test": "label"}, map[string]string{"test": "annotation", "sneaky": "annotation"}),
		},
	}

	for _, test := range tests {
		deploymentStatusStrategy{}.PrepareForUpdate(genericapirequest.NewContext(), test.obj, test.old)
		if !reflect.DeepEqual(test.expected, test.obj) {
			t.Errorf("Unexpected object mismatch! Expected:\n%#v\ngot:\n%#v", test.expected, test.obj)
		}
	}
}

func newDeployment(labels, annotations map[string]string) *extensions.Deployment {
	return &extensions.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: 1,
			Strategy: extensions.DeploymentStrategy{
				Type: extensions.RecreateDeploymentStrategyType,
			},
			Template: api.PodTemplateSpec{
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "test",
							Image: "test",
						},
					},
				},
			},
		},
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

		oldDeployment := newDeploymentWithSelectorLabels(&test.oldSelectorLabels)
		newDeployment := newDeploymentWithSelectorLabels(&test.newSelectorLabels)

		context := genericapirequest.NewContext()
		context = genericapirequest.WithRequestInfo(context, apiRequestInfo)

		deploymentStrategy{}.PrepareForUpdate(context, newDeployment, oldDeployment)

		if !reflect.DeepEqual(test.expectedSelectorLabels, newDeployment.Spec.Selector.MatchLabels) {
			t.Errorf("Unexpected Spec.Selector, expected: %v, actual: %v", test.expectedSelectorLabels, newDeployment.Spec.Selector.MatchLabels)
		}
	}
}

func newTestRequestInfoResolver() *genericapirequest.RequestInfoFactory {
	return &genericapirequest.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
}

func newDeploymentWithSelectorLabels(selectorLabels *map[string]string) *extensions.Deployment {
	return &extensions.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: extensions.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels:      *selectorLabels,
				MatchExpressions: []metav1.LabelSelectorRequirement{},
			},
		},
	}
}
