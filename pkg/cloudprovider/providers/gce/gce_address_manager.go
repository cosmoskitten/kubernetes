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

	"github.com/golang/glog"
	computebeta "google.golang.org/api/compute/v0.beta"
)

type addressManager struct {
	svc         CloudAddressService
	lbName      string
	serviceName string
	requestedIP string
	addressType lbScheme
	region      string
	tryRelease  bool
}

func newAddressManager(svc CloudAddressService, serviceName, region, lbName, requestedIP string, addressType lbScheme) *addressManager {
	return &addressManager{
		svc:         svc,
		region:      region,
		serviceName: serviceName,
		lbName:      lbName,
		requestedIP: requestedIP,
		addressType: addressType,
		tryRelease:  true,
	}
}

func (am *addressManager) HoldAddress() error {
	// Get the address in case it was orphaned earlier
	addr, err := am.svc.GetRegionAddress(am.lbName, am.region)
	if err != nil && !isNotFound(err) {
		return err
	}

	if addr != nil {
		// If address exists, check if the address IP is different from what's expected.
		if addr.Address != am.requestedIP {
			glog.V(3).Infof("AddressManager(%q): Current address %q has IP %v which does not match requested IP %v. Attemping to delete address.", am.lbName, addr.Name, addr.Address, am.requestedIP)
			err := am.svc.DeleteRegionAddress(addr.Name, am.region)
			if err != nil {
				if isNotFound(err) {
					glog.V(3).Infof("AddressManager(%q): Address %q was not found. Ignoring.", am.lbName, addr.Name)
				} else {
					return err
				}
			} else {
				glog.V(3).Infof("AddressManager(%q): Successfully deleted address %q", am.lbName, addr.Name)
			}
		} else {
			glog.V(3).Infof("AddressManager(%q): Address %q already reserves requested IP %v. No further action required.", am.lbName, addr.Name, am.requestedIP)

			return nil
		}
	}

	// Ensure the requested IP is reserved
	return am.ensureAddressReservation()
}

func (am *addressManager) ReleaseAddress() error {
	if !am.tryRelease {
		glog.V(3).Infof("AddressManager(%q): Not attempting release of address %v.", am.lbName, am.requestedIP)
		return nil
	}

	glog.V(3).Infof("AddressManager(%q): Releasing address %v named %q", am.lbName, am.requestedIP, am.lbName)
	// Controller only ever tries to unreserve the address named with the load balancer's name.
	err := am.svc.DeleteRegionAddress(am.lbName, am.region)
	if err != nil {
		if isNotFound(err) {
			glog.Warningf("AddressManager(%q): Address %q was not found. Ignoring.", am.lbName, am.requestedIP, am.lbName)
			return nil
		}

		return err
	}

	glog.V(3).Infof("AddressManager(%q): Successfully released ip %v named %q", am.lbName, am.requestedIP, am.lbName)
	return nil
}

func (am *addressManager) ensureAddressReservation() error {
	// Try reserving the IP with controller-owned address name
	newAddr := &computebeta.Address{
		Name:        am.lbName,
		Description: fmt.Sprintf(`{"kubernetes.io/service-name":"%s"}`, am.serviceName),
		Address:     am.requestedIP,
		AddressType: string(am.addressType),
	}

	if err := am.svc.ReserveBetaRegionAddress(newAddr, am.region); err != nil {
		if !isHTTPErrorCode(err, http.StatusConflict) {
			return err
		}
	} else {
		glog.V(3).Infof("AddressManager(%q): Successfully reserved IP %q with name %q", am.lbName, am.requestedIP, newAddr.Name)
		return nil
	}

	// Could not reserve requestedIP because of a conflict. Retrieving address which currently reserves it.
	addr, err := am.svc.GetRegionAddressByIP(am.region, am.requestedIP)
	if err != nil {
		return fmt.Errorf("could not get address with IP %v after getting conflict while creating address: %v", am.requestedIP, err)
	}

	// If the retrieved address is not named with the loadbalancer name, then the controller does not own it.
	if addr.Name != am.lbName {
		glog.V(3).Infof("AddressManager(%q): address %q was already reserved with name: %q, description: %q", am.lbName, am.requestedIP, addr.Name, addr.Description)
		am.tryRelease = false
	} else {
		// The address with this name is checked at the beginning of 'HoldAddress()', but for some reason
		// it was re-created by this point. May be possible that two controllers are running.
		glog.Warning("AddressManager(%q): address %q unexpectedly existed with IP %q.", am.lbName, addr.Name, am.requestedIP)
	}
	return nil
}
