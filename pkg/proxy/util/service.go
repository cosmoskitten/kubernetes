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

package util

import (
	"reflect"
	"sync"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/proxy"
)

type ProxyServiceMap map[proxy.ServicePortName]proxy.ServiceInfo

type serviceChange struct {
	previous ProxyServiceMap
	current  ProxyServiceMap
}

type ServiceChangeMap struct {
	lock  sync.Mutex
	items map[types.NamespacedName]*serviceChange
}

func NewServiceChangeMap() ServiceChangeMap {
	return ServiceChangeMap{
		items: make(map[types.NamespacedName]*serviceChange),
	}
}

func (sm *ProxyServiceMap) Merge(other ProxyServiceMap) sets.String {
	existingPorts := sets.NewString()
	for svcPortName, info := range other {
		existingPorts.Insert(svcPortName.Port)
		_, exists := (*sm)[svcPortName]
		if !exists {
			glog.V(1).Infof("Adding new service port %q at %s:%d/%s", svcPortName, info.ClusterIP(), info.Port(), info.Protocol())
		} else {
			glog.V(1).Infof("Updating existing service port %q at %s:%d/%s", svcPortName, info.ClusterIP(), info.Port(), info.Protocol())
		}
		(*sm)[svcPortName] = info
	}
	return existingPorts
}

func (sm *ProxyServiceMap) Unmerge(other ProxyServiceMap, existingPorts, staleServices sets.String) {
	for svcPortName := range other {
		if existingPorts.Has(svcPortName.Port) {
			continue
		}
		info, exists := (*sm)[svcPortName]
		if exists {
			glog.V(1).Infof("Removing service port %q", svcPortName)
			if info.Protocol() == api.ProtocolUDP {
				staleServices.Insert(info.ClusterIP())
			}
			delete(*sm, svcPortName)
		} else {
			glog.Errorf("Service port %q removed, but doesn't exists", svcPortName)
		}
	}
}

// <serviceMap> is updated by this function (based on the given changes).
// <changes> map is cleared after applying them.
func UpdateServiceMap(
	serviceMap ProxyServiceMap,
	changes *ServiceChangeMap) (result UpdateServiceMapResult) {
	result.StaleServices = sets.NewString()

	func() {
		changes.lock.Lock()
		defer changes.lock.Unlock()
		for _, change := range changes.items {
			existingPorts := serviceMap.Merge(change.current)
			serviceMap.Unmerge(change.previous, existingPorts, result.StaleServices)
		}
		changes.items = make(map[types.NamespacedName]*serviceChange)
	}()

	// TODO: If this will appear to be computationally expensive, consider
	// computing this incrementally similarly to serviceMap.
	result.HCServices = make(map[types.NamespacedName]uint16)
	for svcPortName, info := range serviceMap {
		if info.HealthCheckNodePort() != 0 {
			result.HCServices[svcPortName.NamespacedName] = uint16(info.HealthCheckNodePort())
		}
	}

	return result
}

func (scm *ServiceChangeMap) Update(namespacedName *types.NamespacedName, previous, current *api.Service, serviceToServiceMap func(service *api.Service) ProxyServiceMap) bool {
	scm.lock.Lock()
	defer scm.lock.Unlock()

	change, exists := scm.items[*namespacedName]
	if !exists {
		change = &serviceChange{}
		change.previous = serviceToServiceMap(previous)
		scm.items[*namespacedName] = change
	}
	change.current = serviceToServiceMap(current)
	if reflect.DeepEqual(change.previous, change.current) {
		delete(scm.items, *namespacedName)
	}
	return len(scm.items) > 0
}
