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
	"net/url"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	"k8s.io/kubernetes/pkg/api"
	extensions "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var scaleGroupVersionEncoder = schema.GroupVersion{Group: "extensions", Version: "v1beta1"}
var scaleGroupVersionDecoder = schema.GroupVersion{Group: "extensions", Version: runtime.APIVersionInternal}

func TestScale(t *testing.T) {
	deploymentName := "test-deployment"
	f, tf, _, ns := cmdtesting.NewAPIFactory()
	info, _ := runtime.SerializerInfoForMediaType(ns.SupportedMediaTypes(), runtime.ContentTypeJSON)
	encoder := ns.EncoderForVersion(info.Serializer, scaleGroupVersionEncoder)

	tf.Client = &ScaleRESTClient{
		RESTClient: &fake.RESTClient{
			APIRegistry:          api.Registry,
			NegotiatedSerializer: ns,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				responseDeployment := &extensions.Deployment{}
				responseDeployment.Name = deploymentName
				responseDeployment.Spec.Replicas = 4

				body := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(encoder, responseDeployment))))
				return &http.Response{StatusCode: http.StatusOK, Header: defaultResumeHeader(), Body: body}, nil
			}),
		},
	}

	fakeScaler := &fakeScaler{}
	fake := &fakeScalerFactory{Factory: f, scaler: fakeScaler}

	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})
	cmd := NewCmdScale(fake, buf)
	cmd.Flags().Set("replicas", "3")
	cmd.Run(cmd, []string{"deployment/" + deploymentName})

	expectedOutput := "deployment \"" + deploymentName + "\" scaled\n"
	if buf.String() != expectedOutput {
		t.Errorf("expected output: %s, but got: %s", expectedOutput, buf.String())
	}
}

type ScaleRESTClient struct {
	*fake.RESTClient
}

func (c *ScaleRESTClient) Get() *restclient.Request {
	config := restclient.ContentConfig{
		ContentType:          runtime.ContentTypeJSON,
		GroupVersion:         &scaleGroupVersionEncoder,
		NegotiatedSerializer: c.NegotiatedSerializer,
	}

	info, _ := runtime.SerializerInfoForMediaType(c.NegotiatedSerializer.SupportedMediaTypes(), runtime.ContentTypeJSON)
	serializers := restclient.Serializers{
		Encoder: c.NegotiatedSerializer.EncoderForVersion(info.Serializer, scaleGroupVersionEncoder),
		Decoder: c.NegotiatedSerializer.DecoderToVersion(info.Serializer, scaleGroupVersionDecoder),
	}
	if info.StreamSerializer != nil {
		serializers.StreamingSerializer = info.StreamSerializer.Serializer
		serializers.Framer = info.StreamSerializer.Framer
	}
	return restclient.NewRequest(c, "GET", &url.URL{Host: "localhost"}, c.VersionedAPIPath, config, serializers, nil, nil)
}

func defaultResumeHeader() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}

type fakeScaler struct{}

func (s *fakeScaler) Scale(namespace, name string, newSize uint, preconditions *kubectl.ScalePrecondition, retry, wait *kubectl.RetryParams) error {
	return nil
}

func (s *fakeScaler) ScaleSimple(namespace, name string, preconditions *kubectl.ScalePrecondition, newSize uint) (updatedResourceVersion string, err error) {
	return "", nil
}

type fakeScalerFactory struct {
	cmdutil.Factory
	scaler kubectl.Scaler
}

func (f *fakeScalerFactory) Scaler(*meta.RESTMapping) (kubectl.Scaler, error) {
	return f.scaler, nil
}
