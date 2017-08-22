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

package cm

import (
	"fmt"
	"sync"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
	"k8s.io/kubernetes/pkg/kubelet/deviceplugin"
)

// podDevices represents a list of pod to Device mappings.
type containerToDevice map[string]sets.String
type podDevices struct {
	podDeviceMapping map[string]containerToDevice
}

func newPodDevices() *podDevices {
	return &podDevices{
		podDeviceMapping: make(map[string]containerToDevice),
	}
}
func (pdev *podDevices) pods() sets.String {
	ret := sets.NewString()
	for k := range pdev.podDeviceMapping {
		ret.Insert(k)
	}
	return ret
}

func (pdev *podDevices) insert(podUID, contName string, device string) {
	if _, exists := pdev.podDeviceMapping[podUID]; !exists {
		pdev.podDeviceMapping[podUID] = make(containerToDevice)
	}
	if _, exists := pdev.podDeviceMapping[podUID][contName]; !exists {
		pdev.podDeviceMapping[podUID][contName] = sets.NewString()
	}
	pdev.podDeviceMapping[podUID][contName].Insert(device)
}

func (pdev *podDevices) getDevices(podUID, contName string) sets.String {
	containers, exists := pdev.podDeviceMapping[podUID]
	if !exists {
		return nil
	}
	devices, exists := containers[contName]
	if !exists {
		return nil
	}
	return devices
}

func (pdev *podDevices) delete(pods []string) {
	for _, uid := range pods {
		delete(pdev.podDeviceMapping, uid)
	}
}

func (pdev *podDevices) devices() sets.String {
	ret := sets.NewString()
	for _, containerToDevice := range pdev.podDeviceMapping {
		for _, deviceSet := range containerToDevice {
			ret = ret.Union(deviceSet)
		}
	}
	return ret
}

type DevicePluginHandler interface {
	Start() error
	Devices() map[string][]*pluginapi.Device
	Allocate(pod *v1.Pod, container *v1.Container, activePods []*v1.Pod) ([]*pluginapi.AllocateResponse, error)
}

// updateCapacityCallback is called to update ContainerManager capacity when
// device capacity changes.
type updateCapacityCallback func(v1.ResourceList)

type DevicePluginHandlerImpl struct {
	sync.Mutex
	devicePluginManager deviceplugin.Manager
	// devicePluginManagerMonitorCallback is used for testing only.
	devicePluginManagerMonitorCallback deviceplugin.MonitorCallback
	// allDevices and allocated are keyed by resource_name.
	allDevices         map[string]sets.String
	allocatedDevices   map[string]*podDevices
	updateCapacityFunc updateCapacityCallback
}

// NewDevicePluginHandler create a DevicePluginHandler
func NewDevicePluginHandlerImpl(updateCapacityFunc updateCapacityCallback) (*DevicePluginHandlerImpl, error) {
	glog.V(2).Infof("Starting Device Plugin Handler")
	handler := &DevicePluginHandlerImpl{
		allDevices:         make(map[string]sets.String),
		allocatedDevices:   devicesInUse(),
		updateCapacityFunc: updateCapacityFunc,
	}

	deviceManagerMonitorCallback := func(resourceName string, added, updated, deleted []*pluginapi.Device) {
		var capacity = v1.ResourceList{}
		kept := append(updated, added...)
		if _, ok := handler.allDevices[resourceName]; !ok {
			handler.allDevices[resourceName] = sets.NewString()
		}
		// For now, DevicePluginHandler only keeps track of healthy devices.
		// We can revisit this later when the need comes to track unhealthy devices here.
		for _, dev := range kept {
			if dev.Health == pluginapi.Healthy {
				handler.allDevices[resourceName].Insert(dev.ID)
			} else {
				handler.allDevices[resourceName].Delete(dev.ID)
			}
		}
		for _, dev := range deleted {
			handler.allDevices[resourceName].Delete(dev.ID)
		}
		capacity[v1.ResourceName(resourceName)] = *resource.NewQuantity(int64(handler.allDevices[resourceName].Len()), resource.DecimalSI)
		updateCapacityFunc(capacity)
	}

	mgr, err := deviceplugin.NewManagerImpl(pluginapi.KubeletSocket, deviceManagerMonitorCallback)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize device plugin manager: %+v", err)
	}

	handler.devicePluginManager = mgr
	handler.devicePluginManagerMonitorCallback = deviceManagerMonitorCallback
	return handler, nil
}

func (h *DevicePluginHandlerImpl) Start() error {
	return h.devicePluginManager.Start()
}

func (h *DevicePluginHandlerImpl) Devices() map[string][]*pluginapi.Device {
	return h.devicePluginManager.Devices()
}

func (h *DevicePluginHandlerImpl) Allocate(pod *v1.Pod, container *v1.Container, activePods []*v1.Pod) ([]*pluginapi.AllocateResponse, error) {
	glog.V(3).Infof("Allocate called")
	var ret []*pluginapi.AllocateResponse
	h.updateAllocatedDevices(activePods)
	for k, v := range container.Resources.Limits {
		key := string(k)
		needed := int(v.Value())
		glog.V(3).Infof("needs %d %s", needed, key)
		if !deviceplugin.IsDeviceName(k) || needed == 0 {
			continue
		}
		h.Lock()
		// Gets list of devices that have already been allocated.
		// This can happen if a container restarts for example.
		if h.allocatedDevices[key] == nil {
			h.allocatedDevices[key] = newPodDevices()
		}
		devices := h.allocatedDevices[key].getDevices(string(pod.UID), container.Name)
		if devices != nil {
			glog.V(3).Infof("Found pre-allocated devices for container %q in Pod %q: %v", container.Name, pod.UID, devices.List())
			needed = needed - devices.Len()
		}
		// Get Devices in use.
		devicesInUse := h.allocatedDevices[key].devices()
		glog.V(3).Infof("all devices: %v", h.allDevices[key].List())
		glog.V(3).Infof("devices in use: %v", devicesInUse.List())
		// Get a list of available devices.
		available := h.allDevices[key].Difference(devicesInUse)
		glog.V(3).Infof("devices available: %v", available.List())
		if int(available.Len()) < needed {
			return nil, fmt.Errorf("requested number of devices unavailable. Requested: %d, Available: %d", needed, available.Len())
		}
		allocated := available.UnsortedList()[:needed]
		for _, device := range allocated {
			// Update internal allocated device cache.
			h.allocatedDevices[key].insert(string(pod.UID), container.Name, device)
		}
		h.Unlock()
		// devicePluginManager.Allocate involves RPC calls to device plugin, which
		// could be heavy-weight. Therefore we want to perform this operation outside
		// mutex lock. Note if Allcate call fails, we may leave container resources
		// partially allocated for the failed container. We rely on updateAllocatedDevices()
		// to garbage collect these resources later. Another side effect is that if
		// we have X resource A and Y resource B in total, and two containers, container1
		// and container2 both require X resource A and Y resource B. Both allocation
		// requests may fail if we serve them in mixed order.
		// TODO: may revisit this part later if we see inefficient resource allocation
		// in real use as the result of this.
		resp, err := h.devicePluginManager.Allocate(key, append(devices.UnsortedList(), allocated...))
		if err != nil {
			return nil, err
		}
		ret = append(ret, resp)
	}
	return ret, nil
}

// devicesInUse returns a list of custom devices in use along with the
// respective pods that are using them.
func devicesInUse() map[string]*podDevices {
	// TODO: gets the initial state from checkpointing.
	return make(map[string]*podDevices)
}

// updateAllDevices gets all healthy devices.
// TODO: consider to remove this function.
func (h *DevicePluginHandlerImpl) updateAllDevices() {
	h.allDevices = make(map[string]sets.String)
	for key, devs := range h.devicePluginManager.Devices() {
		h.allDevices[key] = sets.NewString()
		for _, dev := range devs {
			glog.V(3).Infof("insert device %s for resource %s", dev.ID, key)
			h.allDevices[key].Insert(dev.ID)
		}
	}
}

// updateAllocatedDevices updates the list of GPUs in use.
// It gets a list of active pods and then frees any GPUs that are bound to
// terminated pods. Returns error on failure.
func (h *DevicePluginHandlerImpl) updateAllocatedDevices(activePods []*v1.Pod) {
	h.Lock()
	defer h.Unlock()
	activePodUids := sets.NewString()
	for _, pod := range activePods {
		activePodUids.Insert(string(pod.UID))
	}
	for _, v := range h.allocatedDevices {
		allocatedPodUids := v.pods()
		podsToBeRemoved := allocatedPodUids.Difference(activePodUids)
		glog.V(5).Infof("pods to be removed: %v", podsToBeRemoved.List())
		v.delete(podsToBeRemoved.List())
	}
}
