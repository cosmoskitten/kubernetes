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
	"net"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/proxy"
)

func TestIPPart(t *testing.T) {
	const noError = ""

	testCases := []struct {
		endpoint      string
		expectedIP    string
		expectedError string
	}{
		{"1.2.3.4", "1.2.3.4", noError},
		{"1.2.3.4:9999", "1.2.3.4", noError},
		{"2001:db8::1:1", "2001:db8::1:1", noError},
		{"[2001:db8::2:2]:9999", "2001:db8::2:2", noError},
		{"1.2.3.4::9999", "", "too many colons"},
		{"1.2.3.4:[0]", "", "unexpected '[' in address"},
	}

	for _, tc := range testCases {
		ip := IPPart(tc.endpoint)
		if tc.expectedError == noError {
			if ip != tc.expectedIP {
				t.Errorf("Unexpected IP for %s: Expected: %s, Got %s", tc.endpoint, tc.expectedIP, ip)
			}
		} else if ip != "" {
			t.Errorf("Error did not occur for %s, expected: '%s' error", tc.endpoint, tc.expectedError)
		}
	}
}

func TestToCIDR(t *testing.T) {
	testCases := []struct {
		ip           string
		expectedAddr string
	}{
		{"1.2.3.4", "1.2.3.4/32"},
		{"2001:db8::1:1", "2001:db8::1:1/128"},
	}

	for _, tc := range testCases {
		ip := net.ParseIP(tc.ip)
		addr := ToCIDR(ip)
		if addr != tc.expectedAddr {
			t.Errorf("Unexpected host address for %s: Expected: %s, Got %s", tc.ip, tc.expectedAddr, addr)
		}
	}
}

func makeNSN(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}

func makeServicePortName(ns, name, port string) proxy.ServicePortName {
	return proxy.ServicePortName{
		NamespacedName: makeNSN(ns, name),
		Port:           port,
	}
}

type fakeEndpointsInfo struct {
	endpoint string
	isLocal  bool
}

func (f *fakeEndpointsInfo) Endpoint() string {
	return f.endpoint
}

func (f *fakeEndpointsInfo) IsLocal() bool {
	return f.isLocal
}

func (f *fakeEndpointsInfo) IPPart() string {
	return IPPart(f.endpoint)
}

func (f *fakeEndpointsInfo) Equal(other proxy.EndpointsInfo) bool {
	return f.Endpoint() == other.Endpoint() &&
		f.IsLocal() == other.IsLocal() &&
		f.IPPart() == other.IPPart()
}

func TestGetLocalIPs(t *testing.T) {
	testCases := []struct {
		endpointsMap ProxyEndpointsMap
		expected     map[types.NamespacedName]sets.String
	}{{
		// Case[0]: nothing
		endpointsMap: ProxyEndpointsMap{},
		expected:     map[types.NamespacedName]sets.String{},
	}, {
		// Case[1]: unnamed port
		endpointsMap: ProxyEndpointsMap{
			makeServicePortName("ns1", "ep1", ""): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "1.1.1.1:11", isLocal: false},
			},
		},
		expected: map[types.NamespacedName]sets.String{},
	}, {
		// Case[2]: unnamed port local
		endpointsMap: ProxyEndpointsMap{
			makeServicePortName("ns1", "ep1", ""): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "1.1.1.1:11", isLocal: true},
			},
		},
		expected: map[types.NamespacedName]sets.String{
			{Namespace: "ns1", Name: "ep1"}: sets.NewString("1.1.1.1"),
		},
	}, {
		// Case[3]: named local and non-local ports for the same IP.
		endpointsMap: ProxyEndpointsMap{
			makeServicePortName("ns1", "ep1", "p11"): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "1.1.1.1:11", isLocal: false},
				&fakeEndpointsInfo{endpoint: "1.1.1.2:11", isLocal: true},
			},
			makeServicePortName("ns1", "ep1", "p12"): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "1.1.1.1:12", isLocal: false},
				&fakeEndpointsInfo{endpoint: "1.1.1.2:12", isLocal: true},
			},
		},
		expected: map[types.NamespacedName]sets.String{
			{Namespace: "ns1", Name: "ep1"}: sets.NewString("1.1.1.2"),
		},
	}, {
		// Case[4]: named local and non-local ports for different IPs.
		endpointsMap: ProxyEndpointsMap{
			makeServicePortName("ns1", "ep1", "p11"): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "1.1.1.1:11", isLocal: false},
			},
			makeServicePortName("ns2", "ep2", "p22"): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "2.2.2.2:22", isLocal: true},
				&fakeEndpointsInfo{endpoint: "2.2.2.22:22", isLocal: true},
			},
			makeServicePortName("ns2", "ep2", "p23"): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "2.2.2.3:23", isLocal: true},
			},
			makeServicePortName("ns4", "ep4", "p44"): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "4.4.4.4:44", isLocal: true},
				&fakeEndpointsInfo{endpoint: "4.4.4.5:44", isLocal: false},
			},
			makeServicePortName("ns4", "ep4", "p45"): []proxy.EndpointsInfo{
				&fakeEndpointsInfo{endpoint: "4.4.4.6:45", isLocal: true},
			},
		},
		expected: map[types.NamespacedName]sets.String{
			{Namespace: "ns2", Name: "ep2"}: sets.NewString("2.2.2.2", "2.2.2.22", "2.2.2.3"),
			{Namespace: "ns4", Name: "ep4"}: sets.NewString("4.4.4.4", "4.4.4.6"),
		},
	}}

	for tci, tc := range testCases {
		// outputs
		localIPs := GetLocalIPs(tc.endpointsMap)

		if !reflect.DeepEqual(localIPs, tc.expected) {
			t.Errorf("[%d] expected %#v, got %#v", tci, tc.expected, localIPs)
		}
	}
}
