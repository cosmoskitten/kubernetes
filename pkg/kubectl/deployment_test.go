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

package kubectl

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeploymentGenerate(t *testing.T) {
	one := int32(1)
	tests := []struct {
		params    map[string]interface{}
		expected  *extensionsv1beta1.Deployment
		expectErr bool
	}{
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": []string{"abc/app:v4"},
			},
			expected: &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"app": "foo"},
				},
				Spec: extensionsv1beta1.DeploymentSpec{
					Replicas: &one,
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "foo"},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{{Name: "app", Image: "abc/app:v4"}},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": []string{"abc/app:v4", "zyx/ape"},
			},
			expected: &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"app": "foo"},
				},
				Spec: extensionsv1beta1.DeploymentSpec{
					Replicas: &one,
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "foo"},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{{Name: "app", Image: "abc/app:v4"},
								{Name: "ape", Image: "zyx/ape"}},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			params:    map[string]interface{}{},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name": 1,
			},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name": nil,
			},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": []string{},
			},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"NAME": "some_value",
			},
			expectErr: true,
		},
	}
	generator := DeploymentBasicGeneratorV1{}
	for index, test := range tests {
		t.Logf("running scenario %d", index)
		obj, err := generator.Generate(test.params)
		switch {
		case test.expectErr && err != nil:
			continue // loop, since there's no output to check
		case test.expectErr && err == nil:
			t.Errorf("expected error and didn't get one")
			continue // loop, no expected output object
		case !test.expectErr && err != nil:
			t.Errorf("unexpected error %v", err)
			continue // loop, no output object
		case !test.expectErr && err == nil:
			// do nothing and drop through
		}
		expectedBytes, _ := json.MarshalIndent(test.expected, "", "\t")
		receivedBytes, _ := json.MarshalIndent(obj, "", "\t")
		expectedStr, receivedStr := string(expectedBytes), string(receivedBytes)
		if expectedStr != receivedStr {
			t.Errorf("expected:\n%s\nsaw:\n%s", expectedStr, receivedStr)
		}
	}
}

func TestAppsDeploymentGenerate(t *testing.T) {
	one := int32(1)
	tests := []struct {
		params    map[string]interface{}
		expected  *appsv1beta1.Deployment
		expectErr bool
	}{
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": []string{"abc/app:v4"},
			},
			expected: &appsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"app": "foo"},
				},
				Spec: appsv1beta1.DeploymentSpec{
					Replicas: &one,
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "foo"},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{{Name: "app", Image: "abc/app:v4"}},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": []string{"abc/app:v4", "zyx/ape"},
			},
			expected: &appsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"app": "foo"},
				},
				Spec: appsv1beta1.DeploymentSpec{
					Replicas: &one,
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "foo"},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{{Name: "app", Image: "abc/app:v4"},
								{Name: "ape", Image: "zyx/ape"}},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			params:    map[string]interface{}{},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name": 1,
			},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name": nil,
			},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"name":  "foo",
				"image": []string{},
			},
			expectErr: true,
		},
		{
			params: map[string]interface{}{
				"NAME": "some_value",
			},
			expectErr: true,
		},
	}
	generator := DeploymentBasicAppsGeneratorV1{}
	for index, test := range tests {
		t.Logf("running scenario %d", index)
		obj, err := generator.Generate(test.params)
		switch {
		case test.expectErr && err != nil:
			continue // loop, since there's no output to check
		case test.expectErr && err == nil:
			t.Errorf("expected error and didn't get one")
			continue // loop, no expected output object
		case !test.expectErr && err != nil:
			t.Errorf("unexpected error %v", err)
			continue // loop, no output object
		case !test.expectErr && err == nil:
			// do nothing and drop through
		}
		expectedBytes, _ := json.MarshalIndent(test.expected, "", "\t")
		receivedBytes, _ := json.MarshalIndent(obj, "", "\t")
		expectedStr, receivedStr := string(expectedBytes), string(receivedBytes)
		if expectedStr != receivedStr {
			t.Errorf("expected:\n%s\nsaw:\n%s", expectedStr, receivedStr)
		}
	}
}

func TestBaseDeploymentGenerator_validate(t *testing.T) {
	// Valid params should not result in an error.
	b := BaseDeploymentGenerator{
		Name:    "my-deployment",
		Images:  []string{"nginx"},
		Command: []string{"/bin/bash"},
	}
	assert.NoError(t, b.validate())

	// You should not be able to specify a Command when there are multiple
	// Images.
	b = BaseDeploymentGenerator{
		Name:    "my-deployment",
		Images:  []string{"nginx", "alpine"},
		Command: []string{"/bin/bash"},
	}
	assert.Error(t, b.validate())

	// But multiple Images with no Command is fine.
	b = BaseDeploymentGenerator{
		Name:   "my-deployment",
		Images: []string{"nginx", "alpine"},
	}
	assert.NoError(t, b.validate())
}
