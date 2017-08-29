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

	"github.com/stretchr/testify/assert"
	computebeta "google.golang.org/api/compute/v0.beta"
)

const svcNm = "my-service"
const region = "us-central1"
const subnet = "/projects/x/regions/us-central1/subnetworks/customsub"

// TestAddressManagerNoRequestedIP tests the typical case of passing in no requested IP
func TestAddressManagerNoRequestedIP(t *testing.T) {
	svc := NewFakeCloudAddressService()
	loadBalancerName := "a111111111111111"
	targetIP := ""

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	ipToUse, err := mgr.HoldAddress()
	assert.NoError(t, err)
	assert.NotEmpty(t, ipToUse)

	addr, err := svc.GetRegionAddress(loadBalancerName, region)
	assert.NoError(t, err)
	assert.EqualValues(t, ipToUse, addr.Address)

	err = mgr.ReleaseAddress()
	assert.NoError(t, err)
	_, err = svc.GetRegionAddress(loadBalancerName, region)
	assert.True(t, isNotFound(err))
}

// TestAddressManagerBasic tests the typical case of reserving and unreserving an address.
func TestAddressManagerBasic(t *testing.T) {
	svc := NewFakeCloudAddressService()
	loadBalancerName := "a111111111111111"
	forwardingRuleAddr := "1.1.1.1"
	loadBalancerIP := ""
	targetIP := getTargetIP(forwardingRuleAddr, loadBalancerIP)

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	ipToUse, err := mgr.HoldAddress()
	assert.NoError(t, err)
	assert.NotEmpty(t, ipToUse)

	addr, err := svc.GetRegionAddress(loadBalancerName, region)
	assert.NoError(t, err)
	assert.EqualValues(t, targetIP, addr.Address)

	err = mgr.ReleaseAddress()
	assert.NoError(t, err)
	_, err = svc.GetRegionAddress(loadBalancerName, region)
	assert.True(t, isNotFound(err))
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
	err := svc.ReserveBetaRegionAddress(addr, region)
	assert.NoError(t, err)

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	ipToUse, err := mgr.HoldAddress()
	assert.NoError(t, err)
	assert.NotEmpty(t, ipToUse)

	addr, err = svc.GetBetaRegionAddress(loadBalancerName, region)
	assert.NoError(t, err)
	assert.EqualValues(t, targetIP, addr.Address)

	err = mgr.ReleaseAddress()
	assert.NoError(t, err)
	_, err = svc.GetRegionAddress(loadBalancerName, region)
	assert.True(t, isNotFound(err))
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
	err := svc.ReserveBetaRegionAddress(addr, region)
	assert.NoError(t, err)

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	ipToUse, err := mgr.HoldAddress()
	assert.NoError(t, err)
	assert.NotEmpty(t, ipToUse)

	addr, err = svc.GetBetaRegionAddress(loadBalancerName, region)
	assert.NoError(t, err)
	assert.EqualValues(t, targetIP, addr.Address)

	err = mgr.ReleaseAddress()
	assert.NoError(t, err)
	_, err = svc.GetRegionAddress(loadBalancerName, region)
	assert.True(t, isNotFound(err))
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

	addr := &computebeta.Address{Name: addrName, Address: targetIP, AddressType: string(schemeInternal)}
	err := svc.ReserveBetaRegionAddress(addr, region)
	assert.NoError(t, err)

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	ipToUse, err := mgr.HoldAddress()
	assert.NoError(t, err)
	assert.NotEmpty(t, ipToUse)

	addr, err = svc.GetBetaRegionAddress(addrName, region)
	assert.NoError(t, err)
	assert.EqualValues(t, targetIP, addr.Address)

	err = mgr.ReleaseAddress()
	assert.NoError(t, err)
	_, err = svc.GetRegionAddress(addrName, region)
	assert.NoError(t, err)
}

// TestAddressManagerExternallyOwned tests the case where the address exists but isn't
// owned by the controller. However, this address has the wrong type.
func TestAddressManagerBadExternallyOwned(t *testing.T) {
	svc := NewFakeCloudAddressService()
	loadBalancerName := "a111111111111111"
	forwardingRuleAddr := "1.1.1.1"
	loadBalancerIP := ""
	region := "us-central1"
	addrName := "my-important-address"
	targetIP := getTargetIP(forwardingRuleAddr, loadBalancerIP)

	addr := &computebeta.Address{Name: addrName, Address: targetIP, AddressType: string(schemeExternal)}
	err := svc.ReserveBetaRegionAddress(addr, region)
	assert.NoError(t, err)

	mgr := newAddressManager(svc, svcNm, region, subnet, loadBalancerName, targetIP, schemeInternal)
	_, err = mgr.HoldAddress()
	assert.NotNil(t, err)
}

func getTargetIP(forwardingRuleAddr, loadBalancerIP string) string {
	if loadBalancerIP != "" {
		return loadBalancerIP
	}
	return forwardingRuleAddr
}
