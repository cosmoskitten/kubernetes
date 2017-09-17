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

package deviceplugin

import (
	"sync"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
)

type store interface {
	Devices() map[string][]*pluginapi.Device

	Get(resourceName string) (*endpoint, bool)
	Delete(resourceName string)

	// Replace will stop the old endpoint if it exists
	Replace(resourceName string, e *endpoint)

	StopEndpoints()
}

type endpointStore struct {
	mutex     sync.Mutex
	endpoints map[string]*endpoint // Key is the ResourceName
}

func newEndpointStore() store {
	return &endpointStore{
		endpoints: make(map[string]*endpoint),
	}
}

func (s *endpointStore) Devices() map[string][]*pluginapi.Device {
	devs := make(map[string][]*pluginapi.Device)

	s.mutex.Lock()
	for k, v := range s.endpoints {
		devs[k] = v.getDevices()
	}
	s.mutex.Unlock()

	return devs
}

func (s *endpointStore) Get(resourceName string) (*endpoint, bool) {
	s.mutex.Lock()
	e, ok := s.endpoints[resourceName]
	s.mutex.Unlock()

	return e, ok
}

func (s *endpointStore) Delete(resourceName string) {
	s.mutex.Lock()
	delete(s.endpoints, resourceName)
	s.mutex.Unlock()
}

func (s *endpointStore) Replace(resourceName string, e *endpoint) {
	s.mutex.Lock()
	old, ok := s.endpoints[resourceName]
	s.endpoints[resourceName] = e
	s.mutex.Unlock()

	if ok && old != nil {
		old.stop()
	}
}

func (s *endpointStore) StopEndpoints() {
	s.mutex.Lock()
	for _, e := range s.endpoints {
		e.stop()
	}
	s.mutex.Unlock()
}
