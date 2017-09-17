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
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
)

type endpointStoreStub struct {
	endpointStore store

	outChan      chan interface{}
	continueChan chan bool
}

type deviceMessage struct{}

type getMessage struct {
	ResourceName string
}

type deleteMessage struct {
	ResourceName string
}

type replaceMessage struct {
	Old *endpoint
	New *endpoint
}

type stopEndpointMessage struct {
}

// ChanOut sends a message containing the parameters of the method called.
// After sending that message, execution blocks until ChanContinue is written on
func newInstrumentedEndpointStoreStub(outChan chan interface{}, continueChan chan bool) store {
	return &endpointStoreStub{
		endpointStore: newEndpointStore(),

		outChan:      outChan,
		continueChan: continueChan,
	}
}

func (s *endpointStoreStub) Devices() map[string][]*pluginapi.Device {
	s.outChan <- deviceMessage{}
	<-s.continueChan

	return s.endpointStore.Devices()
}

func (s *endpointStoreStub) Get(resourceName string) (*endpoint, bool) {
	s.outChan <- getMessage{ResourceName: resourceName}
	<-s.continueChan

	return s.endpointStore.Get(resourceName)
}

func (s *endpointStoreStub) Delete(resourceName string) {
	s.outChan <- deleteMessage{ResourceName: resourceName}
	<-s.continueChan

	s.endpointStore.Delete(resourceName)
}

func (s *endpointStoreStub) Replace(resourceName string, e *endpoint) {
	m := replaceMessage{New: e}
	if old, ok := s.endpointStore.Get(resourceName); ok {
		m.Old = old
	}

	s.outChan <- m
	<-s.continueChan

	s.endpointStore.Replace(resourceName, e)
}

func (s *endpointStoreStub) StopEndpoints() {
	s.outChan <- stopEndpointMessage{}
	<-s.continueChan

	s.endpointStore.StopEndpoints()
}
