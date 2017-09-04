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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api"
	proxytest "k8s.io/kubernetes/pkg/proxy/util/testing"
)

const dummyDevice = "kube-ipvs0"

func TestShouldSkipService(t *testing.T) {
	testCases := []struct {
		service    *api.Service
		svcName    types.NamespacedName
		shouldSkip bool
	}{
		{
			// Cluster IP is None
			service: &api.Service{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				Spec: api.ServiceSpec{
					ClusterIP: api.ClusterIPNone,
				},
			},
			svcName:    types.NamespacedName{Namespace: "foo", Name: "bar"},
			shouldSkip: true,
		},
		{
			// Cluster IP is empty
			service: &api.Service{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				Spec: api.ServiceSpec{
					ClusterIP: "",
				},
			},
			svcName:    types.NamespacedName{Namespace: "foo", Name: "bar"},
			shouldSkip: true,
		},
		{
			// ExternalName type service
			service: &api.Service{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				Spec: api.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Type:      api.ServiceTypeExternalName,
				},
			},
			svcName:    types.NamespacedName{Namespace: "foo", Name: "bar"},
			shouldSkip: true,
		},
		{
			// ClusterIP type service with ClusterIP set
			service: &api.Service{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				Spec: api.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Type:      api.ServiceTypeClusterIP,
				},
			},
			svcName:    types.NamespacedName{Namespace: "foo", Name: "bar"},
			shouldSkip: false,
		},
		{
			// NodePort type service with ClusterIP set
			service: &api.Service{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				Spec: api.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Type:      api.ServiceTypeNodePort,
				},
			},
			svcName:    types.NamespacedName{Namespace: "foo", Name: "bar"},
			shouldSkip: false,
		},
		{
			// LoadBalancer type service with ClusterIP set
			service: &api.Service{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				Spec: api.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Type:      api.ServiceTypeLoadBalancer,
				},
			},
			svcName:    types.NamespacedName{Namespace: "foo", Name: "bar"},
			shouldSkip: false,
		},
	}

	for i := range testCases {
		skip := ShouldSkipService(testCases[i].svcName, testCases[i].service)
		if skip != testCases[i].shouldSkip {
			t.Errorf("case %d: expect %v, got %v", i, testCases[i].shouldSkip, skip)
		}
	}
}

func TestEnsureAddressBind(t *testing.T) {
	testIpv4 := "10.20.30.40"
	testIpv6 := "2001::1"
	handle := proxytest.NewFakeNetlinkHandle()
	// Success.
	if exists, err := EnsureAddressBind(testIpv4, dummyDevice, handle); err != nil {
		t.Errorf("expected success, got %v", err)
	} else if exists {
		t.Errorf("expected exists = false")
	}

	if exists, err := EnsureAddressBind(testIpv6, dummyDevice, handle); err != nil {
		t.Errorf("expected success, got %v", err)
	} else if exists {
		t.Errorf("expected exists = false")
	}

	if _, ok := handle.BoundIP[dummyDevice]; !ok {
		t.Errorf("expected ip bound on %s", dummyDevice)
	}
	if handle.BoundIP[dummyDevice].Len() != 2 ||
		!handle.BoundIP[dummyDevice].Has(testIpv4+"/32") ||
		!handle.BoundIP[dummyDevice].Has(testIpv6+"/128") {
		t.Errorf("wrong bound ip, got %v", handle.BoundIP[dummyDevice].List())
	}
	// Exists.
	if exists, err := EnsureAddressBind(testIpv4, dummyDevice, handle); err != nil {
		t.Errorf("expected success, got %v", err)
	} else if !exists {
		t.Errorf("expected exists = true")
	}
}

func TestUnbindAddress(t *testing.T) {
	testIpv4 := "10.20.30.41"
	testIpv6 := "2001::1"
	handle := proxytest.NewFakeNetlinkHandle()
	handle.BoundIP[dummyDevice] = sets.NewString(testIpv4+"/32", testIpv6+"/128", "10.20.30.42/32")
	// Success.
	if err := UnbindAddress(testIpv4, dummyDevice, handle); err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if err := UnbindAddress(testIpv6, dummyDevice, handle); err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if handle.BoundIP[dummyDevice].Len() != 1 {
		t.Errorf("expected 1 ip left, got %v", handle.BoundIP[dummyDevice].List())
	}
	if handle.BoundIP[dummyDevice].Has(testIpv4+"/32") ||
		handle.BoundIP[dummyDevice].Has(testIpv6+"/128") {
		t.Errorf("wrong ip left, got %v", handle.BoundIP[dummyDevice].List())
	}
	// Failure.
	if err := UnbindAddress(testIpv6, dummyDevice, handle); err == nil {
		t.Errorf("expected failure")
	}
}
