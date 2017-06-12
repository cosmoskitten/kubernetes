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

func newFirewallMetricContext(request string) *metricContext {
	return &metricContext{
		start:      time.Now(),
		attributes: []string{"firewall_" + request, unusedMetricLabel, unusedMetricLabel},
	}
}

// GetFirewall returns the Firewall by name.
func (gce *GCECloud) GetFirewall(name string) (*compute.Firewall, error) {
	mc := newFirewallMetricContext("get")
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Get(%s, %s): start", gce.projectID, name)
	v, err := gce.service.Firewalls.Get(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Get(%s, %s): end", gce.projectID, name)
	return v, mc.Observe(err)
}

// CreateFirewall creates the passed firewall
func (gce *GCECloud) CreateFirewall(f *compute.Firewall) error {
	mc := newFirewallMetricContext("create")
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Insert(%s, %v): start", gce.projectID, f)
	op, err := gce.service.Firewalls.Insert(gce.projectID, f).Do()
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Insert(%s, %v): end", gce.projectID, f)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// DeleteFirewall deletes the given firewall rule.
func (gce *GCECloud) DeleteFirewall(name string) error {
	mc := newFirewallMetricContext("delete")
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Delete(%s, %s): start", gce.projectID, name)
	op, err := gce.service.Firewalls.Delete(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Delete(%s, %s): end", gce.projectID, name)
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForGlobalOp(op, mc)
}

// UpdateFirewall applies the given firewall as an update to an existing service.
func (gce *GCECloud) UpdateFirewall(f *compute.Firewall) error {
	mc := newFirewallMetricContext("update")
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Update(%s, %s, %v): start", gce.projectID, f.Name, f)
	op, err := gce.service.Firewalls.Update(gce.projectID, f.Name, f).Do()
	glog.V(cloudprovider.APILogLevel).Infof("Firewalls.Update(%s, %s, %v): end", gce.projectID, f.Name, f)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}
