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
	"log"
	"net"
	"os"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
)

// DevicePluginStub is a mock device plugin
type DevicePluginStub struct {
	devs   []*pluginapi.Device
	socket string

	stop   chan interface{}
	update chan []*pluginapi.Device

	server *grpc.Server
}

// NewDevicePluginStub returns an initialized DevicePluginStub
func NewDevicePluginStub(devs []*pluginapi.Device, socket string) *DevicePluginStub {
	return &DevicePluginStub{
		devs:   devs,
		socket: socket,

		stop:   make(chan interface{}),
		update: make(chan []*pluginapi.Device),
	}
}

// Start starts the gRPC server of the device plugin
func (m *DevicePluginStub) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)
	log.Println("Starting to serve on", m.socket)

	return nil
}

// Stop stops the gRPC server
func (m *DevicePluginStub) Stop() error {
	m.server.Stop()

	return m.cleanup()
}

// ListAndWatch lists devices and update that list according to the Update call
func (m *DevicePluginStub) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	log.Println("ListAndWatch")
	var devs []*pluginapi.Device

	for _, d := range m.devs {
		devs = append(devs, &pluginapi.Device{
			ID:     d.ID,
			Health: pluginapi.Healthy,
		})
	}

	s.Send(&pluginapi.ListAndWatchResponse{Devices: devs})

	for {
		select {
		case <-m.stop:
			return nil
		case updated := <-m.update:
			s.Send(&pluginapi.ListAndWatchResponse{Devices: updated})
		}
	}
}

// Update allows the device plugin to send new devices through ListAndWatch
func (m *DevicePluginStub) Update(devs []*pluginapi.Device) {
	m.update <- devs
}

// Allocate does a mock allocation
func (m *DevicePluginStub) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	log.Printf("Allocate, %+v", r)

	var response pluginapi.AllocateResponse
	return &response, nil
}

func (m *DevicePluginStub) cleanup() error {
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
