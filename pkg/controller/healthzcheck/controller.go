/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package healthzcheck

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/record"
	unversioned_core "k8s.io/kubernetes/pkg/client/typed/generated/core/unversioned"
	"k8s.io/kubernetes/pkg/probe"
	http_probe "k8s.io/kubernetes/pkg/probe/http"
	"k8s.io/kubernetes/pkg/util/wait"
)

// APIServerHealthChecker checks the health of API servers,
// using the server address and ports provided.
type APIServerHealthChecker interface {
	Check(masterAddress string, masterPorts []api.EndpointPort) (bool, error)
}

// httpsHealthzChecker checks API server health by issuing a GET request over HTTPS to
// the API server at the configured "healthz" path (e.g. "/healthz").  In order for this
// to work, the caller (the controller manager) must be able to reach all of the API servers.
// It searches for HTTP ports if no HTTPS ports are available.
type httpsHealthzChecker struct {
	healthzPath string
	client      *http.Client
}

func (c *httpsHealthzChecker) choosePort(ports []api.EndpointPort) (int, string, error) {
	healthPort := -1
	healthProto := ""
	for _, port := range ports {
		// use the HTTP port if necessary, but let any HTTPS ports override it
		if healthPort == -1 && port.Name == "http" && port.Protocol == api.ProtocolTCP {
			healthPort = port.Port
			healthProto = "http"
			// continue on to check if any HTTPS ports exist further down
		} else if port.Name == "https" && port.Protocol == api.ProtocolTCP {
			healthPort = port.Port
			healthProto = "https"
			break
		}
	}

	if healthPort == -1 {
		return -1, "", fmt.Errorf("no TCP port was called 'https' or 'http'")
	}

	return healthPort, healthProto, nil
}

// Check checks the health of the given API server by issuing a GET request over HTTPS
// to the given address (using the 'https' port, or 'http' port if necessary)
func (c *httpsHealthzChecker) Check(masterAddress string, masterPorts []api.EndpointPort) (bool, error) {
	if len(masterPorts) == 0 {
		return false, fmt.Errorf("unable to determine port to use to check API server %s healthz: no ports provided", masterAddress)
	}

	masterPort, masterProto, err := c.choosePort(masterPorts)
	if err != nil {
		return false, fmt.Errorf("unable to determine port to use to check API server %s healthz URL: %v", masterAddress, err)
	}
	urlParsed, err := url.Parse(fmt.Sprintf("%s://%s:%v/%s", masterProto, masterAddress, masterPort, c.healthzPath))
	if err != nil {
		return false, err
	}
	maxRetries := 4
	retries := 0
	delay := 125 * time.Millisecond
	for retries < maxRetries {
		status, body, err := http_probe.DoHTTPProbe(urlParsed, map[string][]string{}, c.client)
		if err != nil || status != probe.Success {
			glog.V(5).Infof("Unable to probe health URL for master %s (at %s) on retry %v/%v: error: %v, status: %v, result: %s", masterAddress, urlParsed, retries+1, maxRetries, err, status, body)
			delay *= 2
			retries++
			time.Sleep(delay)
			continue
		}

		glog.V(5).Infof("Probe of health URL for master %s (at %s) succeeded: %s", masterAddress, urlParsed, body)
		return true, nil
	}

	return false, nil
}

// NewHTTPSHealthzChecker constructs a new APIServerHealthChecker which uses GET
// requests over HTTPS (or HTTP if necessary) to check the API server health.
func NewHTTPSHealthzChecker(client *http.Client, path string) APIServerHealthChecker {
	return &httpsHealthzChecker{
		healthzPath: path,
		client:      client,
	}
}

// APIServerHealthCheckController is a controller loop which peroidically checks each
// of the API servers listed at a given service (e.g. "kubernetes") using the provided
// APIServerHealthChecker.
type APIServerHealthCheckController struct {
	endpoints         unversioned_core.EndpointsInterface
	healthz           APIServerHealthChecker
	masterServiceName string
	eventRecorder     record.EventRecorder
}

// NewAPIServerHealthCheckController constructs a APIServerHealthCheckController with the provided endpoints
// information, checker, and event recorder.  Note that the process running this controller loop must generally
// be able to reach the API servers in order to perform the health checks.
func NewAPIServerHealthCheckController(endpoints unversioned_core.EndpointsInterface, healthzChecker APIServerHealthChecker, masterServiceName string, eventRecorder record.EventRecorder) *APIServerHealthCheckController {
	return &APIServerHealthCheckController{
		endpoints:         endpoints,
		healthz:           healthzChecker,
		masterServiceName: masterServiceName,
		eventRecorder:     eventRecorder,
	}
}

// Run launches this controller loop at the provided interval.
func (c *APIServerHealthCheckController) Run(syncPeriod time.Duration) {
	go wait.Until(func() {
		if err := c.CheckMasterHealth(); err != nil {
			glog.Errorf("Unable to check the health of the API servers: %v", err)
		}
	}, syncPeriod, wait.NeverStop)
}

// CheckMasterHealth iterates through each of the API servers listed in the configured
// service's endpoints, checking the health of each in turn, and removing dead API servers
// from the list of endpoints.
func (c *APIServerHealthCheckController) CheckMasterHealth() error {
	masterEndpoints, err := c.endpoints.Get(c.masterServiceName)
	if err != nil {
		return err
	}

	// for the master kubernetes service, there should be only one subset
	if len(masterEndpoints.Subsets) != 1 {
		return fmt.Errorf("malformed master service endpoints set: there should be only one subset present")
	}

	masterSubset := masterEndpoints.Subsets[0]
	masterAddrs := masterSubset.Addresses
	aliveAddrs := make([]api.EndpointAddress, 0, len(masterAddrs))
	for _, addr := range masterAddrs {
		if present, err := c.healthz.Check(addr.IP, masterSubset.Ports); err != nil {
			return err
		} else if present {
			glog.V(4).Infof("Master %v is alive", addr)
			aliveAddrs = append(aliveAddrs, addr)
		} else {
			glog.V(4).Infof("Master %v is dead", addr)
			c.eventRecorder.Eventf(masterEndpoints, api.EventTypeNormal, "RemovedAPIServer", "The API server endpoint at %s did not seem to be alive, so it was removed from the list of valid API server endpoints", addr.IP)
		}
	}

	if len(aliveAddrs) == 0 {
		return fmt.Errorf("no API servers appeared to be alive, assuming something is wrong with this controller")
	}

	masterEndpoints.Subsets[0].Addresses = aliveAddrs
	if _, err := c.endpoints.Update(masterEndpoints); err != nil {
		return err
	}

	glog.V(4).Infof("updated master service %q endpoints with %v live masters", c.masterServiceName, len(aliveAddrs))

	return nil
}
