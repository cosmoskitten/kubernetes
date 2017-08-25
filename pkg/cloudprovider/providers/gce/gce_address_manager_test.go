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

package gce

import (
	"testing"

	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
)

const svcNm = "my-service"
const region = "us-central1"
const subnet = "/projects/x/regions/us-central1/subnetworks/customsub"

// TestAddressManagerBasic tests the typical case of reserving and unreserving an address.
func TestAddressManagerBasic(t *testing.T) {
	svc := NewFakeCloudAddressService()
	loadBalancerName := "a111111111111111"
	forwardingRuleAddr := "1.1.1.1"
	loadBalancerIP := ""
	targetIP := getTargetIP(forwardingRuleAddr, loadBalancerIP)

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	err := mgr.HoldAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addr, err := svc.GetRegionAddress(loadBalancerName, region)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.Address != targetIP {
		t.Fatalf("expected address: %v, got: %v", targetIP, addr.Address)
	}

	err = mgr.ReleaseAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = svc.GetRegionAddress(loadBalancerName, region)
	if err == nil || !isNotFound(err) {
		t.Fatalf("expected NotFound error, got: %v", err)
	}
}

// TestAddressManagerOrphaned tests the case where the address exists with the IP being equal
// to the requested address (forwarding rule or loadbalancer IP).
func TestAddressManagerOrphaned(t *testing.T) {
	svc := NewFakeCloudAddressService()
	loadBalancerName := "a111111111111111"
	forwardingRuleAddr := "1.1.1.1"
	loadBalancerIP := ""
	region := "us-central1"
	targetIP := getTargetIP(forwardingRuleAddr, loadBalancerIP)

	addr := &computebeta.Address{Name: loadBalancerName, Address: targetIP, AddressType: "INTERNAL"}
	if err := svc.ReserveBetaRegionAddress(addr, region); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	err := mgr.HoldAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addr, err = svc.GetBetaRegionAddress(loadBalancerName, region)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.Address != targetIP {
		t.Fatalf("expected address: %v, got: %v", targetIP, addr.Address)
	}

	err = mgr.ReleaseAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = svc.GetRegionAddress(loadBalancerName, region)
	if err == nil || !isNotFound(err) {
		t.Fatalf("expected NotFound error, got: %v", err)
	}
}

// TestAddressManagerOutdatedOrphan tests the case where an address exists but points to
// an IP other than the forwarding rule or loadbalancer IP.
func TestAddressManagerOutdatedOrphan(t *testing.T) {
	svc := NewFakeCloudAddressService()
	loadBalancerName := "a111111111111111"
	forwardingRuleAddr := "1.1.1.1"
	loadBalancerIP := ""
	region := "us-central1"
	addrIP := "1.1.0.0"
	targetIP := getTargetIP(forwardingRuleAddr, loadBalancerIP)

	addr := &computebeta.Address{Name: loadBalancerName, Address: addrIP, AddressType: string(schemeExternal)}
	if err := svc.ReserveBetaRegionAddress(addr, region); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	err := mgr.HoldAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addr, err = svc.GetBetaRegionAddress(loadBalancerName, region)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.Address != targetIP {
		t.Fatalf("expected address: %v, got: %v", targetIP, addr.Address)
	}

	err = mgr.ReleaseAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = svc.GetRegionAddress(loadBalancerName, region)
	if err == nil || !isNotFound(err) {
		t.Fatalf("expected NotFound error, got: %v", err)
	}
}

// TestAddressManagerExternallyOwned tests the case where the address exists but isn't
// owned by the controller.
func TestAddressManagerExternallyOwned(t *testing.T) {
	svc := NewFakeCloudAddressService()
	loadBalancerName := "a111111111111111"
	forwardingRuleAddr := "1.1.1.1"
	loadBalancerIP := ""
	region := "us-central1"
	addrName := "my-important-address"
	targetIP := getTargetIP(forwardingRuleAddr, loadBalancerIP)

	addr := &compute.Address{Name: addrName, Address: targetIP}
	if err := svc.ReserveRegionAddress(addr, region); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	err := mgr.HoldAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addr, err = svc.GetRegionAddress(addrName, region)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.Address != targetIP {
		t.Fatalf("expected address: %v, got: %v", targetIP, addr.Address)
	}

	err = mgr.ReleaseAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = svc.GetRegionAddress(addrName, region)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func getTargetIP(forwardingRuleAddr, loadBalancerIP string) string {
	if loadBalancerIP != "" {
		return loadBalancerIP
	}
	return forwardingRuleAddr
}
