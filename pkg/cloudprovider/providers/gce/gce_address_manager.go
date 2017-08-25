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
	"fmt"
	"net/http"

	computebeta "google.golang.org/api/compute/v0.beta"

	"github.com/golang/glog"
)

type addressManager struct {
	logPrefix   string
	svc         CloudAddressService
	lbName      string
	serviceName string
	targetIP    string
	addressType lbScheme
	region      string
	subnetURL   string
	tryRelease  bool
}

func newAddressManager(svc CloudAddressService, serviceName, region, subnetURL, lbName, targetIP string, addressType lbScheme) *addressManager {
	return &addressManager{
		svc:         svc,
		logPrefix:   fmt.Sprintf("AddressManager(%q)", lbName),
		region:      region,
		serviceName: serviceName,
		lbName:      lbName,
		targetIP:    targetIP,
		addressType: addressType,
		tryRelease:  true,
		subnetURL:   subnetURL,
	}
}

// HoldAddress will ensure that the IP is reserved with an address - either owned by the controller
// or by a user. If the address is not the loadBalancerName, then it's assumed to be a user's address.
func (am *addressManager) HoldAddress() error {
	// HoldAddress starts with retrieving the address that we use for this load balancer (by name).
	// Retrieving an address by IP will indicate if the IP is reserved and if reserved by the user
	// or the controller, but won't tell us the current state of the controller's IP. The address
	// could be reserving another address; therefore, it would need to be deleted. In the normal
	// case of using a controller address, retrieving the address by name results in the fewest API
	// calls since it indicates whether a Delete is necessary before Reserve.
	glog.V(3).Infof("%v: attempting hold of IP %v with type %v", am.logPrefix, am.targetIP, am.addressType)
	// Get the address in case it was orphaned earlier
	addr, err := am.svc.GetBetaRegionAddress(am.lbName, am.region)
	if err != nil && !isNotFound(err) {
		return err
	}

	if addr != nil {
		// If address exists, check if the address IP is different from what's expected.
		if addr.Address != am.targetIP || addr.AddressType != string(am.addressType) {
			glog.V(3).Infof("%v: existing address %q has IP %v Type %v which does not match targeted IP %v Type %v. Attemping to delete.", am.logPrefix, addr.Name, addr.Address, addr.AddressType, am.targetIP, am.addressType)
			err := am.svc.DeleteRegionAddress(addr.Name, am.region)
			if err != nil {
				if isNotFound(err) {
					glog.V(3).Infof("%v: address %q was not found. Ignoring.", am.logPrefix, addr.Name)
				} else {
					return err
				}
			} else {
				glog.V(3).Infof("%v: successfully deleted previous address %q", am.logPrefix, addr.Name)
			}
		} else {
			glog.V(3).Infof("%v: address %q already reserves targeted IP %v of type %v. No further action required.", am.logPrefix, addr.Name, am.targetIP, am.addressType)
			return nil
		}
	}

	return am.ensureAddressReservation()
}

// ReleaseAddress will release the address if it's owned by the controller.
func (am *addressManager) ReleaseAddress() error {
	if !am.tryRelease {
		glog.V(3).Infof("%v: not attempting release of address %v.", am.logPrefix, am.targetIP)
		return nil
	}

	glog.V(3).Infof("%v: releasing address %v named %q", am.logPrefix, am.targetIP, am.lbName)
	// Controller only ever tries to unreserve the address named with the load balancer's name.
	err := am.svc.DeleteRegionAddress(am.lbName, am.region)
	if err != nil {
		if isNotFound(err) {
			glog.Warningf("%v: address %q was not found. Ignoring.", am.logPrefix, am.targetIP, am.lbName)
			return nil
		}

		return err
	}

	glog.V(3).Infof("%v: successfully released IP %v named %q", am.logPrefix, am.targetIP, am.lbName)
	return nil
}

func (am *addressManager) ensureAddressReservation() error {
	// Try reserving the IP with controller-owned address name
	newAddr := &computebeta.Address{
		Name:        am.lbName,
		Description: fmt.Sprintf(`{"kubernetes.io/service-name":"%s"}`, am.serviceName),
		Address:     am.targetIP,
		AddressType: string(am.addressType),
		Subnetwork:  am.subnetURL,
	}

	err := am.svc.ReserveBetaRegionAddress(newAddr, am.region)
	if err != nil {
		if !isHTTPErrorCode(err, http.StatusConflict) {
			return err
		}
	} else {
		glog.V(3).Infof("%v: successfully reserved IP %v with name %q", am.logPrefix, am.targetIP, newAddr.Name)
		return nil
	}

	glog.V(3).Infof("%v: could not reserve IP %v due to err: %v", am.logPrefix, am.targetIP, err)

	// Reserving the address failed due to a conflict. The address manager just checked that no address
	// exists with the lbName, so it may belong to the user.
	addr, err := am.svc.GetBetaRegionAddressByIP(am.region, am.targetIP)
	if err != nil {
		return fmt.Errorf("could not find address with IP %v after getting conflict error while creating address: %v", am.targetIP, err)
	}

	// If the retrieved address is not named with the loadbalancer name, then the controller does not own it.
	if addr.Name != am.lbName {
		glog.V(3).Infof("%v: address %q was already reserved with name: %q, description: %q", am.logPrefix, am.targetIP, addr.Name, addr.Description)
		am.tryRelease = false
	} else {
		// The address with this name is checked at the beginning of 'HoldAddress()', but for some reason
		// it was re-created by this point. May be possible that two controllers are running.
		glog.Warning("%v: address %q unexpectedly existed with IP %q.", am.logPrefix, addr.Name, am.targetIP)
	}
	return nil
}
