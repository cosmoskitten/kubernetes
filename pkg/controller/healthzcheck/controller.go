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
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/record"
	unversioned_core "k8s.io/kubernetes/pkg/client/typed/generated/core/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
)

type APIServerHealthChecker interface {
	Check(masterAddress string, masterPorts []api.EndpointPort) (bool, error)
}

type httpsHealthzChecker struct {
	healthzPath string
	client      *http.Client
}

func (c *httpsHealthzChecker) Check(masterAddress string, masterPorts []api.EndpointPort) (bool, error) {
	if len(masterPorts) == 0 {
		return false, fmt.Errorf("unable to determine port to use to check API server %s healthz: no ports provided", masterAddress)
	}

	masterPort := -1
	for _, port := range masterPorts {
		if port.Name == "https" && port.Protocol == api.ProtocolTCP {
			masterPort = port.Port
			break
		}
	}

	if masterPort == -1 {
		return false, fmt.Errorf("unable to determine port to use to check API server %s healthz: no TCP port was called 'https'", masterAddress)
	}

	url := fmt.Sprintf("https://%s:%v/%s", masterAddress, masterPort, c.healthzPath)
	maxRetries := 4
	retries := 0
	delay := 125 * time.Millisecond
	for retries < maxRetries {
		// add in the delay
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return false, err
		}

		// TODO: check contents of response
		resp, err := c.client.Do(req)

		if err != nil || resp.StatusCode != 200 {
			if err == nil {
				glog.V(5).Infof("Unable to GET health URL for master %s (at %s) on retry %v/%v: status: %v", masterAddress, url, retries+1, maxRetries, resp.Status)
			} else {
				glog.V(5).Infof("Unable to GET health URL for master %s (at %s) on retry %v/%v: error: %v", masterAddress, url, retries+1, maxRetries, err)
			}

			delay *= 2
			retries++
			time.Sleep(delay)
			continue
		}

		return true, nil
	}

	return false, nil
}

func NewHTTPSHealthzChecker(client *http.Client, path string) APIServerHealthChecker {
	return &httpsHealthzChecker{
		healthzPath: path,
		client:      client,
	}
}

type APIServerHealthCheckController struct {
	endpoints         unversioned_core.EndpointsInterface
	healthz           APIServerHealthChecker
	masterServiceName string
	eventRecorder     record.EventRecorder
}

func NewAPIServerHealthCheckController(endpoints unversioned_core.EndpointsInterface, healthzChecker APIServerHealthChecker, masterServiceName string, eventRecorder record.EventRecorder) *APIServerHealthCheckController {
	return &APIServerHealthCheckController{
		endpoints:         endpoints,
		healthz:           healthzChecker,
		masterServiceName: masterServiceName,
		eventRecorder:     eventRecorder,
	}
}

func (c *APIServerHealthCheckController) Run(syncPeriod time.Duration) {
	go wait.Until(func() {
		if err := c.CheckMasterHealth(); err != nil {
			glog.Errorf("Unable to check the health of the API servers: %v", err)
		}
	}, syncPeriod, wait.NeverStop)
}

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
