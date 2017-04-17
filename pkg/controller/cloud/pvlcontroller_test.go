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

package cloud

import (
	"encoding/json"
	"testing"

	"k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"

	fakecloud "k8s.io/kubernetes/pkg/cloudprovider/providers/fake"
)

func TestCreatePatch(t *testing.T) {
	ignoredPV := v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "noncloud",
			Initializers: &metav1.Initializers{
				Pending: []metav1.Initializer{
					{
						Name: initializerName,
					},
				},
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/",
				},
			},
		},
	}
	awsPV := v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "awsPV",
			Initializers: &metav1.Initializers{
				Pending: []metav1.Initializer{
					{
						Name: initializerName,
					},
				},
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				AWSElasticBlockStore: &v1.AWSElasticBlockStoreVolumeSource{
					VolumeID: "123",
				},
			},
		},
	}

	testCases := map[string]struct {
		vol    v1.PersistentVolume
		labels map[string]string
	}{
		"non-cloud PV": {
			vol:    ignoredPV,
			labels: nil,
		},
		"no labels": {
			vol:    awsPV,
			labels: nil,
		},
		"cloudprovider returns nil, nil": {
			vol:    awsPV,
			labels: nil,
		},
		"cloudprovider labels": {
			vol:    awsPV,
			labels: map[string]string{"a": "1", "b": "2"},
		},
	}

	for d, tc := range testCases {
		cloud := &fakecloud.FakeCloud{}
		client := fake.NewSimpleClientset()
		pvlController := NewPersistentVolumeLabelController(client, cloud)
		patch, err := pvlController.createPatch(&tc.vol, tc.labels)
		if err != nil {
			t.Errorf("%s: createPatch returned err: %v", d, err)
		}
		obj := &v1.PersistentVolume{}
		json.Unmarshal(patch, obj)
		if tc.labels != nil {
			for k, v := range tc.labels {
				if obj.ObjectMeta.Labels[k] != v {
					t.Errorf("%s: label %s expected %s got %s", d, k, v, obj.ObjectMeta.Labels[k])
				}
			}
		}
		if obj.ObjectMeta.Initializers != nil {
			t.Errorf("%s: initializer wasn't removed: %v", d, obj.ObjectMeta.Initializers)
		}
	}
}
