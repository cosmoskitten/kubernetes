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
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
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
// identfier for a load-balanced service.
type ServicePortName struct {
	types.NamespacedName
	Port string
}

func (spn ServicePortName) String() string {
	return fmt.Sprintf("%s:%s", spn.NamespacedName.String(), spn.Port)
}

type ServiceInfo interface {
	ClusterIP() string
	Port() int
	Protocol() api.Protocol
	ServiceNameString() string
	OnlyNodeLocalEndpoints() bool
	ServicePortChainName() utiliptables.Chain
	ServiceLBChainName() utiliptables.Chain
	ServiceFirewallChainName() utiliptables.Chain
	LoadBalancerStatus() api.LoadBalancerStatus
	ExternalIPs() []string
	HealthCheckNodePort() int
	LoadBalancerSourceRanges() []string
	NodePort() int
	SessionAffinityType() api.ServiceAffinity
	StickyMaxAgeSeconds() int
}

type EndpointsInfo interface {
	Endpoint() string
	IsLocal() bool
	Protocol() string
	ChainName() utiliptables.Chain
	IPPart() string
	EndpointChain(svcNameString, protocol string) utiliptables.Chain
	Equal(other EndpointsInfo) bool
}

type EndpointServicePair struct {
	Endpoint        string
	ServicePortName ServicePortName
}

func (esp *EndpointServicePair) IPPart() string {
	if index := strings.Index(esp.Endpoint, ":"); index != -1 {
		return esp.Endpoint[0:index]
	}
	return esp.Endpoint
}
