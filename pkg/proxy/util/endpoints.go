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
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/proxy"
)

type ProxyEndpointsMap map[proxy.ServicePortName][]proxy.EndpointsInfo

type endpointsChange struct {
	previous ProxyEndpointsMap
	current  ProxyEndpointsMap
}

type EndpointsChangeMap struct {
	lock     sync.Mutex
	hostname string
	items    map[types.NamespacedName]*endpointsChange
}

func NewEndpointsChangeMap(hostname string) EndpointsChangeMap {
	return EndpointsChangeMap{
		hostname: hostname,
		items:    make(map[types.NamespacedName]*endpointsChange),
	}
}

func (em ProxyEndpointsMap) Merge(other ProxyEndpointsMap) {
	for svcPortName := range other {
		em[svcPortName] = other[svcPortName]
	}
}

func (em ProxyEndpointsMap) Unmerge(other ProxyEndpointsMap) {
	for svcPortName := range other {
		delete(em, svcPortName)
	}
}

type UpdateEndpointMapResult struct {
	HCEndpoints       map[types.NamespacedName]int
	StaleEndpoints    map[proxy.EndpointServicePair]bool
	StaleServiceNames map[proxy.ServicePortName]bool
}

type UpdateServiceMapResult struct {
	HCServices    map[types.NamespacedName]uint16
	StaleServices sets.String
}

// <endpointsMap> is updated by this function (based on the given changes).
// <changes> map is cleared after applying them.
func UpdateEndpointsMap(
	endpointsMap ProxyEndpointsMap,
	changes *EndpointsChangeMap,
	hostname string) (result UpdateEndpointMapResult) {
	result.StaleEndpoints = make(map[proxy.EndpointServicePair]bool)
	result.StaleServiceNames = make(map[proxy.ServicePortName]bool)

	func() {
		changes.lock.Lock()
		defer changes.lock.Unlock()
		for _, change := range changes.items {
			endpointsMap.Unmerge(change.previous)
			endpointsMap.Merge(change.current)
			detectStaleConnections(change.previous, change.current, result.StaleEndpoints, result.StaleServiceNames)
		}
		changes.items = make(map[types.NamespacedName]*endpointsChange)
	}()

	if !utilfeature.DefaultFeatureGate.Enabled(features.ExternalTrafficLocalOnly) {
		return
	}

	// TODO: If this will appear to be computationally expensive, consider
	// computing this incrementally similarly to endpointsMap.
	result.HCEndpoints = make(map[types.NamespacedName]int)
	localIPs := GetLocalIPs(endpointsMap)
	for nsn, ips := range localIPs {
		result.HCEndpoints[nsn] = len(ips)
	}

	return result
}

func GetLocalIPs(endpointsMap ProxyEndpointsMap) map[types.NamespacedName]sets.String {
	localIPs := make(map[types.NamespacedName]sets.String)
	for svcPortName := range endpointsMap {
		for _, ep := range endpointsMap[svcPortName] {
			if ep.IsLocal() {
				nsn := svcPortName.NamespacedName
				if localIPs[nsn] == nil {
					localIPs[nsn] = sets.NewString()
				}
				localIPs[nsn].Insert(ep.IPPart()) // just the IP part
			}
		}
	}
	return localIPs
}

// <staleEndpoints> and <staleServices> are modified by this function with detected stale connections.
func detectStaleConnections(oldEndpointsMap, newEndpointsMap ProxyEndpointsMap, staleEndpoints map[proxy.EndpointServicePair]bool, staleServiceNames map[proxy.ServicePortName]bool) {
	for svcPortName, epList := range oldEndpointsMap {
		for _, ep := range epList {
			stale := true
			for i := range newEndpointsMap[svcPortName] {
				if newEndpointsMap[svcPortName][i].Equal(ep) {
					stale = false
					break
				}
			}
			if stale {
				glog.V(4).Infof("Stale endpoint %v -> %v", svcPortName, ep.Endpoint())
				staleEndpoints[proxy.EndpointServicePair{Endpoint: ep.Endpoint(), ServicePortName: svcPortName}] = true
			}
		}
	}

	for svcPortName, epList := range newEndpointsMap {
		// For udp service, if its backend changes from 0 to non-0. There may exist a conntrack entry that could blackhole traffic to the service.
		if len(epList) > 0 && len(oldEndpointsMap[svcPortName]) == 0 {
			staleServiceNames[svcPortName] = true
		}
	}
}

func (ecm *EndpointsChangeMap) Update(namespacedName *types.NamespacedName, previous, current *api.Endpoints, endpointsToEndpointsMap func(endpoints *api.Endpoints, hostname string) ProxyEndpointsMap) bool {
	ecm.lock.Lock()
	defer ecm.lock.Unlock()

	change, exists := ecm.items[*namespacedName]
	if !exists {
		change = &endpointsChange{}
		// change.previous = endpointsToEndpointsMap(previous, ecm.hostname)
		change.previous = endpointsToEndpointsMap(previous, ecm.hostname)
		ecm.items[*namespacedName] = change
	}
	// change.current = endpointsToEndpointsMap(current, ecm.hostname)
	change.current = endpointsToEndpointsMap(current, ecm.hostname)
	if reflect.DeepEqual(change.previous, change.current) {
		delete(ecm.items, *namespacedName)
	}
	return len(ecm.items) > 0
}
