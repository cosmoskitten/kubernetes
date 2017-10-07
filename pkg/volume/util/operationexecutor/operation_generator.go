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

package operationexecutor

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	expandcache "k8s.io/kubernetes/pkg/controller/volume/expand/cache"
	"k8s.io/kubernetes/pkg/features"
	kevents "k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/kubernetes/pkg/volume/util/volumehelper"
)

var _ OperationGenerator = &operationGenerator{}

type operationGenerator struct {
	// Used to fetch objects from the API server like Node in the
	// VerifyControllerAttachedVolume operation.
	kubeClient clientset.Interface

	// volumePluginMgr is the volume plugin manager used to create volume
	// plugin objects.
	volumePluginMgr *volume.VolumePluginMgr

	// recorder is used to record events in the API server
	recorder record.EventRecorder

	// checkNodeCapabilitiesBeforeMount, if set, enables the CanMount check,
	// which verifies that the components (binaries, etc.) required to mount
	// the volume are available on the underlying node before attempting mount.
	checkNodeCapabilitiesBeforeMount bool
}

// NewOperationGenerator is returns instance of operationGenerator
func NewOperationGenerator(kubeClient clientset.Interface,
	volumePluginMgr *volume.VolumePluginMgr,
	recorder record.EventRecorder,
	checkNodeCapabilitiesBeforeMount bool) OperationGenerator {

	return &operationGenerator{
		kubeClient:      kubeClient,
		volumePluginMgr: volumePluginMgr,
		recorder:        recorder,
		checkNodeCapabilitiesBeforeMount: checkNodeCapabilitiesBeforeMount,
	}
}

// OperationGenerator interface that extracts out the functions from operation_executor to make it dependency injectable
type OperationGenerator interface {
	// Generates the MountVolume function needed to perform the mount of a volume plugin
	GenerateMountVolumeFunc(waitForAttachTimeout time.Duration, volumeToMount VolumeToMount, actualStateOfWorldMounterUpdater ActualStateOfWorldMounterUpdater, isRemount bool) (func() error, string, error)

	// Generates the UnmountVolume function needed to perform the unmount of a volume plugin
	GenerateUnmountVolumeFunc(volumeToUnmount MountedVolume, actualStateOfWorld ActualStateOfWorldMounterUpdater) (func() error, string, error)

	// Generates the AttachVolume function needed to perform attach of a volume plugin
	GenerateAttachVolumeFunc(volumeToAttach VolumeToAttach, actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, string, error)

	// Generates the DetachVolume function needed to perform the detach of a volume plugin
	GenerateDetachVolumeFunc(volumeToDetach AttachedVolume, verifySafeToDetach bool, actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, string, error)

	// Generates the VolumesAreAttached function needed to verify if volume plugins are attached
	GenerateVolumesAreAttachedFunc(attachedVolumes []AttachedVolume, nodeName types.NodeName, actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, error)

	// Generates the UnMountDevice function needed to perform the unmount of a device
	GenerateUnmountDeviceFunc(deviceToDetach AttachedVolume, actualStateOfWorld ActualStateOfWorldMounterUpdater, mounter mount.Interface) (func() error, string, error)

	// Generates the function needed to check if the attach_detach controller has attached the volume plugin
	GenerateVerifyControllerAttachedVolumeFunc(volumeToMount VolumeToMount, nodeName types.NodeName, actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, string, error)

	// Generates the MapVolume function needed to perform the map of a volume plugin
	GenerateMapVolumeFunc(waitForAttachTimeout time.Duration, volumeToMount VolumeToMount, actualStateOfWorldMounterUpdater ActualStateOfWorldMounterUpdater) (func() error, string, error)

	// Generates the UnmapVolume function needed to perform the unmap of a volume plugin
	GenerateUnmapVolumeFunc(volumeToUnmount MountedVolume, actualStateOfWorld ActualStateOfWorldMounterUpdater) (func() error, string, error)

	// Generates the UnmapDevice function needed to perform the unmap of a device
	GenerateUnmapDeviceFunc(deviceToDetach AttachedVolume, actualStateOfWorld ActualStateOfWorldMounterUpdater, mounter mount.Interface) (func() error, string, error)

	// GetVolumePluginMgr returns volume plugin manager
	GetVolumePluginMgr() *volume.VolumePluginMgr

	GenerateBulkVolumeVerifyFunc(
		map[types.NodeName][]*volume.Spec,
		string,
		map[*volume.Spec]v1.UniqueVolumeName, ActualStateOfWorldAttacherUpdater) (func() error, error)

	GenerateExpandVolumeFunc(*expandcache.PVCWithResizeRequest, expandcache.VolumeResizeMap) (func() error, string, error)
}

func (og *operationGenerator) GenerateVolumesAreAttachedFunc(
	attachedVolumes []AttachedVolume,
	nodeName types.NodeName,
	actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, error) {

	// volumesPerPlugin maps from a volume plugin to a list of volume specs which belong
	// to this type of plugin
	volumesPerPlugin := make(map[string][]*volume.Spec)
	// volumeSpecMap maps from a volume spec to its unique volumeName which will be used
	// when calling MarkVolumeAsDetached
	volumeSpecMap := make(map[*volume.Spec]v1.UniqueVolumeName)
	// Iterate each volume spec and put them into a map index by the pluginName
	for _, volumeAttached := range attachedVolumes {
		if volumeAttached.VolumeSpec == nil {
			glog.Errorf("VerifyVolumesAreAttached.GenerateVolumesAreAttachedFunc: nil spec for volume %s", volumeAttached.VolumeName)
			continue
		}
		volumePlugin, err :=
			og.volumePluginMgr.FindPluginBySpec(volumeAttached.VolumeSpec)
		if err != nil || volumePlugin == nil {
			glog.Errorf(volumeAttached.GenerateErrorDetailed("VolumesAreAttached.FindPluginBySpec failed", err).Error())
		}
		volumeSpecList, pluginExists := volumesPerPlugin[volumePlugin.GetPluginName()]
		if !pluginExists {
			volumeSpecList = []*volume.Spec{}
		}
		volumeSpecList = append(volumeSpecList, volumeAttached.VolumeSpec)
		volumesPerPlugin[volumePlugin.GetPluginName()] = volumeSpecList
		volumeSpecMap[volumeAttached.VolumeSpec] = volumeAttached.VolumeName
	}

	return func() error {

		// For each volume plugin, pass the list of volume specs to VolumesAreAttached to check
		// whether the volumes are still attached.
		for pluginName, volumesSpecs := range volumesPerPlugin {
			attachableVolumePlugin, err :=
				og.volumePluginMgr.FindAttachablePluginByName(pluginName)
			if err != nil || attachableVolumePlugin == nil {
				glog.Errorf(
					"VolumeAreAttached.FindAttachablePluginBySpec failed for plugin %q with: %v",
					pluginName,
					err)
				continue
			}

			volumeAttacher, newAttacherErr := attachableVolumePlugin.NewAttacher()
			if newAttacherErr != nil {
				glog.Errorf(
					"VolumesAreAttached.NewAttacher failed for getting plugin %q with: %v",
					pluginName,
					newAttacherErr)
				continue
			}

			attached, areAttachedErr := volumeAttacher.VolumesAreAttached(volumesSpecs, nodeName)
			if areAttachedErr != nil {
				glog.Errorf(
					"VolumesAreAttached failed for checking on node %q with: %v",
					nodeName,
					areAttachedErr)
				continue
			}

			for spec, check := range attached {
				if !check {
					actualStateOfWorld.MarkVolumeAsDetached(volumeSpecMap[spec], nodeName)
					glog.V(1).Infof("VerifyVolumesAreAttached determined volume %q (spec.Name: %q) is no longer attached to node %q, therefore it was marked as detached.",
						volumeSpecMap[spec], spec.Name(), nodeName)
				}
			}
		}
		return nil
	}, nil
}

func (og *operationGenerator) GenerateBulkVolumeVerifyFunc(
	pluginNodeVolumes map[types.NodeName][]*volume.Spec,
	pluginName string,
	volumeSpecMap map[*volume.Spec]v1.UniqueVolumeName,
	actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, error) {

	return func() error {
		attachableVolumePlugin, err :=
			og.volumePluginMgr.FindAttachablePluginByName(pluginName)
		if err != nil || attachableVolumePlugin == nil {
			glog.Errorf(
				"BulkVerifyVolume.FindAttachablePluginBySpec failed for plugin %q with: %v",
				pluginName,
				err)
			return nil
		}

		volumeAttacher, newAttacherErr := attachableVolumePlugin.NewAttacher()

		if newAttacherErr != nil {
			glog.Errorf(
				"BulkVerifyVolume.NewAttacher failed for getting plugin %q with: %v",
				attachableVolumePlugin,
				newAttacherErr)
			return nil
		}
		bulkVolumeVerifier, ok := volumeAttacher.(volume.BulkVolumeVerifier)

		if !ok {
			glog.Errorf("BulkVerifyVolume failed to type assert attacher %q", bulkVolumeVerifier)
			return nil
		}

		attached, bulkAttachErr := bulkVolumeVerifier.BulkVerifyVolumes(pluginNodeVolumes)
		if bulkAttachErr != nil {
			glog.Errorf("BulkVerifyVolume.BulkVerifyVolumes Error checking volumes are attached with %v", bulkAttachErr)
			return nil
		}

		for nodeName, volumeSpecs := range pluginNodeVolumes {
			for _, volumeSpec := range volumeSpecs {
				nodeVolumeSpecs, nodeChecked := attached[nodeName]

				if !nodeChecked {
					glog.V(2).Infof("VerifyVolumesAreAttached.BulkVerifyVolumes failed for node %q and leaving volume %q as attached",
						nodeName,
						volumeSpec.Name())
					continue
				}

				check := nodeVolumeSpecs[volumeSpec]

				if !check {
					glog.V(2).Infof("VerifyVolumesAreAttached.BulkVerifyVolumes failed for node %q and volume %q",
						nodeName,
						volumeSpec.Name())
					actualStateOfWorld.MarkVolumeAsDetached(volumeSpecMap[volumeSpec], nodeName)
				}
			}
		}

		return nil
	}, nil
}

func (og *operationGenerator) GenerateAttachVolumeFunc(
	volumeToAttach VolumeToAttach,
	actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, string, error) {
	// Get attacher plugin
	attachableVolumePlugin, err :=
		og.volumePluginMgr.FindAttachablePluginBySpec(volumeToAttach.VolumeSpec)
	if err != nil || attachableVolumePlugin == nil {
		return nil, "", volumeToAttach.GenerateErrorDetailed("AttachVolume.FindAttachablePluginBySpec failed", err)
	}

	volumeAttacher, newAttacherErr := attachableVolumePlugin.NewAttacher()
	if newAttacherErr != nil {
		return nil, attachableVolumePlugin.GetPluginName(), volumeToAttach.GenerateErrorDetailed("AttachVolume.NewAttacher failed", newAttacherErr)
	}

	return func() error {
		// Execute attach
		devicePath, attachErr := volumeAttacher.Attach(
			volumeToAttach.VolumeSpec, volumeToAttach.NodeName)

		if attachErr != nil {
			// On failure, return error. Caller will log and retry.
			eventErr, detailedErr := volumeToAttach.GenerateError("AttachVolume.Attach failed", attachErr)
			for _, pod := range volumeToAttach.ScheduledPods {
				og.recorder.Eventf(pod, v1.EventTypeWarning, kevents.FailedMountVolume, eventErr.Error())
			}
			return detailedErr
		}

		glog.Infof(volumeToAttach.GenerateMsgDetailed("AttachVolume.Attach succeeded", ""))

		// Update actual state of world
		addVolumeNodeErr := actualStateOfWorld.MarkVolumeAsAttached(
			v1.UniqueVolumeName(""), volumeToAttach.VolumeSpec, volumeToAttach.NodeName, devicePath)
		if addVolumeNodeErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToAttach.GenerateErrorDetailed("AttachVolume.MarkVolumeAsAttached failed", addVolumeNodeErr)
		}

		return nil
	}, attachableVolumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) GetVolumePluginMgr() *volume.VolumePluginMgr {
	return og.volumePluginMgr
}

func (og *operationGenerator) GenerateDetachVolumeFunc(
	volumeToDetach AttachedVolume,
	verifySafeToDetach bool,
	actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, string, error) {
	var volumeName string
	var attachableVolumePlugin volume.AttachableVolumePlugin
	var pluginName string
	var err error

	if volumeToDetach.VolumeSpec != nil {
		// Get attacher plugin
		attachableVolumePlugin, err =
			og.volumePluginMgr.FindAttachablePluginBySpec(volumeToDetach.VolumeSpec)
		if err != nil || attachableVolumePlugin == nil {
			return nil, "", volumeToDetach.GenerateErrorDetailed("DetachVolume.FindAttachablePluginBySpec failed", err)
		}

		volumeName, err =
			attachableVolumePlugin.GetVolumeName(volumeToDetach.VolumeSpec)
		if err != nil {
			return nil, attachableVolumePlugin.GetPluginName(), volumeToDetach.GenerateErrorDetailed("DetachVolume.GetVolumeName failed", err)
		}
	} else {
		// Get attacher plugin and the volumeName by splitting the volume unique name in case
		// there's no VolumeSpec: this happens only on attach/detach controller crash recovery
		// when a pod has been deleted during the controller downtime
		pluginName, volumeName, err = volumehelper.SplitUniqueName(volumeToDetach.VolumeName)
		if err != nil {
			return nil, pluginName, volumeToDetach.GenerateErrorDetailed("DetachVolume.SplitUniqueName failed", err)
		}
		attachableVolumePlugin, err = og.volumePluginMgr.FindAttachablePluginByName(pluginName)
		if err != nil {
			return nil, pluginName, volumeToDetach.GenerateErrorDetailed("DetachVolume.FindAttachablePluginBySpec failed", err)
		}
	}

	if pluginName == "" {
		pluginName = attachableVolumePlugin.GetPluginName()
	}

	volumeDetacher, err := attachableVolumePlugin.NewDetacher()
	if err != nil {
		return nil, pluginName, volumeToDetach.GenerateErrorDetailed("DetachVolume.NewDetacher failed", err)
	}

	return func() error {
		var err error
		if verifySafeToDetach {
			err = og.verifyVolumeIsSafeToDetach(volumeToDetach)
		}
		if err == nil {
			err = volumeDetacher.Detach(volumeName, volumeToDetach.NodeName)
		}
		if err != nil {
			// On failure, add volume back to ReportAsAttached list
			actualStateOfWorld.AddVolumeToReportAsAttached(
				volumeToDetach.VolumeName, volumeToDetach.NodeName)
			return volumeToDetach.GenerateErrorDetailed("DetachVolume.Detach failed", err)
		}

		glog.Infof(volumeToDetach.GenerateMsgDetailed("DetachVolume.Detach succeeded", ""))

		// Update actual state of world
		actualStateOfWorld.MarkVolumeAsDetached(
			volumeToDetach.VolumeName, volumeToDetach.NodeName)

		return nil
	}, pluginName, nil
}

func (og *operationGenerator) GenerateMountVolumeFunc(
	waitForAttachTimeout time.Duration,
	volumeToMount VolumeToMount,
	actualStateOfWorld ActualStateOfWorldMounterUpdater,
	isRemount bool) (func() error, string, error) {
	// Get mounter plugin
	volumePlugin, err :=
		og.volumePluginMgr.FindPluginBySpec(volumeToMount.VolumeSpec)
	if err != nil || volumePlugin == nil {
		return nil, "", volumeToMount.GenerateErrorDetailed("MountVolume.FindPluginBySpec failed", err)
	}

	affinityErr := checkNodeAffinity(og, volumeToMount, volumePlugin)
	if affinityErr != nil {
		return nil, volumePlugin.GetPluginName(), affinityErr
	}

	volumeMounter, newMounterErr := volumePlugin.NewMounter(
		volumeToMount.VolumeSpec,
		volumeToMount.Pod,
		volume.VolumeOptions{})
	if newMounterErr != nil {
		eventErr, detailedErr := volumeToMount.GenerateError("MountVolume.NewMounter initialization failed", newMounterErr)
		og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMountVolume, eventErr.Error())
		return nil, volumePlugin.GetPluginName(), detailedErr
	}

	mountCheckError := checkMountOptionSupport(og, volumeToMount, volumePlugin)

	if mountCheckError != nil {
		return nil, volumePlugin.GetPluginName(), mountCheckError
	}

	// Get attacher, if possible
	attachableVolumePlugin, _ :=
		og.volumePluginMgr.FindAttachablePluginBySpec(volumeToMount.VolumeSpec)
	var volumeAttacher volume.Attacher
	if attachableVolumePlugin != nil {
		volumeAttacher, _ = attachableVolumePlugin.NewAttacher()
	}

	var fsGroup *int64
	if volumeToMount.Pod.Spec.SecurityContext != nil &&
		volumeToMount.Pod.Spec.SecurityContext.FSGroup != nil {
		fsGroup = volumeToMount.Pod.Spec.SecurityContext.FSGroup
	}

	return func() error {
		if volumeAttacher != nil {
			// Wait for attachable volumes to finish attaching
			glog.Infof(volumeToMount.GenerateMsgDetailed("MountVolume.WaitForAttach entering", fmt.Sprintf("DevicePath %q", volumeToMount.DevicePath)))

			devicePath, err := volumeAttacher.WaitForAttach(
				volumeToMount.VolumeSpec, volumeToMount.DevicePath, volumeToMount.Pod, waitForAttachTimeout)
			if err != nil {
				// On failure, return error. Caller will log and retry.
				return volumeToMount.GenerateErrorDetailed("MountVolume.WaitForAttach failed", err)
			}

			glog.Infof(volumeToMount.GenerateMsgDetailed("MountVolume.WaitForAttach succeeded", ""))

			deviceMountPath, err :=
				volumeAttacher.GetDeviceMountPath(volumeToMount.VolumeSpec)
			if err != nil {
				// On failure, return error. Caller will log and retry.
				return volumeToMount.GenerateErrorDetailed("MountVolume.GetDeviceMountPath failed", err)
			}

			// Mount device to global mount path
			err = volumeAttacher.MountDevice(
				volumeToMount.VolumeSpec,
				devicePath,
				deviceMountPath)
			if err != nil {
				// On failure, return error. Caller will log and retry.
				eventErr, detailedErr := volumeToMount.GenerateError("MountVolume.MountDevice failed", err)
				og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMountVolume, eventErr.Error())
				return detailedErr
			}

			glog.Infof(volumeToMount.GenerateMsgDetailed("MountVolume.MountDevice succeeded", fmt.Sprintf("device mount path %q", deviceMountPath)))

			// Update actual state of world to reflect volume is globally mounted
			markDeviceMountedErr := actualStateOfWorld.MarkDeviceAsMounted(
				volumeToMount.VolumeName)
			if markDeviceMountedErr != nil {
				// On failure, return error. Caller will log and retry.
				return volumeToMount.GenerateErrorDetailed("MountVolume.MarkDeviceAsMounted failed", markDeviceMountedErr)
			}
		}

		if og.checkNodeCapabilitiesBeforeMount {
			if canMountErr := volumeMounter.CanMount(); canMountErr != nil {
				err = fmt.Errorf(
					"Verify that your node machine has the required components before attempting to mount this volume type. %s",
					canMountErr)
				eventErr, detailedErr := volumeToMount.GenerateError("MountVolume.CanMount failed", err)
				og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMountVolume, eventErr.Error())
				return detailedErr
			}
		}

		// Execute mount
		mountErr := volumeMounter.SetUp(fsGroup)
		if mountErr != nil {
			// On failure, return error. Caller will log and retry.
			eventErr, detailedErr := volumeToMount.GenerateError("MountVolume.SetUp failed", mountErr)
			og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMountVolume, eventErr.Error())
			return detailedErr
		}

		simpleMsg, detailedMsg := volumeToMount.GenerateMsg("MountVolume.SetUp succeeded", "")
		verbosity := glog.Level(1)
		if isRemount {
			verbosity = glog.Level(7)
		} else {
			og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeNormal, kevents.SuccessfulMountVolume, simpleMsg)
		}
		glog.V(verbosity).Infof(detailedMsg)

		// Update actual state of world
		markVolMountedErr := actualStateOfWorld.MarkVolumeAsMounted(
			volumeToMount.PodName,
			volumeToMount.Pod.UID,
			volumeToMount.VolumeName,
			volumeMounter,
			nil,
			volumeToMount.OuterVolumeSpecName,
			volumeToMount.VolumeGidValue)
		if markVolMountedErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToMount.GenerateErrorDetailed("MountVolume.MarkVolumeAsMounted failed", markVolMountedErr)
		}

		return nil
	}, volumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) GenerateUnmountVolumeFunc(
	volumeToUnmount MountedVolume,
	actualStateOfWorld ActualStateOfWorldMounterUpdater) (func() error, string, error) {
	// Get mountable plugin
	volumePlugin, err :=
		og.volumePluginMgr.FindPluginByName(volumeToUnmount.PluginName)
	if err != nil || volumePlugin == nil {
		return nil, "", volumeToUnmount.GenerateErrorDetailed("UnmountVolume.FindPluginByName failed", err)
	}

	volumeUnmounter, newUnmounterErr := volumePlugin.NewUnmounter(
		volumeToUnmount.InnerVolumeSpecName, volumeToUnmount.PodUID)
	if newUnmounterErr != nil {
		return nil, volumePlugin.GetPluginName(), volumeToUnmount.GenerateErrorDetailed("UnmountVolume.NewUnmounter failed", newUnmounterErr)
	}

	return func() error {
		// Execute unmount
		unmountErr := volumeUnmounter.TearDown()
		if unmountErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToUnmount.GenerateErrorDetailed("UnmountVolume.TearDown failed", unmountErr)
		}

		glog.Infof(
			"UnmountVolume.TearDown succeeded for volume %q (OuterVolumeSpecName: %q) pod %q (UID: %q). InnerVolumeSpecName %q. PluginName %q, VolumeGidValue %q",
			volumeToUnmount.VolumeName,
			volumeToUnmount.OuterVolumeSpecName,
			volumeToUnmount.PodName,
			volumeToUnmount.PodUID,
			volumeToUnmount.InnerVolumeSpecName,
			volumeToUnmount.PluginName,
			volumeToUnmount.VolumeGidValue)

		// Update actual state of world
		markVolMountedErr := actualStateOfWorld.MarkVolumeAsUnmounted(
			volumeToUnmount.PodName, volumeToUnmount.VolumeName)
		if markVolMountedErr != nil {
			// On failure, just log and exit
			glog.Errorf(volumeToUnmount.GenerateErrorDetailed("UnmountVolume.MarkVolumeAsUnmounted failed", markVolMountedErr).Error())
		}

		return nil
	}, volumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) GenerateUnmountDeviceFunc(
	deviceToDetach AttachedVolume,
	actualStateOfWorld ActualStateOfWorldMounterUpdater,
	mounter mount.Interface) (func() error, string, error) {
	// Get attacher plugin
	attachableVolumePlugin, err :=
		og.volumePluginMgr.FindAttachablePluginBySpec(deviceToDetach.VolumeSpec)
	if err != nil || attachableVolumePlugin == nil {
		return nil, "", deviceToDetach.GenerateErrorDetailed("UnmountDevice.FindAttachablePluginBySpec failed", err)
	}

	volumeDetacher, err := attachableVolumePlugin.NewDetacher()
	if err != nil {
		return nil, attachableVolumePlugin.GetPluginName(), deviceToDetach.GenerateErrorDetailed("UnmountDevice.NewDetacher failed", err)
	}

	volumeAttacher, err := attachableVolumePlugin.NewAttacher()
	if err != nil {
		return nil, attachableVolumePlugin.GetPluginName(), deviceToDetach.GenerateErrorDetailed("UnmountDevice.NewAttacher failed", err)
	}

	return func() error {
		deviceMountPath, err :=
			volumeAttacher.GetDeviceMountPath(deviceToDetach.VolumeSpec)
		if err != nil {
			// On failure, return error. Caller will log and retry.
			return deviceToDetach.GenerateErrorDetailed("GetDeviceMountPath failed", err)
		}
		refs, err := attachableVolumePlugin.GetDeviceMountRefs(deviceMountPath)

		if err != nil || hasMountRefs(deviceMountPath, refs) {
			if err == nil {
				err = fmt.Errorf("The device mount path %q is still mounted by other references %v", deviceMountPath, refs)
			}
			return deviceToDetach.GenerateErrorDetailed("GetDeviceMountRefs check failed", err)
		}
		// Execute unmount
		unmountDeviceErr := volumeDetacher.UnmountDevice(deviceMountPath)
		if unmountDeviceErr != nil {
			// On failure, return error. Caller will log and retry.
			return deviceToDetach.GenerateErrorDetailed("UnmountDevice failed", unmountDeviceErr)
		}
		// Before logging that UnmountDevice succeeded and moving on,
		// use mounter.PathIsDevice to check if the path is a device,
		// if so use mounter.DeviceOpened to check if the device is in use anywhere
		// else on the system. Retry if it returns true.
		deviceOpened, deviceOpenedErr := isDeviceOpened(deviceToDetach, mounter)
		if deviceOpenedErr != nil {
			return deviceOpenedErr
		}
		// The device is still in use elsewhere. Caller will log and retry.
		if deviceOpened {
			return deviceToDetach.GenerateErrorDetailed(
				"UnmountDevice failed",
				fmt.Errorf("the device is in use when it was no longer expected to be in use"))
		}

		glog.Infof(deviceToDetach.GenerateMsgDetailed("UnmountDevice succeeded", ""))

		// Update actual state of world
		markDeviceUnmountedErr := actualStateOfWorld.MarkDeviceAsUnmounted(
			deviceToDetach.VolumeName)
		if markDeviceUnmountedErr != nil {
			// On failure, return error. Caller will log and retry.
			return deviceToDetach.GenerateErrorDetailed("MarkDeviceAsUnmounted failed", markDeviceUnmountedErr)
		}

		return nil
	}, attachableVolumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) GenerateMapVolumeFunc(
	waitForAttachTimeout time.Duration,
	volumeToMount VolumeToMount,
	actualStateOfWorld ActualStateOfWorldMounterUpdater) (func() error, string, error) {

	// Get block volume mapper plugin
	var blockVolumeMapper volume.BlockVolumeMapper
	blockVolumePlugin, err :=
		og.volumePluginMgr.FindMapperPluginBySpec(volumeToMount.VolumeSpec)
	if err != nil {
		return nil, "", volumeToMount.GenerateErrorDetailed("MapVolume.FindMapperPluginBySpec failed", err)
	}
	if blockVolumePlugin == nil {
		return nil, "", volumeToMount.GenerateErrorDetailed("MapVolume.FindMapperPluginBySpec failed to find BlockVolumeMapper plugin. Volume plugin is nil.", nil)
	}
	affinityErr := checkNodeAffinity(og, volumeToMount, blockVolumePlugin)
	if affinityErr != nil {
		return nil, blockVolumePlugin.GetPluginName(), affinityErr
	}
	blockVolumeMapper, newMapperErr := blockVolumePlugin.NewBlockVolumeMapper(
		volumeToMount.VolumeSpec,
		volumeToMount.Pod,
		volume.VolumeOptions{})
	if newMapperErr != nil {
		eventErr, detailedErr := volumeToMount.GenerateError("MapVolume.NewBlockVolumeMapper initialization failed", newMapperErr)
		og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMapVolume, eventErr.Error())
		return nil, blockVolumePlugin.GetPluginName(), detailedErr
	}

	// Get attacher, if possible
	attachableVolumePlugin, _ :=
		og.volumePluginMgr.FindAttachablePluginBySpec(volumeToMount.VolumeSpec)
	var volumeAttacher volume.Attacher
	if attachableVolumePlugin != nil {
		volumeAttacher, _ = attachableVolumePlugin.NewAttacher()
	}

	return func() error {
		var devicePath string
		if volumeAttacher != nil {
			// Wait for attachable volumes to finish attaching
			glog.Infof(volumeToMount.GenerateMsgDetailed("MapVolume.WaitForAttach entering", fmt.Sprintf("DevicePath %q", volumeToMount.DevicePath)))

			devicePath, err = volumeAttacher.WaitForAttach(
				volumeToMount.VolumeSpec, volumeToMount.DevicePath, volumeToMount.Pod, waitForAttachTimeout)
			if err != nil {
				// On failure, return error. Caller will log and retry.
				return volumeToMount.GenerateErrorDetailed("MapVolume.WaitForAttach failed", err)
			}

			glog.Infof(volumeToMount.GenerateMsgDetailed("MapVolume.WaitForAttach succeeded", ""))

			// Update actual state of world to reflect volume is globally mounted
			markDeviceMappedErr := actualStateOfWorld.MarkDeviceAsMounted(
				volumeToMount.VolumeName)
			if markDeviceMappedErr != nil {
				// On failure, return error. Caller will log and retry.
				return volumeToMount.GenerateErrorDetailed("MapVolume.MarkDeviceAsMounted failed", markDeviceMappedErr)
			}
		}
		// A plugin doesn't have attacher also needs to map device to global map path with SetUpDevice()
		pluginDevicePath, mapErr := blockVolumeMapper.SetUpDevice()
		if mapErr != nil {
			// On failure, return error. Caller will log and retry.
			eventErr, detailedErr := volumeToMount.GenerateError("MapVolume.SetUp failed", mapErr)
			og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMapVolume, eventErr.Error())
			return detailedErr
		}
		// Update devicePath for none attachable plugin case
		if len(devicePath) == 0 {
			if len(pluginDevicePath) != 0 {
				devicePath = pluginDevicePath
			} else {
				return volumeToMount.GenerateErrorDetailed("MapVolume failed", fmt.Errorf("Device path of the volume is empty"))
			}
		}
		// Set up global map path under the given plugin directory using symbolic link
		globalMapPath, err :=
			blockVolumeMapper.GetGlobalMapPath(volumeToMount.VolumeSpec)
		if err != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToMount.GenerateErrorDetailed("MapVolume.GetDeviceMountPath failed", err)
		}
		mapErr = util.MapDevice(devicePath, globalMapPath, string(volumeToMount.Pod.UID))
		if mapErr != nil {
			// On failure, return error. Caller will log and retry.
			eventErr, detailedErr := volumeToMount.GenerateError("MapVolume.MapDevice failed", mapErr)
			og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMapVolume, eventErr.Error())
			return detailedErr
		}
		// Device mapping for global map path succeeded
		simpleMsg, detailedMsg := volumeToMount.GenerateMsg("MapVolume.MapDevice succeeded", fmt.Sprintf("globalMapPath %q", globalMapPath))
		verbosity := glog.Level(4)
		og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeNormal, kevents.SuccessfulMountVolume, simpleMsg)
		glog.V(verbosity).Infof(detailedMsg)

		// Map device to pod device map path under the given pod directory using symbolic link
		volumeMapPath, volName := blockVolumeMapper.GetPodDeviceMapPath()
		mapErr = util.MapDevice(devicePath, volumeMapPath, volName)
		if mapErr != nil {
			// On failure, return error. Caller will log and retry.
			eventErr, detailedErr := volumeToMount.GenerateError("MapVolume.MapDevice failed", mapErr)
			og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMapVolume, eventErr.Error())
			return detailedErr
		}
		// Device mapping for pod device map path succeeded
		simpleMsg, detailedMsg = volumeToMount.GenerateMsg("MapVolume.MapDevice succeeded", fmt.Sprintf("volumeMapPath %q", volumeMapPath))
		verbosity = glog.Level(1)
		og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeNormal, kevents.SuccessfulMountVolume, simpleMsg)
		glog.V(verbosity).Infof(detailedMsg)

		// Update actual state of world
		markVolMountedErr := actualStateOfWorld.MarkVolumeAsMounted(
			volumeToMount.PodName,
			volumeToMount.Pod.UID,
			volumeToMount.VolumeName,
			nil,
			blockVolumeMapper,
			volumeToMount.OuterVolumeSpecName,
			volumeToMount.VolumeGidValue)
		if markVolMountedErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToMount.GenerateErrorDetailed("MapVolume.MarkVolumeAsMounted failed", markVolMountedErr)
		}

		return nil
	}, blockVolumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) GenerateUnmapVolumeFunc(
	volumeToUnmount MountedVolume,
	actualStateOfWorld ActualStateOfWorldMounterUpdater) (func() error, string, error) {

	// Get block volume unmapper plugin
	var blockVolumeUnmapper volume.BlockVolumeUnmapper
	blockVolumePlugin, err :=
		og.volumePluginMgr.FindMapperPluginByName(volumeToUnmount.PluginName)
	if err != nil {
		return nil, "", volumeToUnmount.GenerateErrorDetailed("UnmapVolume.FindMapperPluginByName failed", err)
	}
	if blockVolumePlugin == nil {
		return nil, "", volumeToUnmount.GenerateErrorDetailed("UnmapVolume.FindMapperPluginByName failed to find BlockVolumeMapper plugin. Volume plugin is nil.", nil)
	}
	blockVolumeUnmapper, newUnmapperErr := blockVolumePlugin.NewBlockVolumeUnmapper(
		volumeToUnmount.InnerVolumeSpecName, volumeToUnmount.PodUID)
	if newUnmapperErr != nil {
		return nil, blockVolumePlugin.GetPluginName(), volumeToUnmount.GenerateErrorDetailed("UnmapVolume.NewUnmapper failed", newUnmapperErr)
	}

	return func() error {
		// Execute tear down device
		unmapErr := blockVolumeUnmapper.TearDownDevice()
		if unmapErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToUnmount.GenerateErrorDetailed("UnmapVolume.TearDownDevice failed", unmapErr)
		}

		// Try to unmap symlink on global map path
		globalUnmapPath, err :=
			blockVolumeUnmapper.GetGlobalUnmapPath(volumeToUnmount.VolumeSpec)
		if err != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToUnmount.GenerateErrorDetailed("GetGlobalUnmapPath failed", err)
		}
		unmapDeviceErr := util.UnmapDevice(globalUnmapPath, string(volumeToUnmount.PodUID))
		if unmapDeviceErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToUnmount.GenerateErrorDetailed("UnmapVolume.UnmapDevice on global map path failed", unmapDeviceErr)
		}

		// Try to unmap symlink on pod device map path
		podDeviceUnmapPath, volName := blockVolumeUnmapper.GetPodDeviceUnmapPath()
		unmapDeviceErr = util.UnmapDevice(podDeviceUnmapPath, volName)
		if unmapDeviceErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToUnmount.GenerateErrorDetailed("UnmapVolume.UnmapDevice on pod device map path failed", unmapDeviceErr)
		}
		removeLinkErr := util.RemoveMapPath(podDeviceUnmapPath + "/" + volName)
		if removeLinkErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToUnmount.GenerateErrorDetailed("UnmapVolume.RemoveSymlink failed", removeLinkErr)
		}

		glog.Infof(
			"UnmapVolume.TearDown succeeded for volume %q (OuterVolumeSpecName: %q) pod %q (UID: %q). InnerVolumeSpecName %q. PluginName %q, VolumeGidValue %q",
			volumeToUnmount.VolumeName,
			volumeToUnmount.OuterVolumeSpecName,
			volumeToUnmount.PodName,
			volumeToUnmount.PodUID,
			volumeToUnmount.InnerVolumeSpecName,
			volumeToUnmount.PluginName,
			volumeToUnmount.VolumeGidValue)

		// Update actual state of world
		markVolUnmountedErr := actualStateOfWorld.MarkVolumeAsUnmounted(
			volumeToUnmount.PodName, volumeToUnmount.VolumeName)
		if markVolUnmountedErr != nil {
			// On failure, just log and exit
			glog.Errorf(volumeToUnmount.GenerateErrorDetailed("UnmapVolume.MarkVolumeAsUnmounted failed", markVolUnmountedErr).Error())
		}

		return nil
	}, blockVolumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) GenerateUnmapDeviceFunc(
	deviceToDetach AttachedVolume,
	actualStateOfWorld ActualStateOfWorldMounterUpdater,
	mounter mount.Interface) (func() error, string, error) {

	// Get block volume mapper plugin
	var blockVolumeMapper volume.BlockVolumeMapper
	blockVolumePlugin, err :=
		og.volumePluginMgr.FindMapperPluginBySpec(deviceToDetach.VolumeSpec)
	if err != nil {
		return nil, "", deviceToDetach.GenerateErrorDetailed("MapVolume.FindMapperPluginBySpec failed", err)
	}
	if blockVolumePlugin == nil {
		return nil, "", deviceToDetach.GenerateErrorDetailed("MapVolume.FindMapperPluginBySpec failed to find BlockVolumeMapper plugin. Volume plugin is nil.", nil)
	}
	blockVolumeMapper, newMapperErr := blockVolumePlugin.NewBlockVolumeMapper(
		deviceToDetach.VolumeSpec,
		nil, /* Pod */
		volume.VolumeOptions{})
	if newMapperErr != nil {
		return nil, "", deviceToDetach.GenerateErrorDetailed("UnmapDevice.NewBlockVolumeMapper initialization failed", newMapperErr)
	}

	return func() error {
		globalMapPath, err :=
			blockVolumeMapper.GetGlobalMapPath(deviceToDetach.VolumeSpec)
		if err != nil {
			// On failure, return error. Caller will log and retry.
			return deviceToDetach.GenerateErrorDetailed("GetGlobalMapPath failed", err)
		}
		refs, err := util.GetDeviceSymlinkRefs(deviceToDetach.DevicePath, globalMapPath)
		if err != nil {
			return deviceToDetach.GenerateErrorDetailed("GetDeviceSymlinkRefs check failed", err)
		}
		if len(refs) > 0 {
			err = fmt.Errorf("The device %q is still referenced from other Pods %v", globalMapPath, refs)
			return deviceToDetach.GenerateErrorDetailed("UnmapDevice failed", err)
		}
		// The globalMapPath directory is empty. Remove the directory
		removeMapPathErr := util.RemoveMapPath(globalMapPath)
		if removeMapPathErr != nil {
			// On failure, return error. Caller will log and retry.
			return deviceToDetach.GenerateErrorDetailed("UnmapDevice failed", removeMapPathErr)
		}
		// Before logging that UnmapDevice succeeded and moving on,
		// use mounter.PathIsDevice to check if the path is a device,
		// if so use mounter.DeviceOpened to check if the device is in use anywhere
		// else on the system. Retry if it returns true.
		deviceOpened, deviceOpenedErr := isDeviceOpened(deviceToDetach, mounter)
		if deviceOpenedErr != nil {
			return deviceOpenedErr
		}
		// The device is still in use elsewhere. Caller will log and retry.
		if deviceOpened {
			return deviceToDetach.GenerateErrorDetailed(
				"UnmapDevice failed",
				fmt.Errorf("the device is in use when it was no longer expected to be in use"))
		}

		glog.Infof(deviceToDetach.GenerateMsgDetailed("UnmapDevice succeeded", ""))

		// Update actual state of world
		markDeviceUnmountedErr := actualStateOfWorld.MarkDeviceAsUnmounted(
			deviceToDetach.VolumeName)
		if markDeviceUnmountedErr != nil {
			// On failure, return error. Caller will log and retry.
			return deviceToDetach.GenerateErrorDetailed("MarkDeviceAsUnmounted failed", markDeviceUnmountedErr)
		}

		return nil
	}, blockVolumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) GenerateVerifyControllerAttachedVolumeFunc(
	volumeToMount VolumeToMount,
	nodeName types.NodeName,
	actualStateOfWorld ActualStateOfWorldAttacherUpdater) (func() error, string, error) {
	volumePlugin, err :=
		og.volumePluginMgr.FindPluginBySpec(volumeToMount.VolumeSpec)
	if err != nil || volumePlugin == nil {
		return nil, "", volumeToMount.GenerateErrorDetailed("VerifyControllerAttachedVolume.FindPluginBySpec failed", err)
	}

	return func() error {
		if !volumeToMount.PluginIsAttachable {
			// If the volume does not implement the attacher interface, it is
			// assumed to be attached and the actual state of the world is
			// updated accordingly.

			addVolumeNodeErr := actualStateOfWorld.MarkVolumeAsAttached(
				volumeToMount.VolumeName, volumeToMount.VolumeSpec, nodeName, "" /* devicePath */)
			if addVolumeNodeErr != nil {
				// On failure, return error. Caller will log and retry.
				return volumeToMount.GenerateErrorDetailed("VerifyControllerAttachedVolume.MarkVolumeAsAttachedByUniqueVolumeName failed", addVolumeNodeErr)
			}

			return nil
		}

		if !volumeToMount.ReportedInUse {
			// If the given volume has not yet been added to the list of
			// VolumesInUse in the node's volume status, do not proceed, return
			// error. Caller will log and retry. The node status is updated
			// periodically by kubelet, so it may take as much as 10 seconds
			// before this clears.
			// Issue #28141 to enable on demand status updates.
			return volumeToMount.GenerateErrorDetailed("Volume has not been added to the list of VolumesInUse in the node's volume status", nil)
		}

		// Fetch current node object
		node, fetchErr := og.kubeClient.Core().Nodes().Get(string(nodeName), metav1.GetOptions{})
		if fetchErr != nil {
			// On failure, return error. Caller will log and retry.
			return volumeToMount.GenerateErrorDetailed("VerifyControllerAttachedVolume failed fetching node from API server", fetchErr)
		}

		if node == nil {
			// On failure, return error. Caller will log and retry.
			return volumeToMount.GenerateErrorDetailed(
				"VerifyControllerAttachedVolume failed",
				fmt.Errorf("Node object retrieved from API server is nil"))
		}

		for _, attachedVolume := range node.Status.VolumesAttached {
			if attachedVolume.Name == volumeToMount.VolumeName {
				addVolumeNodeErr := actualStateOfWorld.MarkVolumeAsAttached(
					v1.UniqueVolumeName(""), volumeToMount.VolumeSpec, nodeName, attachedVolume.DevicePath)
				glog.Infof(volumeToMount.GenerateMsgDetailed("Controller attach succeeded", fmt.Sprintf("device path: %q", attachedVolume.DevicePath)))
				if addVolumeNodeErr != nil {
					// On failure, return error. Caller will log and retry.
					return volumeToMount.GenerateErrorDetailed("VerifyControllerAttachedVolume.MarkVolumeAsAttached failed", addVolumeNodeErr)
				}
				return nil
			}
		}

		// Volume not attached, return error. Caller will log and retry.
		return volumeToMount.GenerateErrorDetailed("Volume not attached according to node status", nil)
	}, volumePlugin.GetPluginName(), nil
}

func (og *operationGenerator) verifyVolumeIsSafeToDetach(
	volumeToDetach AttachedVolume) error {
	// Fetch current node object
	node, fetchErr := og.kubeClient.Core().Nodes().Get(string(volumeToDetach.NodeName), metav1.GetOptions{})
	if fetchErr != nil {
		if errors.IsNotFound(fetchErr) {
			glog.Warningf(volumeToDetach.GenerateMsgDetailed("Node not found on API server. DetachVolume will skip safe to detach check", ""))
			return nil
		}

		// On failure, return error. Caller will log and retry.
		return volumeToDetach.GenerateErrorDetailed("DetachVolume failed fetching node from API server", fetchErr)
	}

	if node == nil {
		// On failure, return error. Caller will log and retry.
		return volumeToDetach.GenerateErrorDetailed(
			"DetachVolume failed fetching node from API server",
			fmt.Errorf("node object retrieved from API server is nil"))
	}

	for _, inUseVolume := range node.Status.VolumesInUse {
		if inUseVolume == volumeToDetach.VolumeName {
			return volumeToDetach.GenerateErrorDetailed(
				"DetachVolume failed",
				fmt.Errorf("volume is still in use by node, according to Node status"))
		}
	}

	// Volume is not marked as in use by node
	glog.Infof(volumeToDetach.GenerateMsgDetailed("Verified volume is safe to detach", ""))
	return nil
}

func (og *operationGenerator) GenerateExpandVolumeFunc(
	pvcWithResizeRequest *expandcache.PVCWithResizeRequest,
	resizeMap expandcache.VolumeResizeMap) (func() error, string, error) {

	volumeSpec := volume.NewSpecFromPersistentVolume(pvcWithResizeRequest.PersistentVolume, false)

	volumePlugin, err := og.volumePluginMgr.FindExpandablePluginBySpec(volumeSpec)

	if err != nil {
		return nil, "", fmt.Errorf("Error finding plugin for expanding volume: %q with error %v", pvcWithResizeRequest.QualifiedName(), err)
	}

	expandFunc := func() error {
		newSize := pvcWithResizeRequest.ExpectedSize
		pvSize := pvcWithResizeRequest.PersistentVolume.Spec.Capacity[v1.ResourceStorage]
		if pvSize.Cmp(newSize) < 0 {
			updatedSize, expandErr := volumePlugin.ExpandVolumeDevice(
				volumeSpec,
				pvcWithResizeRequest.ExpectedSize,
				pvcWithResizeRequest.CurrentSize)

			if expandErr != nil {
				glog.Errorf("Error expanding volume %q of plugin %s : %v", pvcWithResizeRequest.QualifiedName(), volumePlugin.GetPluginName(), expandErr)
				og.recorder.Eventf(pvcWithResizeRequest.PVC, v1.EventTypeWarning, kevents.VolumeResizeFailed, expandErr.Error())
				return expandErr
			}
			newSize = updatedSize
			// k8s doesn't have transactions, we can't guarantee that after updating PV - updating PVC will be
			// successful, that is why all PVCs for which pvc.Spec.Size > pvc.Status.Size must be reprocessed
			// until they reflect user requested size in pvc.Status.Size
			updateErr := resizeMap.UpdatePVSize(pvcWithResizeRequest, newSize)

			if updateErr != nil {
				glog.V(4).Infof("Error updating PV spec capacity for volume %q with : %v", pvcWithResizeRequest.QualifiedName(), updateErr)
				og.recorder.Eventf(pvcWithResizeRequest.PVC, v1.EventTypeWarning, kevents.VolumeResizeFailed, updateErr.Error())
				return updateErr
			}
		}

		// No Cloudprovider resize needed, lets mark resizing as done
		// Rest of the volume expand controller code will assume PVC as *not* resized until pvc.Status.Size
		// reflects user requested size.
		if !volumePlugin.RequiresFSResize() {
			glog.V(4).Infof("Controller resizing done for PVC %s", pvcWithResizeRequest.QualifiedName())
			err := resizeMap.MarkAsResized(pvcWithResizeRequest, newSize)

			if err != nil {
				glog.Errorf("Error marking pvc %s as resized : %v", pvcWithResizeRequest.QualifiedName(), err)
				og.recorder.Eventf(pvcWithResizeRequest.PVC, v1.EventTypeWarning, kevents.VolumeResizeFailed, err.Error())
				return err
			}
		}
		return nil

	}
	return expandFunc, volumePlugin.GetPluginName(), nil
}

func checkMountOptionSupport(og *operationGenerator, volumeToMount VolumeToMount, plugin volume.VolumePlugin) error {
	mountOptions := volume.MountOptionFromSpec(volumeToMount.VolumeSpec)

	if len(mountOptions) > 0 && !plugin.SupportsMountOption() {
		eventErr, detailedErr := volumeToMount.GenerateError("Mount options are not supported for this volume type", nil)
		og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.UnsupportedMountOption, eventErr.Error())
		return detailedErr
	}
	return nil
}

// checkNodeAffinity looks at the PV node affinity, and checks if the node has the same corresponding labels
// This ensures that we don't mount a volume that doesn't belong to this node
func checkNodeAffinity(og *operationGenerator, volumeToMount VolumeToMount, plugin volume.VolumePlugin) error {
	if !utilfeature.DefaultFeatureGate.Enabled(features.PersistentLocalVolumes) {
		return nil
	}

	pv := volumeToMount.VolumeSpec.PersistentVolume
	if pv != nil {
		nodeLabels, err := og.volumePluginMgr.Host.GetNodeLabels()
		if err != nil {
			return volumeToMount.GenerateErrorDetailed("Error getting node labels", err)
		}

		err = util.CheckNodeAffinity(pv, nodeLabels)
		if err != nil {
			eventErr, detailedErr := volumeToMount.GenerateError("Storage node affinity check failed", err)
			og.recorder.Eventf(volumeToMount.Pod, v1.EventTypeWarning, kevents.FailedMountVolume, eventErr.Error())
			return detailedErr
		}
	}
	return nil
}

// isDeviceOpened checks the device status if the device is in use anywhere else on the system
func isDeviceOpened(deviceToDetach AttachedVolume, mounter mount.Interface) (bool, error) {
	isDevicePath, devicePathErr := mounter.PathIsDevice(deviceToDetach.DevicePath)
	var deviceOpened bool
	var deviceOpenedErr error
	if !isDevicePath && devicePathErr == nil {
		// not a device path or path doesn't exist
		//TODO: refer to #36092
		glog.V(3).Infof("Not checking device path %s", deviceToDetach.DevicePath)
		deviceOpened = false
	} else {
		deviceOpened, deviceOpenedErr = mounter.DeviceOpened(deviceToDetach.DevicePath)
		if deviceOpenedErr != nil {
			return false, deviceToDetach.GenerateErrorDetailed("UnmountDevice.DeviceOpened failed", deviceOpenedErr)
		}
	}
	return deviceOpened, nil
}
