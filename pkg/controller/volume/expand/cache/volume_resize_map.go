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

package cache

import (
	"fmt"
	"sync"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/pkg/controller/volume/expand/util"
	"k8s.io/kubernetes/pkg/util/strings"
	"k8s.io/kubernetes/pkg/volume/util/types"
)

// VolumeResizeMap defines an interface that serves as a cache for holding pending resizing requests
type VolumeResizeMap interface {
	AddPVCUpdate(pvc *v1.PersistentVolumeClaim, pv *v1.PersistentVolume)

	DeletePVC(pvc *v1.PersistentVolumeClaim)
	GetPVCsWithResizeRequest() []*PVCWithResizeRequest
	// Mark this volume as resize
	MarkAsResized(*PVCWithResizeRequest, resource.Quantity) error
	UpdatePVSize(*PVCWithResizeRequest, resource.Quantity) error
	MarkResizeFailed(*PVCWithResizeRequest, string) error
}

type volumeResizeMap struct {
	// map of unique pvc name and resize requests that are pending or inflight
	pvcrs map[types.UniquePvcName]*PVCWithResizeRequest
	// kube client for making API calls
	kubeClient clientset.Interface
	// for guarding access to pvcrs map
	sync.RWMutex
}

// PVCWithResizeRequest struct defines data structure that stores state needed for
// performing file system resize
type PVCWithResizeRequest struct {
	// PVC that needs to be resized
	PVC *v1.PersistentVolumeClaim
	// persistentvolume
	PersistentVolume *v1.PersistentVolume
	// Current volume size
	CurrentSize resource.Quantity
	// Expended volume size
	ExpectedSize resource.Quantity
}

// UniquePvcKey returns unique key of the PVC based on its UID
func (pvcr *PVCWithResizeRequest) UniquePvcKey() types.UniquePvcName {
	return types.UniquePvcName(pvcr.PVC.UID)
}

// QualifiedName returns namespace and name combination of the PVC
func (pvcr *PVCWithResizeRequest) QualifiedName() string {
	return strings.JoinQualifiedName(pvcr.PVC.Namespace, pvcr.PVC.Name)
}

// NewVolumeResizeMap returns new VolumeResizeMap which acts as a cache
// for holding pending resize requests.
func NewVolumeResizeMap(kubeClient clientset.Interface) VolumeResizeMap {
	resizeMap := &volumeResizeMap{}
	resizeMap.pvcrs = make(map[types.UniquePvcName]*PVCWithResizeRequest)
	resizeMap.kubeClient = kubeClient
	return resizeMap
}

func (resizeMap *volumeResizeMap) AddPVCUpdate(pvc *v1.PersistentVolumeClaim, pv *v1.PersistentVolume) {
	if pvc.Namespace != pv.Spec.ClaimRef.Namespace || pvc.Name != pv.Spec.ClaimRef.Name {
		glog.Errorf("Persistent Volume is not mapped to PVC being updated : %s", util.ClaimToClaimKey(pvc))
		return
	}

	if pvc.Status.Phase != v1.ClaimBound {
		return
	}

	resizeMap.Lock()
	defer resizeMap.Unlock()

	pvcSize := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	pvcStatusSize := pvc.Status.Capacity[v1.ResourceStorage]

	if pvcStatusSize.Cmp(pvcSize) >= 0 {
		return
	}

	glog.V(4).Infof("Adding pvc %s with Size %s/%s for resizing", util.ClaimToClaimKey(pvc), pvcSize.String(), pvcStatusSize.String())

	pvcRequest := &PVCWithResizeRequest{
		PVC:              pvc,
		CurrentSize:      pvcStatusSize,
		ExpectedSize:     pvcSize,
		PersistentVolume: pv,
	}
	resizeMap.pvcrs[types.UniquePvcName(pvc.UID)] = pvcRequest
}

// Return Pvcrs that require resize
func (resizeMap *volumeResizeMap) GetPVCsWithResizeRequest() []*PVCWithResizeRequest {
	resizeMap.Lock()
	defer resizeMap.Unlock()

	pvcrs := []*PVCWithResizeRequest{}
	for _, pvcr := range resizeMap.pvcrs {
		pvcrs = append(pvcrs, pvcr)
	}
	// Empty out pvcrs map, we will add back failed resize requests later
	resizeMap.pvcrs = map[types.UniquePvcName]*PVCWithResizeRequest{}
	return pvcrs
}

// DeletePVC removes given pvc object from list of pvcs that needs resizing.
// deleting a pvc in this map doesn't affect operations that are already inflight.
func (resizeMap *volumeResizeMap) DeletePVC(pvc *v1.PersistentVolumeClaim) {
	resizeMap.Lock()
	defer resizeMap.Unlock()
	pvcUniqueName := types.UniquePvcName(pvc.UID)
	glog.V(5).Infof("Removing PVC %v from resize map", pvcUniqueName)
	delete(resizeMap.pvcrs, pvcUniqueName)
}

func (resizeMap *volumeResizeMap) MarkAsResized(pvcr *PVCWithResizeRequest, newSize resource.Quantity) error {
	resizeMap.Lock()
	defer resizeMap.Unlock()

	emptyCondition := []v1.PersistentVolumeClaimCondition{}

	err := resizeMap.updatePvcCapacityAndConditions(pvcr, newSize, emptyCondition)
	if err != nil {
		glog.V(4).Infof("Error updating PV spec capacity for volume %q with : %v", pvcr.QualifiedName(), err)
		resizeMap.addBackResizeRequest(pvcr)
		return err
	}
	return nil
}

func (resizeMap *volumeResizeMap) MarkResizeFailed(pvcr *PVCWithResizeRequest, reason string) error {
	resizeMap.Lock()
	defer resizeMap.Unlock()
	resizeMap.addBackResizeRequest(pvcr)
	return nil
}

func (resizeMap *volumeResizeMap) UpdatePVSize(pvcr *PVCWithResizeRequest, newSize resource.Quantity) error {
	resizeMap.Lock()
	defer resizeMap.Unlock()

	err := resizeMap.updatePvSize(pvcr, newSize)
	if err != nil {
		resizeMap.addBackResizeRequest(pvcr)
		return err
	}
	return nil
}

func (resizeMap *volumeResizeMap) updatePvSize(pvcr *PVCWithResizeRequest, newSize resource.Quantity) error {
	oldPv := pvcr.PersistentVolume
	clone, err := scheme.Scheme.DeepCopy(oldPv)

	if err != nil {
		return fmt.Errorf("Error cloning PV %q with error : %v", oldPv.Name, err)
	}
	pvClone, ok := clone.(*v1.PersistentVolume)

	if !ok {
		return fmt.Errorf("Unexpected cast error for PV : %v", pvClone)
	}

	pvClone.Spec.Capacity[v1.ResourceStorage] = newSize
	_, updateErr := resizeMap.kubeClient.CoreV1().PersistentVolumes().Update(pvClone)

	if updateErr != nil {
		glog.V(4).Infof("Error updating pv %q with error : %v", pvClone.Name, updateErr)
		return updateErr
	}
	return nil
}

func (resizeMap *volumeResizeMap) updatePvcCapacity(pvcr *PVCWithResizeRequest, newSize resource.Quantity) error {
	claimClone, err := util.ClonePVC(pvcr.PVC)
	if err != nil {
		return err
	}

	claimClone.Status.Capacity[v1.ResourceStorage] = newSize
	_, updateErr := resizeMap.kubeClient.Core().PersistentVolumeClaims(claimClone.Namespace).UpdateStatus(claimClone)
	if updateErr != nil {
		glog.V(4).Infof("updating PersistentVolumeClaim[%s] status: failed: %v", pvcr.QualifiedName(), updateErr)
		return updateErr
	}
	return nil
}

func (resizeMap *volumeResizeMap) updatePvcCapacityAndConditions(pvcr *PVCWithResizeRequest, newSize resource.Quantity, pvcConditions []v1.PersistentVolumeClaimCondition) error {

	claimClone, err := util.ClonePVC(pvcr.PVC)
	if err != nil {
		return err
	}

	claimClone.Status.Capacity[v1.ResourceStorage] = newSize
	claimClone.Status.Conditions = pvcConditions
	_, updateErr := resizeMap.kubeClient.CoreV1().PersistentVolumeClaims(claimClone.Namespace).UpdateStatus(claimClone)
	if updateErr != nil {
		glog.V(4).Infof("updating PersistentVolumeClaim[%s] status: failed: %v", pvcr.QualifiedName(), updateErr)
		return updateErr
	}
	return nil
}

// add back the resize request because it may have failed. Assumes that caller has necessary locks.
func (resizeMap *volumeResizeMap) addBackResizeRequest(pvcr *PVCWithResizeRequest) {
	pvcUniqueName := pvcr.UniquePvcKey()
	if _, ok := resizeMap.pvcrs[pvcUniqueName]; ok {
		glog.V(5).Infof("Found another resize request pending for volume %s", pvcr.QualifiedName())
		return
	}
	pvc, err := resizeMap.kubeClient.CoreV1().PersistentVolumeClaims(pvcr.PVC.Namespace).Get(pvcr.PVC.Name, metav1.GetOptions{})
	// if pvc has been deleted since it was requested for resizing, this could result in error fetching PVC
	if err != nil {
		glog.V(5).Infof("Error fetching pvc %v/%v for resize with error : %v", pvc.Namespace, pvc.Name, err)
		return
	}
	pv, pvfetchErr := resizeMap.kubeClient.CoreV1().PersistentVolumes().Get(pvc.Spec.VolumeName, metav1.GetOptions{})
	if pvfetchErr != nil {
		glog.Errorf("Error fetching pv %v for resize with error : %v", pvc.Spec.VolumeName, err)
		return
	}
	newSize := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	resizeMap.pvcrs[pvcUniqueName] = &PVCWithResizeRequest{
		PVC:              pvc,
		CurrentSize:      pvc.Status.Capacity[v1.ResourceStorage],
		ExpectedSize:     newSize,
		PersistentVolume: pv,
	}
}
