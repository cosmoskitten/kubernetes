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

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util/types"
)

func Test_AddValidPvcUpdate(t *testing.T) {
	resizeMap := createTestVolumeResizeMap()
	claim1 := testVolumeClaim("foo", "ns", v1.PersistentVolumeClaimSpec{
		AccessModes: []v1.PersistentVolumeAccessMode{
			v1.ReadWriteOnce,
			v1.ReadOnlyMany,
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse("10G"),
			},
		},
	})

	claimClone, _ := clonePVC(claim1)
	claimClone.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse("12G")
	volumeSpec := getTestVolumeSpec("foo", "disk-foobar")
	resizeMap.AddPvcUpdate(claimClone, claim1, volumeSpec)
	pvcr := resizeMap.GetPvcsWithResizeRequest()
	if len(pvcr) != 1 {
		t.Fatalf("Expected 1 pvc resize request got 0")
	}
	assert.Equal(t, resource.MustParse("12G"), pvcr[0].ExpectedSize)
	assert.Equal(t, 0, len(resizeMap.pvcrs))
}

func createTestVolumeResizeMap() *volumeResizeMap {
	fakeClient := &fake.Clientset{}
	resizeMap := &volumeResizeMap{}
	resizeMap.pvcrs = make(map[types.UniquePvcName]*PvcWithResizeRequest)
	resizeMap.kubeClient = fakeClient
	return resizeMap
}

func testVolumeClaim(name string, namespace string, spec v1.PersistentVolumeClaimSpec) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       spec,
	}
}

// GetTestVolumeSpec returns a test volume spec
func getTestVolumeSpec(volumeName string, diskName v1.UniqueVolumeName) *volume.Spec {
	return &volume.Spec{
		Volume: &v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{
					PDName:   string(diskName),
					FSType:   "fake",
					ReadOnly: false,
				},
			},
		},
		PersistentVolume: &v1.PersistentVolume{
			Spec: v1.PersistentVolumeSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteOnce,
				},
			},
		},
	}
}
