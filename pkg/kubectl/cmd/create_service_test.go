/*
Copyright 2016 The Kubernetes Authors.

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

package cmd

import (
	"bytes"
	"net/http"
	"testing"

	"k8s.io/client-go/rest/fake"
	"k8s.io/kubernetes/pkg/api"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
)

var tests = []struct {
	name      string
	flag      string
	value     string
	expected  string
	expectErr bool
}{
	{
		name:      "TCP-single-port-valid",
		flag:      "tcp",
		value:     "8080",
		expected:  "service/TCP-single-port-valid" + "\n",
		expectErr: false,
	},
	{
		name:      "TCP-single-port-invalid",
		flag:      "tcp",
		value:     "65536",
		expected:  "service/TCP-single-port-invalid" + "\n",
		expectErr: true,
	},
	{
		name:      "TCP-multi-port-valid",
		flag:      "tcp",
		value:     "8080:8080",
		expected:  "service/TCP-multi-port-valid" + "\n",
		expectErr: false,
	},
	{
		name:      "TCP-multi-port-invalid",
		flag:      "tcp",
		value:     "8080:65536",
		expected:  "service/TCP-multi-port-invalid" + "\n",
		expectErr: true,
	},
	{
		name:      "mismatching-name",
		flag:      "tcp",
		value:     "8080:8080",
		expected:  "service/mismatching" + "\n",
		expectErr: true,
	},
}

func TestCreateService(t *testing.T) {
	for index, test := range tests {
		service := &api.Service{}
		service.Name = test.name
		f, tf, codec, negSer := cmdtesting.NewAPIFactory()
		tf.Printer = &testPrinter{}
		tf.Client = &fake.RESTClient{
			APIRegistry:          api.Registry,
			NegotiatedSerializer: negSer,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/namespaces/test/services" && m == "POST":
					return &http.Response{StatusCode: 201, Header: defaultHeader(), Body: objBody(codec, service)}, nil
				default:
					t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
					return nil, nil
				}
			}),
		}
		tf.Namespace = "test"
		buf := bytes.NewBuffer([]byte{})
		cmd := NewCmdCreateServiceClusterIP(f, buf)
		cmd.Flags().Set("output", "name")
		cmd.Flags().Set(test.flag, test.value)
		cmd.Run(cmd, []string{service.Name})
		testPass := buf.String() == test.expected
		if test.expectErr && !testPass {
			continue
		}
		if test.expectErr && testPass {
			t.Errorf("testdata index: %d, this test must fail but it passed", index)
		}
		if !testPass {
			t.Errorf("testdata index: %d, expected output: %s, but got: %s", index, test.expected, buf.String())
		}
	}
}

func TestCreateServiceNodePort(t *testing.T) {
	for index, test := range tests {
		service := &api.Service{}
		service.Name = test.name
		f, tf, codec, negSer := cmdtesting.NewAPIFactory()
		tf.Printer = &testPrinter{}
		tf.Client = &fake.RESTClient{
			APIRegistry:          api.Registry,
			NegotiatedSerializer: negSer,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/namespaces/test/services" && m == http.MethodPost:
					return &http.Response{StatusCode: http.StatusCreated, Header: defaultHeader(), Body: objBody(codec, service)}, nil
				default:
					t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
					return nil, nil
				}
			}),
		}
		tf.Namespace = "test"
		buf := bytes.NewBuffer([]byte{})
		cmd := NewCmdCreateServiceNodePort(f, buf)
		cmd.Flags().Set("output", "name")
		cmd.Flags().Set(test.flag, test.value)
		cmd.Run(cmd, []string{service.Name})
		testPass := buf.String() == test.expected
		if test.expectErr && !testPass {
			continue
		}
		if test.expectErr && testPass {
			t.Errorf("testdata index: %d, this test must fail but it passed", index)
		}
		if !testPass {
			t.Errorf("testdata index: %d, expected output: %s, but got: %s", index, test.expected, buf.String())
		}
	}
}

func TestCreateServiceExternalName(t *testing.T) {
	service := &api.Service{}
	service.Name = "my-external-name-service"
	f, tf, codec, negSer := cmdtesting.NewAPIFactory()
	tf.Printer = &testPrinter{}
	tf.Client = &fake.RESTClient{
		APIRegistry:          api.Registry,
		NegotiatedSerializer: negSer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/namespaces/test/services" && m == http.MethodPost:
				return &http.Response{StatusCode: http.StatusCreated, Header: defaultHeader(), Body: objBody(codec, service)}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})
	cmd := NewCmdCreateServiceExternalName(f, buf)
	cmd.Flags().Set("output", "name")
	cmd.Flags().Set("external-name", "name")
	cmd.Run(cmd, []string{service.Name})
	expectedOutput := "service/" + service.Name + "\n"
	if buf.String() != expectedOutput {
		t.Errorf("expected output: %s, but got: %s", expectedOutput, buf.String())
	}
}
