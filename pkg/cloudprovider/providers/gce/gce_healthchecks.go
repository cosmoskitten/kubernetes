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

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/master/ports"
	utilversion "k8s.io/kubernetes/pkg/util/version"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"
)

const (
	nodesHealthCheckPath   = "/healthz"
	lbNodesHealthCheckPort = ports.ProxyHealthzPort
)

var (
	minNodesHealthCheckVersion *utilversion.Version
)

func init() {
	if v, err := utilversion.ParseGeneric("1.7.0"); err != nil {
		panic(err)
	} else {
		minNodesHealthCheckVersion = v
	}
}

func newHealthcheckMetricContext(request string) *metricContext {
	return &metricContext{
		start:      time.Now(),
		attributes: []string{"healthcheck_" + request, unusedMetricLabel, unusedMetricLabel},
	}
}

// GetHttpHealthCheck returns the given HttpHealthCheck by name.
func (gce *GCECloud) GetHttpHealthCheck(name string) (*compute.HttpHealthCheck, error) {
	mc := newHealthcheckMetricContext("get_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Get(%s, %s): start", gce.projectID, name)
	v, err := gce.service.HttpHealthChecks.Get(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Get(%s, %s): end", gce.projectID, name)
	return v, mc.Observe(err)
}

// UpdateHttpHealthCheck applies the given HttpHealthCheck as an update.
func (gce *GCECloud) UpdateHttpHealthCheck(hc *compute.HttpHealthCheck) error {
	mc := newHealthcheckMetricContext("update_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Update(%s, %s, %v): start", gce.projectID, hc.Name, hc)
	op, err := gce.service.HttpHealthChecks.Update(gce.projectID, hc.Name, hc).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Update(%s, %s, %v): end", gce.projectID, hc.Name, hc)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// DeleteHttpHealthCheck deletes the given HttpHealthCheck by name.
func (gce *GCECloud) DeleteHttpHealthCheck(name string) error {
	mc := newHealthcheckMetricContext("delete_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Delete(%s, %s): start", gce.projectID, name)
	op, err := gce.service.HttpHealthChecks.Delete(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Delete(%s, %s): end", gce.projectID, name)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// CreateHttpHealthCheck creates the given HttpHealthCheck.
func (gce *GCECloud) CreateHttpHealthCheck(hc *compute.HttpHealthCheck) error {
	mc := newHealthcheckMetricContext("create_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Insert(%s, %v): start", gce.projectID, hc)
	op, err := gce.service.HttpHealthChecks.Insert(gce.projectID, hc).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.Insert(%s, %v): end", gce.projectID, hc)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// ListHttpHealthChecks lists all HttpHealthChecks in the project.
func (gce *GCECloud) ListHttpHealthChecks() (*compute.HttpHealthCheckList, error) {
	mc := newHealthcheckMetricContext("list_legacy")
	// TODO: use PageToken to list all not just the first 500
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.List(%s): start", gce.projectID)
	v, err := gce.service.HttpHealthChecks.List(gce.projectID).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpHealthChecks.List(%s): end", gce.projectID)
	return v, mc.Observe(err)
}

// Legacy HTTPS Health Checks

// GetHttpsHealthCheck returns the given HttpsHealthCheck by name.
func (gce *GCECloud) GetHttpsHealthCheck(name string) (*compute.HttpsHealthCheck, error) {
	mc := newHealthcheckMetricContext("get_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Get(%s, %s): start", gce.projectID, name)
	v, err := gce.service.HttpsHealthChecks.Get(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Get(%s, %s): end", gce.projectID, name)
	mc.Observe(err)
	return v, err
}

// UpdateHttpsHealthCheck applies the given HttpsHealthCheck as an update.
func (gce *GCECloud) UpdateHttpsHealthCheck(hc *compute.HttpsHealthCheck) error {
	mc := newHealthcheckMetricContext("update_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Update(%s, %s, %v): start", gce.projectID, hc.Name, hc)
	op, err := gce.service.HttpsHealthChecks.Update(gce.projectID, hc.Name, hc).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Update(%s, %s, %v): end", gce.projectID, hc.Name, hc)
	if err != nil {
		mc.Observe(err)
		return err
	}

	return gce.waitForGlobalOp(op, mc)
}

// DeleteHttpsHealthCheck deletes the given HttpsHealthCheck by name.
func (gce *GCECloud) DeleteHttpsHealthCheck(name string) error {
	mc := newHealthcheckMetricContext("delete_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Delete(%s, %s): start", gce.projectID, name)
	op, err := gce.service.HttpsHealthChecks.Delete(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Delete(%s, %s): end", gce.projectID, name)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// CreateHttpsHealthCheck creates the given HttpsHealthCheck.
func (gce *GCECloud) CreateHttpsHealthCheck(hc *compute.HttpsHealthCheck) error {
	mc := newHealthcheckMetricContext("create_legacy")
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Insert(%s, %v): start", gce.projectID, hc)
	op, err := gce.service.HttpsHealthChecks.Insert(gce.projectID, hc).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.Insert(%s, %v): end", gce.projectID, hc)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// ListHttpsHealthChecks lists all HttpsHealthChecks in the project.
func (gce *GCECloud) ListHttpsHealthChecks() (*compute.HttpsHealthCheckList, error) {
	mc := newHealthcheckMetricContext("list_legacy")
	// TODO: use PageToken to list all not just the first 500
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.List(%s): start", gce.projectID)
	v, err := gce.service.HttpsHealthChecks.List(gce.projectID).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.List(%s): end", gce.projectID)
	return v, mc.Observe(err)
}

// Generic HealthCheck

// GetHealthCheck returns the given HealthCheck by name.
func (gce *GCECloud) GetHealthCheck(name string) (*compute.HealthCheck, error) {
	mc := newHealthcheckMetricContext("get")
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.List(%s, %v): start", gce.projectID, name)
	v, err := gce.service.HealthChecks.Get(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HttpsHealthChecks.List(%s, %v): end", gce.projectID, name)
	return v, mc.Observe(err)
}

// UpdateHealthCheck applies the given HealthCheck as an update.
func (gce *GCECloud) UpdateHealthCheck(hc *compute.HealthCheck) error {
	mc := newHealthcheckMetricContext("update")
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.Update(%s, %s, %v): start", gce.projectID, hc.Name, hc)
	op, err := gce.service.HealthChecks.Update(gce.projectID, hc.Name, hc).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.Update(%s, %s, %v): end", gce.projectID, hc.Name, hc)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// DeleteHealthCheck deletes the given HealthCheck by name.
func (gce *GCECloud) DeleteHealthCheck(name string) error {
	mc := newHealthcheckMetricContext("delete")
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.Delete(%s, %s): start", gce.projectID, name)
	op, err := gce.service.HealthChecks.Delete(gce.projectID, name).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.Delete(%s, %s): end", gce.projectID, name)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// CreateHealthCheck creates the given HealthCheck.
func (gce *GCECloud) CreateHealthCheck(hc *compute.HealthCheck) error {
	mc := newHealthcheckMetricContext("create")
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.Insert(%s, %v): start", gce.projectID, hc)
	op, err := gce.service.HealthChecks.Insert(gce.projectID, hc).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.Insert(%s, %v): end", gce.projectID, hc)
	if err != nil {
		return mc.Observe(err)
	}

	return gce.waitForGlobalOp(op, mc)
}

// ListHealthChecks lists all HealthCheck in the project.
func (gce *GCECloud) ListHealthChecks() (*compute.HealthCheckList, error) {
	mc := newHealthcheckMetricContext("list")
	// TODO: use PageToken to list all not just the first 500
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.List(%s): start", gce.projectID)
	v, err := gce.service.HealthChecks.List(gce.projectID).Do()
	glog.V(cloudprovider.APILogLevel).Infof("HealthChecks.List(%s): end", gce.projectID)
	return v, mc.Observe(err)
}

// GetNodesHealthCheckPort returns the health check port used by the GCE load
// balancers (l4) for performing health checks on nodes.
func GetNodesHealthCheckPort() int32 {
	return lbNodesHealthCheckPort
}

// GetNodesHealthCheckPath returns the health check path used by the GCE load
// balancers (l4) for performing health checks on nodes.
func GetNodesHealthCheckPath() string {
	return nodesHealthCheckPath
}

// isAtLeastMinNodesHealthCheckVersion checks if a version is higher than
// `minNodesHealthCheckVersion`.
func isAtLeastMinNodesHealthCheckVersion(vstring string) bool {
	version, err := utilversion.ParseGeneric(vstring)
	if err != nil {
		glog.Errorf("vstring (%s) is not a valid version string: %v", vstring, err)
		return false
	}
	return version.AtLeast(minNodesHealthCheckVersion)
}

// supportsNodesHealthCheck returns false if anyone of the nodes has version
// lower than `minNodesHealthCheckVersion`.
func supportsNodesHealthCheck(nodes []*v1.Node) bool {
	for _, node := range nodes {
		if !isAtLeastMinNodesHealthCheckVersion(node.Status.NodeInfo.KubeProxyVersion) {
			return false
		}
	}
	return true
}
