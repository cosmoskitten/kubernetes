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
	"fmt"
	"testing"
	"time"

	"k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"k8s.io/kubernetes/pkg/cloudprovider/providers/aws"
	fakecloud "k8s.io/kubernetes/pkg/cloudprovider/providers/fake"
)

type mockVolumes struct {
	volumeLabels      map[string]string
	volumeLabelsError error
}

var _ aws.Volumes = &mockVolumes{}

func (v *mockVolumes) AttachDisk(diskName aws.KubernetesVolumeID, nodeName types.NodeName, readOnly bool) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (v *mockVolumes) DetachDisk(diskName aws.KubernetesVolumeID, nodeName types.NodeName) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (v *mockVolumes) CreateDisk(volumeOptions *aws.VolumeOptions) (volumeName aws.KubernetesVolumeID, err error) {
	return "", fmt.Errorf("not implemented")
}

func (v *mockVolumes) DeleteDisk(volumeName aws.KubernetesVolumeID) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (v *mockVolumes) GetVolumeLabels(volumeName aws.KubernetesVolumeID) (map[string]string, error) {
	return v.volumeLabels, v.volumeLabelsError
}

func (v *mockVolumes) GetDiskPath(volumeName aws.KubernetesVolumeID) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (v *mockVolumes) DiskIsAttached(volumeName aws.KubernetesVolumeID, nodeName types.NodeName) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (v *mockVolumes) DisksAreAttached(nodeDisks map[types.NodeName][]aws.KubernetesVolumeID) (map[types.NodeName]map[aws.KubernetesVolumeID]bool, error) {
	return nil, fmt.Errorf("not implemented")
}

func mockVolumeFailure(err error) *mockVolumes {
	return &mockVolumes{volumeLabelsError: err}
}

func mockVolumeLabels(labels map[string]string) *mockVolumes {
	return &mockVolumes{volumeLabels: labels}
}

func TestPVLabels(t *testing.T) {
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
		vol          v1.PersistentVolume
		volumeLabels *mockVolumes
	}{
		"non-cloud PV": {
			vol:          ignoredPV,
			volumeLabels: nil,
		},
		"no labels": {
			vol:          awsPV,
			volumeLabels: mockVolumeLabels(make(map[string]string)),
		},
		"cloudprovider returns nil, nil": {
			vol:          awsPV,
			volumeLabels: mockVolumeFailure(nil),
		},
		"cloudprovider labels": {
			vol:          awsPV,
			volumeLabels: mockVolumeLabels(map[string]string{"a": "1", "b": "2"}),
		},
	}

	for d, tc := range testCases {
		correctPatch := false
		cloud := &fakecloud.FakeCloud{}
		client := fake.NewSimpleClientset()
		fakeWatch := watch.NewFake()
		client.PrependWatchReactor("persistentvolumes", core.DefaultWatchReactor(fakeWatch, nil))
		client.PrependReactor("patch", "persistentvolumes", func(a core.Action) (handled bool, ret runtime.Object, err error) {
			patch := a.(core.PatchActionImpl).GetPatch()
			obj := &v1.PersistentVolume{}
			json.Unmarshal(patch, obj)
			if tc.volumeLabels != nil {
				for k, v := range tc.volumeLabels.volumeLabels {
					if obj.ObjectMeta.Labels[k] != v {
						t.Errorf("%s: label %s expected %s got %s", d, k, v, obj.ObjectMeta.Labels[k])
						return false, nil, nil
					}
				}
			}
			if obj.ObjectMeta.Initializers != nil {
				t.Errorf("%s: initializer wasn't removed: %v", d, obj.ObjectMeta.Initializers)
				return false, nil, nil
			}
			correctPatch = true
			return true, nil, nil
		})

		stopCh := make(chan struct{})
		pvlController := NewPersistentVolumeLabelController(client, cloud)
		pvlController.ebsVolumes = tc.volumeLabels

		go pvlController.Run(1, stopCh)
		fakeWatch.Add(&tc.vol)
		time.Sleep(500 * time.Millisecond)
		if correctPatch != true {
			t.Errorf("%s: patch operation failure", d)
		}
		close(stopCh)
	}
}
