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

package kubectl

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
)

func TestHPAGenerate(t *testing.T) {
	tests := []struct {
		min        int32
		cpuPercent int32
		params     map[string]interface{}
		expected   *autoscaling.HorizontalPodAutoscaler
		expectErr  bool
		reason     string
	}{
		{
			params: map[string]interface{}{
				"name":                "foo",
				"min":                 "1",
				"max":                 "10",
				"cpu-percent":         "80",
				"scaleRef-kind":       "kind",
				"scaleRef-name":       "name",
				"scaleRef-apiVersion": "apiVersion",
			},
			expected: &autoscaling.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: autoscaling.HorizontalPodAutoscalerSpec{
					Metrics: []autoscaling.MetricSpec{
						{
							Type: autoscaling.ResourceMetricSourceType,
							Resource: &autoscaling.ResourceMetricSource{
								Name: api.ResourceCPU,
								TargetAverageUtilization: newInt32(80),
							},
						},
					},
					ScaleTargetRef: autoscaling.CrossVersionObjectReference{
						Kind:       "kind",
						Name:       "name",
						APIVersion: "apiVersion",
					},
					MaxReplicas: int32(10),
					MinReplicas: newInt32(1),
				},
			},
			expectErr: false,
		},
		{
			params: map[string]interface{}{
				"scaleRef-kind":       "kind",
				"scaleRef-name":       "name",
				"scaleRef-apiVersion": "apiVersion",
			},
			expectErr: true,
			reason:    "'name' is a required parameter",
		},
		{
			params: map[string]interface{}{
				"default-name":        "foo",
				"scaleRef-kind":       "kind",
				"scaleRef-name":       "name",
				"scaleRef-apiVersion": "apiVersion",
			},
			expectErr: true,
			reason:    "'max' is a required parameter",
		},
		{
			params: map[string]interface{}{
				"name":                "foo",
				"min":                 "10",
				"max":                 "1",
				"scaleRef-kind":       "kind",
				"scaleRef-name":       "name",
				"scaleRef-apiVersion": "apiVersion",
			},
			expectErr: true,
			reason:    "'max' must be greater than or equal to 'min'",
		},
		{
			params: map[string]interface{}{
				"name":                "foo",
				"min":                 "1",
				"max":                 "10",
				"cpu-percent":         "",
				"scaleRef-kind":       "kind",
				"scaleRef-name":       "name",
				"scaleRef-apiVersion": "apiVersion",
			},
			expectErr: true,
			reason:    "cpu-percent must be an integer if specified",
		},
		{
			params: map[string]interface{}{
				"name":                "foo",
				"min":                 "foo",
				"max":                 "10",
				"cpu-percent":         "60",
				"scaleRef-kind":       "kind",
				"scaleRef-name":       "name",
				"scaleRef-apiVersion": "apiVersion",
			},
			expectErr: true,
			reason:    "'min' must be an integer if specified",
		},
		{
			params: map[string]interface{}{
				"name":                "foo",
				"min":                 "1",
				"max":                 "bar",
				"cpu-percent":         "90",
				"scaleRef-kind":       "kind",
				"scaleRef-name":       "name",
				"scaleRef-apiVersion": "apiVersion",
			},
			expectErr: true,
			reason:    "'max' must be an integer if specified",
		},
	}
	generator := HorizontalPodAutoscalerV1{}
	for i, test := range tests {
		obj, err := generator.Generate(test.params)
		if test.expectErr && err != nil {
			continue
		}
		if !test.expectErr && err != nil {
			t.Errorf("[%d] unexpected error: %v", i, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("Expect error, reason: %v, got nil", test.reason)
		}
		if !reflect.DeepEqual(obj.(*autoscaling.HorizontalPodAutoscaler), test.expected) {
			t.Errorf("\n[%d] want:\n%#v\n[%d] got:\n%#v", i, test.expected, i, obj.(*autoscaling.HorizontalPodAutoscaler))
		}
	}
}

func newInt32(value int) *int32 {
	v := int32(value)
	return &v
}
