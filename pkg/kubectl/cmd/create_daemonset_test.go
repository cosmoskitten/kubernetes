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

package cmd

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	"k8s.io/kubernetes/pkg/api"

	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func TestCreateDaemonSet(t *testing.T) {
	daeName := "jonny-dae"
	f, tf, _, ns := cmdtesting.NewAPIFactory()
	tf.Client = &fake.RESTClient{
		APIRegistry:          api.Registry,
		NegotiatedSerializer: ns,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte("{}"))),
			}, nil
		}),
	}
	tf.ClientConfig = &restclient.Config{}
	tf.Printer = &testPrinter{}
	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})

	cmd := NewCmdCreateDaemonset(f, buf)
	cmd.Flags().Set("dry-run", "true")
	cmd.Flags().Set("output", "name")
	cmd.Flags().Set("image", "hollywood/jonny.depp:v2")
	cmd.Run(cmd, []string{daeName})
	expectedOutput := "daemonset/" + daeName + "\n"
	if buf.String() != expectedOutput {
		t.Errorf("expected output: %s, but got: %s", expectedOutput, buf.String())
	}
}

func TestCreateDaemonsetNoImage(t *testing.T) {
	daeName := "jonny-dae"
	f, tf, _, ns := cmdtesting.NewAPIFactory()
	tf.Client = &fake.RESTClient{
		APIRegistry:          api.Registry,
		NegotiatedSerializer: ns,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(&bytes.Buffer{}),
			}, nil
		}),
	}
	tf.ClientConfig = &restclient.Config{}
	tf.Printer = &testPrinter{}
	tf.Namespace = "test"

	buf := bytes.NewBuffer([]byte{})
	cmd := NewCmdCreateDaemonset(f, buf)
	cmd.Flags().Set("dry-run", "true")
	cmd.Flags().Set("output", "name")
	err := CreateDaemonset(f, buf, cmd, []string{daeName})
	assert.Error(t, err, "at least one image must be specified")
}

func TestCreateDaemonSetV1Beta2(t *testing.T) {
	daeName := "jonny-dae"
	f, tf, _, ns := cmdtesting.NewAPIFactory()
	tf.Client = &fake.RESTClient{
		APIRegistry:          api.Registry,
		NegotiatedSerializer: ns,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte("{}"))),
			}, nil
		}),
	}
	tf.ClientConfig = &restclient.Config{}
	tf.Printer = &testPrinter{}
	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})

	cmd := NewCmdCreateDaemonset(f, buf)
	cmd.Flags().Set("dry-run", "true")
	cmd.Flags().Set("output", "name")
	cmd.Flags().Set("image", "hollywood/jonny.depp:v2")
	cmd.Flags().Set("generate", cmdutil.DaemonsetV1Beta2GeneratorName)
	cmd.Run(cmd, []string{daeName})
	expectedOutput := "daemonset/" + daeName + "\n"
	if buf.String() != expectedOutput {
		t.Errorf("expected output: %s, but got: %s", expectedOutput, buf.String())
	}
}
