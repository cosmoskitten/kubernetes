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
	"strings"
	"time"

	computealpha "google.golang.org/api/compute/v0.alpha"
)

const (
	ExperimentalKey              = "X-Goog-Experiments"
	NEGExperimentalValue         = "EnableNetworkEndpointGroup"
	NEGLoadBalancerType          = "LOAD_BALANCING"
	NEGIPPortNetworkEndpointType = "GCE_VM_IP_PORT"
)

func newNetworkEndpointGroupMetricContext(request string, zone string) *metricContext {
	zoneLabel := zone
	if len(strings.TrimSpace(zone)) == 0 {
		zoneLabel = unusedMetricLabel
	}
	return &metricContext{
		start:      time.Now(),
		attributes: []string{"networkendpointgroup_" + request, unusedMetricLabel, zoneLabel},
	}
}

func (gce *GCECloud) GetNetworkEndpointGroup(name string, zone string) (*computealpha.NetworkEndpointGroup, error) {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return nil, err
	}
	mc := newNetworkEndpointGroupMetricContext("get", zone)
	v, err := gce.serviceAlpha.NetworkEndpointGroups.Get(gce.GetProjectID(), zone, name).Do()
	return v, mc.Observe(err)
}

func (gce *GCECloud) ListNetworkEndpointGroup(zone string) (*computealpha.NetworkEndpointGroupList, error) {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return nil, err
	}
	mc := newNetworkEndpointGroupMetricContext("list", zone)
	v, err := gce.serviceAlpha.NetworkEndpointGroups.List(gce.GetProjectID(), zone).Do()
	return v, mc.Observe(err)
}

func (gce *GCECloud) AggregatedListNetworkEndpointGroup() (*computealpha.NetworkEndpointGroupAggregatedList, error) {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return nil, err
	}
	mc := newNetworkEndpointGroupMetricContext("aggregated", "")
	v, err := gce.serviceAlpha.NetworkEndpointGroups.AggregatedList(gce.GetProjectID()).Do()
	return v, mc.Observe(err)
}

func (gce *GCECloud) CreateNetworkEndpointGroup(neg *computealpha.NetworkEndpointGroup, zone string) error {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return err
	}
	mc := newNetworkEndpointGroupMetricContext("create", zone)
	call := gce.serviceAlpha.NetworkEndpointGroups.Insert(gce.GetProjectID(), zone, neg)
	// TODO: remove this after NEG is enabled
	call.Header().Add(ExperimentalKey, NEGExperimentalValue)
	op, err := call.Do()
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForZoneOp(op, zone, mc)
}

func (gce *GCECloud) DeleteNetworkEndpointGroup(name string, zone string) error {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return err
	}
	mc := newNetworkEndpointGroupMetricContext("delete", zone)
	call := gce.serviceAlpha.NetworkEndpointGroups.Delete(gce.GetProjectID(), zone, name)
	// TODO: remove this after NEG is enabled
	call.Header().Add("X-Goog-Experiments", "EnableNetworkEndpointGroup")
	op, err := call.Do()
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForZoneOp(op, zone, mc)
}

func (gce *GCECloud) AttachNetworkEndpoints(name, zone string, endpoints []*computealpha.NetworkEndpoint) error {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return err
	}
	mc := newNetworkEndpointGroupMetricContext("attach", zone)
	call := gce.serviceAlpha.NetworkEndpointGroups.AttachNetworkEndpoints(gce.GetProjectID(), zone, name, &computealpha.NetworkEndpointGroupsAttachEndpointsRequest{
		NetworkEndpoints: endpoints,
	})
	// TODO: remove this after NEG is enabled
	call.Header().Add("X-Goog-Experiments", "EnableNetworkEndpointGroup")
	op, err := call.Do()
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForZoneOp(op, zone, mc)
}

func (gce *GCECloud) DetachNetworkEndpoints(name, zone string, endpoints []*computealpha.NetworkEndpoint) error {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return err
	}
	mc := newNetworkEndpointGroupMetricContext("detach", zone)
	call := gce.serviceAlpha.NetworkEndpointGroups.DetachNetworkEndpoints(gce.GetProjectID(), zone, name, &computealpha.NetworkEndpointGroupsDetachEndpointsRequest{
		NetworkEndpoints: endpoints,
	})
	// TODO: remove this after NEG is enabled
	call.Header().Add("X-Goog-Experiments", "EnableNetworkEndpointGroup")
	op, err := call.Do()
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForZoneOp(op, zone, mc)
}

func (gce *GCECloud) ListNetworkEndpoints(name, zone string, showHealthStatus bool) (*computealpha.NetworkEndpointGroupsListNetworkEndpoints, error) {
	if err := gce.alphaFeatureEnabled(AlphaFeatureNetworkEndpointGroup); err != nil {
		return nil, err
	}
	healthStatus := "SKIP"
	if showHealthStatus {
		healthStatus = "SHOW"
	}
	mc := newNetworkEndpointGroupMetricContext("list_networkendpoints", zone)
	call := gce.serviceAlpha.NetworkEndpointGroups.ListNetworkEndpoints(gce.GetProjectID(), zone, name, &computealpha.NetworkEndpointGroupsListEndpointsRequest{
		HealthStatus: healthStatus,
	})
	// TODO: remove this after NEG is enabled
	call.Header().Add("X-Goog-Experiments", "EnableNetworkEndpointGroup")
	v, err := call.Do()
	return v, mc.Observe(err)
}
