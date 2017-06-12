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

func newTargetPoolMetricContext(request, region string) *metricContext {
	return &metricContext{
		start:      time.Now(),
		attributes: []string{"targetpool_" + request, region, unusedMetricLabel},
	}
}

// GetTargetPool returns the TargetPool by name.
func (gce *GCECloud) GetTargetPool(name, region string) (*compute.TargetPool, error) {
	mc := newTargetPoolMetricContext("get", region)
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.Get(%s, %s, %s): start", gce.projectID, region, name)
	v, err := gce.service.TargetPools.Get(gce.projectID, region, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.Get(%s, %s, %s): stop", gce.projectID, region, name)
	return v, mc.Observe(err)
}

// CreateTargetPool creates the passed TargetPool
func (gce *GCECloud) CreateTargetPool(tp *compute.TargetPool, region string) (*compute.TargetPool, error) {
	mc := newTargetPoolMetricContext("create", region)
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.Insert(%s, %s, %v): start", gce.projectID, region, tp)
	op, err := gce.service.TargetPools.Insert(gce.projectID, region, tp).Do()
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.Insert(%s, %s, %v): stop", gce.projectID, region, tp)
	if err != nil {
		return nil, mc.Observe(err)
	}

	if err := gce.waitForRegionOp(op, region, mc); err != nil {
		return nil, err
	}

	return gce.GetTargetPool(tp.Name, region)
}

// DeleteTargetPool deletes TargetPool by name.
func (gce *GCECloud) DeleteTargetPool(name, region string) error {
	mc := newTargetPoolMetricContext("delete", region)
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.Delete(%s, %s, %s): start", gce.projectID, region, name)
	op, err := gce.service.TargetPools.Delete(gce.projectID, region, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.Delete(%s, %s, %s): stop", gce.projectID, region, name)
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForRegionOp(op, region, mc)
}

// AddInstancesToTargetPool adds instances by link to the TargetPool
func (gce *GCECloud) AddInstancesToTargetPool(name, region string, instanceRefs []*compute.InstanceReference) error {
	add := &compute.TargetPoolsAddInstanceRequest{Instances: instanceRefs}
	mc := newTargetPoolMetricContext("add_instances", region)
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.AddInstance(%s, %s, %s, %v): start", gce.projectID, region, name, add)
	op, err := gce.service.TargetPools.AddInstance(gce.projectID, region, name, add).Do()
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.AddInstance(%s, %s, %s, %v): stop", gce.projectID, region, name, add)
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForRegionOp(op, region, mc)
}

// RemoveInstancesToTargetPool removes instances by link to the TargetPool
func (gce *GCECloud) RemoveInstancesFromTargetPool(name, region string, instanceRefs []*compute.InstanceReference) error {
	remove := &compute.TargetPoolsRemoveInstanceRequest{Instances: instanceRefs}
	mc := newTargetPoolMetricContext("remove_instances", region)
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.RemoveInstance(%s, %s, %s, %v): start", gce.projectID, region, name, remove)
	op, err := gce.service.TargetPools.RemoveInstance(gce.projectID, region, name, remove).Do()
	glog.V(cloudprovider.APILogLevel).Infof("TargetPools.RemoveInstance(%s, %s, %s, %v): stop", gce.projectID, region, name, remove)
	if err != nil {
		return mc.Observe(err)
	}
	return gce.waitForRegionOp(op, region, mc)
}
