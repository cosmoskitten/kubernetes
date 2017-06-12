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

	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"
)

func newForwardingRuleMetricContext(request, region string) *metricContext {
	return &metricContext{
		start:      time.Now(),
		attributes: []string{"forwardingrule_" + request, region, unusedMetricLabel},
	}
}

// CreateGlobalForwardingRule creates and returns a
// GlobalForwardingRule that points to the given TargetHttp(s)Proxy.
// targetProxyLink is the SelfLink of a TargetHttp(s)Proxy.
func (gce *GCECloud) CreateGlobalForwardingRule(targetProxyLink, ip, name, portRange string) (*compute.ForwardingRule, error) {
	mc := newForwardingRuleMetricContext("create", "")
	rule := &compute.ForwardingRule{
		Name:       name,
		IPAddress:  ip,
		Target:     targetProxyLink,
		PortRange:  portRange,
		IPProtocol: "TCP",
	}
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.Insert(%s, %v): start", gce.projectID, rule)
	op, err := gce.service.GlobalForwardingRules.Insert(gce.projectID, rule).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.Insert(%s, %v): end", gce.projectID, rule)
	if err != nil {
		return nil, mc.Observe(err)
	}
	if err = gce.waitForGlobalOp(op, mc); err != nil {
		return nil, err
	}

	return gce.GetGlobalForwardingRule(name)
}

// SetProxyForGlobalForwardingRule links the given TargetHttp(s)Proxy with the given GlobalForwardingRule.
// targetProxyLink is the SelfLink of a TargetHttp(s)Proxy.
func (gce *GCECloud) SetProxyForGlobalForwardingRule(forwardingRuleName, targetProxyLink string) error {
	mc := newForwardingRuleMetricContext("set_proxy", "")
	op, err := gce.service.GlobalForwardingRules.SetTarget(
		gce.projectID, forwardingRuleName, &compute.TargetReference{Target: targetProxyLink}).Do()
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// DeleteGlobalForwardingRule deletes the GlobalForwardingRule by name.
func (gce *GCECloud) DeleteGlobalForwardingRule(name string) error {
	mc := newForwardingRuleMetricContext("delete", "")
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.Delete(%s, %s): start", gce.projectID, name)
	op, err := gce.service.GlobalForwardingRules.Delete(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.Delete(%s, %s): end", gce.projectID, name)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// GetGlobalForwardingRule returns the GlobalForwardingRule by name.
func (gce *GCECloud) GetGlobalForwardingRule(name string) (*compute.ForwardingRule, error) {
	mc := newForwardingRuleMetricContext("get", "")
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.Get(%s, %s): start", gce.projectID, name)
	v, err := gce.service.GlobalForwardingRules.Get(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.Get(%s, %s): end", gce.projectID, name)
	return v, mc.Observe(err)
}

// ListGlobalForwardingRules lists all GlobalForwardingRules in the project.
func (gce *GCECloud) ListGlobalForwardingRules() (*compute.ForwardingRuleList, error) {
	mc := newForwardingRuleMetricContext("list", "")
	// TODO: use PageToken to list all not just the first 500
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.List(%s): start", gce.projectID)
	v, err := gce.service.GlobalForwardingRules.List(gce.projectID).Do()
	glog.V(cloudprovider.APILogLevel).Infof("GlobalForwardingRules.List(%s): end", gce.projectID)
	return v, mc.Observe(err)
}

// GetRegionForwardingRule returns the RegionalForwardingRule by name & region.
func (gce *GCECloud) GetRegionForwardingRule(name, region string) (*compute.ForwardingRule, error) {
	mc := newForwardingRuleMetricContext("get", region)
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.Get(%s, %s, %s): start", gce.projectID, region, name)
	v, err := gce.service.ForwardingRules.Get(gce.projectID, region, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.Get(%s, %s, %s): end", gce.projectID, region, name)
	return v, mc.Observe(err)
}

// ListRegionForwardingRules lists all RegionalForwardingRules in the project & region.
func (gce *GCECloud) ListRegionForwardingRules(region string) (*compute.ForwardingRuleList, error) {
	mc := newForwardingRuleMetricContext("list", region)
	// TODO: use PageToken to list all not just the first 500
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.List(%s, %s): start", gce.projectID, region)
	v, err := gce.service.ForwardingRules.List(gce.projectID, region).Do()
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.List(%s, %s): end", gce.projectID, region)
	return v, mc.Observe(err)
}

// CreateRegionForwardingRule creates and returns a
// RegionalForwardingRule that points to the given BackendService
func (gce *GCECloud) CreateRegionForwardingRule(rule *compute.ForwardingRule, region string) error {
	mc := newForwardingRuleMetricContext("create", region)
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.Insert(%s, %s, %v): start", gce.projectID, region, rule)
	op, err := gce.service.ForwardingRules.Insert(gce.projectID, region, rule).Do()
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.Insert(%s, %s, %v): end", gce.projectID, region, rule)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForRegionOp(op, region, mc)
}

// DeleteRegionForwardingRule deletes the RegionalForwardingRule by name & region.
func (gce *GCECloud) DeleteRegionForwardingRule(name, region string) error {
	mc := newForwardingRuleMetricContext("delete", region)
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.Delete(%s, %s, %s): start", gce.projectID, region, name)
	op, err := gce.service.ForwardingRules.Delete(gce.projectID, region, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("ForwardingRules.Delete(%s, %s, %s): end", gce.projectID, region, name)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForRegionOp(op, region, mc)
}
