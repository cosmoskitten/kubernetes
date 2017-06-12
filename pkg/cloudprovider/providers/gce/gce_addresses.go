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
	"time"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"

	"k8s.io/kubernetes/pkg/cloudprovider"
)

func newAddressMetricContext(request, region string) *metricContext {
	return &metricContext{
		start:      time.Now(),
		attributes: []string{"address_" + request, region, unusedMetricLabel},
	}
}

// ReserveGlobalAddress creates a global address.
// Caller is allocated a random IP if they do not specify an ipAddress. If an
// ipAddress is specified, it must belong to the current project, eg: an
// ephemeral IP associated with a global forwarding rule.
func (gce *GCECloud) ReserveGlobalAddress(addr *compute.Address) (*compute.Address, error) {
	mc := newAddressMetricContext("reserve", "")
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Insert(%s, %v): start", gce.projectID, addr)
	op, err := gce.service.GlobalAddresses.Insert(gce.projectID, addr).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Insert(%s, %v): end", gce.projectID, addr)
	if err != nil {
		return nil, mc.Observe(err)
	}

	if err := gce.waitForGlobalOp(op, mc); err != nil {
		return nil, err
	}

	return gce.GetGlobalAddress(addr.Name)
}

// DeleteGlobalAddress deletes a global address by name.
func (gce *GCECloud) DeleteGlobalAddress(name string) error {
	mc := newAddressMetricContext("delete", "")
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Delete(%s, %s): start", gce.projectID, name)
	op, err := gce.service.GlobalAddresses.Delete(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Delete(%s, %s): end", gce.projectID, name)
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForGlobalOp(op, mc)
}

// GetGlobalAddress returns the global address by name.
func (gce *GCECloud) GetGlobalAddress(name string) (*compute.Address, error) {
	mc := newAddressMetricContext("get", "")
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Get(%s, %s): start", gce.projectID, name)
	v, err := gce.service.GlobalAddresses.Get(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Get(%s, %s): end", gce.projectID, name)
	return v, mc.Observe(err)
}

// ReserveRegionAddress creates a region address
func (gce *GCECloud) ReserveRegionAddress(addr *compute.Address, region string) (*compute.Address, error) {
	mc := newAddressMetricContext("reserve", region)
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Insert(%s, %s, %v): start", gce.projectID, region, addr)
	op, err := gce.service.Addresses.Insert(gce.projectID, region, addr).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Insert(%s, %s, %v): end", gce.projectID, region, addr)
	if err != nil {
		return nil, mc.Observe(err)
	}
	if err := gce.waitForRegionOp(op, region, mc); err != nil {
		return nil, err
	}

	return gce.GetRegionAddress(addr.Name, region)
}

// DeleteRegionAddress deletes a region address by name.
func (gce *GCECloud) DeleteRegionAddress(name, region string) error {
	mc := newAddressMetricContext("delete", region)
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Delete(%s, %s, %s): start", gce.projectID, region, name)
	op, err := gce.service.Addresses.Delete(gce.projectID, region, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Delete(%s, %s, %s): end", gce.projectID, region, name)
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForRegionOp(op, region, mc)
}

// GetRegionAddress returns the region address by name
func (gce *GCECloud) GetRegionAddress(name, region string) (*compute.Address, error) {
	mc := newAddressMetricContext("get", region)
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Get(%s, %s, %s): start", gce.projectID, region, name)
	v, err := gce.service.Addresses.Get(gce.projectID, region, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalAddresses.Get(%s, %s, %s): end", gce.projectID, region, name)
	return v, mc.Observe(err)
}
