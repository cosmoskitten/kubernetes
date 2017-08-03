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
	"k8s.io/kubernetes/pkg/apis/rbac"
)

func TestClusterRoleBindingGenerate(t *testing.T) {
	tests := []struct {
		params    map[string]interface{}
		expected  *rbac.ClusterRoleBinding
		expectErr bool
		reason    string
	}{
		{
			params: map[string]interface{}{
				"name":           "foo",
				"clusterrole":    "admin",
				"user":           []string{"user"},
				"group":          []string{"group"},
				"serviceaccount": []string{"ns1:name1"},
			},
			expected: &rbac.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     "admin",
				},
				Subjects: []rbac.Subject{
					{
						APIGroup: rbac.GroupName,
						Kind:     rbac.UserKind,
						Name:     "user",
					},
					{
						APIGroup: rbac.GroupName,
						Kind:     rbac.GroupKind,
						Name:     "group",
					},
					{
						Kind:      rbac.ServiceAccountKind,
						APIGroup:  "",
						Namespace: "ns1",
						Name:      "name1",
					},
				},
			},
			expectErr: false,
		},
		{
			params: map[string]interface{}{
				"name":           "foo",
				"clusterrole":    "admin",
				"user":           []string{"user1", "user2"},
				"group":          []string{"group1", "group2"},
				"serviceaccount": []string{"ns1:name1", "ns2:name2"},
			},
			expected: &rbac.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     "admin",
				},
				Subjects: []rbac.Subject{
					{
						APIGroup: rbac.GroupName,
						Kind:     rbac.UserKind,
						Name:     "user1",
					},
					{
						APIGroup: rbac.GroupName,
						Kind:     rbac.UserKind,
						Name:     "user2",
					},
					{
						APIGroup: rbac.GroupName,
						Kind:     rbac.GroupKind,
						Name:     "group1",
					},
					{
						APIGroup: rbac.GroupName,
						Kind:     rbac.GroupKind,
						Name:     "group2",
					},
					{
						Kind:      rbac.ServiceAccountKind,
						APIGroup:  "",
						Namespace: "ns1",
						Name:      "name1",
					},
					{
						Kind:      rbac.ServiceAccountKind,
						APIGroup:  "",
						Namespace: "ns2",
						Name:      "name2",
					},
				},
			},
			expectErr: false,
		},
		{
			params: map[string]interface{}{
				"name":        "foo",
				"clusterrole": "admin",
			},
			expected: &rbac.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     "admin",
				},
			},
			expectErr: false,
		},
		{
			params: map[string]interface{}{
				"name":           "role",
				"clusterrole":    "admin",
				"user":           []string{"user"},
				"group":          []string{"group"},
				"serviceaccount": []string{"ns1-name1"},
			},
			expectErr: true,
			reason:    "invalid serviceaccount, expected format: <namespace:name>",
		},
		{
			params: map[string]interface{}{
				"name":           "",
				"clusterrole":    "admin",
				"user":           []string{"user"},
				"group":          []string{"group"},
				"serviceaccount": []string{"ns1:name1"},
			},
			expectErr: true,
			reason:    "name must be specified",
		},
		{
			params: map[string]interface{}{
				"name":           "role",
				"clusterrole":    "admin",
				"user":           "user",
				"group":          []string{"group"},
				"serviceaccount": []string{"ns1:name1"},
			},
			expectErr: true,
			reason:    "expected user []string",
		},
		{
			params: map[string]interface{}{
				"name":           "role",
				"clusterrole":    "admin",
				"user":           []string{"user"},
				"group":          "group",
				"serviceaccount": []string{"ns1:name1"},
			},
			expectErr: true,
			reason:    "expected group []string",
		},
		{
			params: map[string]interface{}{
				"name":           "role",
				"clusterrole":    "admin",
				"user":           []string{"user"},
				"group":          []string{"group"},
				"serviceaccount": "ns1",
			},
			expectErr: true,
			reason:    "expected serviceaccount []string",
		},
	}
	generator := ClusterRoleBindingGeneratorV1{}
	for i, test := range tests {
		obj, err := generator.Generate(test.params)
		if !test.expectErr && err != nil {
			t.Errorf("[%d] unexpected error: %v", i, err)
		}
		if test.expectErr && err != nil {
			continue
		}
		if test.expectErr && err == nil {
			t.Errorf("[%d] expect error, reason: %v, got nil", i, test.reason)
		}
		if !reflect.DeepEqual(obj.(*rbac.ClusterRoleBinding), test.expected) {
			t.Errorf("\n[%d] want:\n%#v\n[%d] got:\n%#v", i, test.expected, i, obj.(*rbac.ClusterRoleBinding))
		}
	}
}
