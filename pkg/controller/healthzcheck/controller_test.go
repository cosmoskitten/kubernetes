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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/client/testing/fake"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
)

type fakeHealthClient struct {
	validAddrs map[string]bool
}

func (c *fakeHealthClient) Check(masterAddress string, masterPorts []api.EndpointPort) (bool, error) {
	if _, ok := c.validAddrs[masterAddress]; ok {
		return true, nil
	}

	return false, nil
}

func TestHTTPSHealthzChecker(t *testing.T) {
	httpServer := httptest.NewTLSServer(http.HandlerFunc(func(respWriter http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/healthz" {
			fmt.Fprintln(respWriter, "ok")
			return
		}

		respWriter.WriteHeader(http.StatusNotFound)
	}))
	defer httpServer.Close()

	serverURL, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serverHost, serverPortStr, err := net.SplitHostPort(serverURL.Host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serverPort, err := strconv.Atoi(serverPortStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ports := []api.EndpointPort{
		{
			Name: "other",
			Port: 8675,
		},
		{
			Name:     "https",
			Port:     8675,
			Protocol: api.ProtocolUDP,
		},
		{
			Name:     "https",
			Port:     serverPort,
			Protocol: api.ProtocolTCP,
		},
	}

	insecureTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpsClient := &http.Client{Transport: insecureTransport}

	checker := NewHTTPSHealthzChecker(httpsClient, "healthz")
	if ok, err := checker.Check(serverHost, ports); err != nil {
		t.Errorf("expected check to succeed on valid healthz server, but got error: %v", err)
	} else if !ok {
		t.Errorf("expected check to succeed on valid healthz server, but it failed")
	}

	checker = NewHTTPSHealthzChecker(httpsClient, "somethingelse")
	if ok, err := checker.Check(serverHost, ports); err != nil {
		t.Errorf("expected check to fail on a 404 status code, but got error: %v", err)
	} else if ok {
		t.Errorf("expected check to fail on a 404 status code, but it succeeded")
	}

	// close the HTTP server
	httpServer.Close()

	checker = NewHTTPSHealthzChecker(httpsClient, "healthz")
	if ok, err := checker.Check(serverHost, ports); err != nil {
		t.Errorf("expected check to fail on a non-existant HTTP server, but got error: %v", err)
	} else if ok {
		t.Errorf("expected check to fail on a non-existant HTTP server, but it succeeded")
	}

}

func TestMasterHealthCheckController(t *testing.T) {
	masterServiceName := "test-kube-master"
	masterServiceNamespace := "test-master-ns"

	initialEndpoints := &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			Name:      masterServiceName,
			Namespace: masterServiceNamespace,
		},
		Subsets: []api.EndpointSubset{
			{
				Addresses: []api.EndpointAddress{
					{
						IP: "172.30.0.1",
					},
					{
						IP: "172.30.0.2",
					},
					{
						IP: "172.30.0.3",
					},
				},
				Ports: []api.EndpointPort{
					{
						Name:     "https",
						Port:     8443,
						Protocol: api.ProtocolTCP,
					},
				},
			},
		},
	}

	finalEndpoints := &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			Name:      masterServiceName,
			Namespace: masterServiceNamespace,
		},
		Subsets: []api.EndpointSubset{
			{
				Addresses: []api.EndpointAddress{
					{
						IP: "172.30.0.1",
					},
					{
						IP: "172.30.0.3",
					},
				},
				Ports: []api.EndpointPort{
					{
						Name:     "https",
						Port:     8443,
						Protocol: api.ProtocolTCP,
					},
				},
			},
		},
	}

	kc := &fake.Clientset{}
	kc.AddReactor("get", "endpoints", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(testclient.GetAction)
		if getAction.GetName() != masterServiceName || getAction.GetNamespace() != masterServiceNamespace {
			return false, nil, nil
		}

		return true, initialEndpoints, nil
	})

	updatedEndpoints := make(map[string]api.Endpoints)
	kc.AddReactor("update", "endpoints", func(action core.Action) (bool, runtime.Object, error) {
		updateAction := action.(testclient.UpdateAction)
		ep := updateAction.GetObject().(*api.Endpoints)
		updatedEndpoints[updateAction.GetNamespace()+"/"+ep.Name] = *ep
		return true, ep, nil
	})

	healthClient := &fakeHealthClient{validAddrs: map[string]bool{"172.30.0.1": true, "172.30.0.3": true}}
	healthController := NewAPIServerHealthCheckController(kc.Core().Endpoints(masterServiceNamespace), healthClient, masterServiceName, &record.FakeRecorder{})

	if err := healthController.CheckMasterHealth(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if eps, ok := updatedEndpoints[masterServiceNamespace+"/"+masterServiceName]; ok {
		if !api.Semantic.DeepEqual(*finalEndpoints, eps) {
			t.Errorf("actual updated endpoints did not match expected updated endpoints: %v", util.ObjectDiff(finalEndpoints, eps))
		}
	} else {
		t.Errorf("controller did not update endpoints for %s/%s at all", masterServiceNamespace, masterServiceName, updatedEndpoints)
	}
}
