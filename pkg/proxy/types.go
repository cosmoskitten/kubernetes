/*
Copyright 2015 The Kubernetes Authors.

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

package proxy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api"
)

// ProxyProvider is the interface provided by proxier implementations.
type ProxyProvider interface {
	// Sync immediately synchronizes the ProxyProvider's current state to iptables.
	Sync()
	// SyncLoop runs periodic work.
	// This is expected to run as a goroutine or as the main loop of the app.
	// It does not return.
	SyncLoop()
}

// ServicePortName carries a namespace + name + portname.  This is the unique
// identifier for a load-balanced service.
type ServicePortName struct {
	types.NamespacedName
	Port string
}

func (spn ServicePortName) String() string {
	return fmt.Sprintf("%s:%s", spn.NamespacedName.String(), spn.Port)
}

// ServiceInfo is an interface which abstracts a service info.
type ServiceInfo interface {
	// ClusterIP returns service cluster IP.
	ClusterIP() string
	// Port returns service port.
	Port() int
	// Protocol returns service protocol.
	Protocol() api.Protocol
	// HealthCheckNodePort returns service health check node port.
	HealthCheckNodePort() int
}

// EndpointsInfo is an interface which abstracts an endpoints info.
type EndpointsInfo interface {
	// Endpoint returns endpoints string.
	Endpoint() string
	// IsLocal returns if endpoints is local.
	IsLocal() bool
	// IPPart returns IP part of endpoints.
	IPPart() string
	// Equal checks if two endpoints are equal.
	Equal(EndpointsInfo) bool
}

// EndpointServicePair is an Endpoint and ServicePortName pair.
type EndpointServicePair struct {
	Endpoint        string
	ServicePortName ServicePortName
}
