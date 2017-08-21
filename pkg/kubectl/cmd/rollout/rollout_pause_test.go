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

package rollout

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	"k8s.io/kubernetes/pkg/api"
	extensions "k8s.io/kubernetes/pkg/apis/extensions"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
)

var rolloutPauseGroupVersionEncoder = schema.GroupVersion{Group: "extensions", Version: "v1beta1"}
var rolloutPauseGroupVersionDecoder = schema.GroupVersion{Group: "extensions", Version: runtime.APIVersionInternal}

func TestRolloutPause(t *testing.T) {
	deploymentName := "deployment/nginx-deployment"
	f, tf, _, ns := cmdtesting.NewAPIFactory()
	info, _ := runtime.SerializerInfoForMediaType(ns.SupportedMediaTypes(), runtime.ContentTypeJSON)
	encoder := ns.EncoderForVersion(info.Serializer, rolloutPauseGroupVersionEncoder)

	tf.Client = &RolloutPauseRESTClient{
		RESTClient: &fake.RESTClient{
			APIRegistry:          api.Registry,
			NegotiatedSerializer: ns,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				responseDeployment := &extensions.Deployment{}
				responseDeployment.Name = deploymentName
				body := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(encoder, responseDeployment))))
				return &http.Response{StatusCode: http.StatusOK, Header: defaultHeader(), Body: body}, nil
			}),
		},
	}

	tf.Namespace = "test"
	buf := bytes.NewBuffer([]byte{})
	cmd := NewCmdRolloutPause(f, buf)

	cmd.Run(cmd, []string{deploymentName})
	expectedOutput := "deployment \"" + deploymentName + "\" paused\n"
	if buf.String() != expectedOutput {
		t.Errorf("expected output: %s, but got: %s", expectedOutput, buf.String())
	}
}

type RolloutPauseRESTClient struct {
	*fake.RESTClient
}

func (c *RolloutPauseRESTClient) Get() *restclient.Request {
	config := restclient.ContentConfig{
		ContentType:          runtime.ContentTypeJSON,
		GroupVersion:         &rolloutPauseGroupVersionEncoder,
		NegotiatedSerializer: c.NegotiatedSerializer,
	}

	info, _ := runtime.SerializerInfoForMediaType(c.NegotiatedSerializer.SupportedMediaTypes(), runtime.ContentTypeJSON)
	serializers := restclient.Serializers{
		Encoder: c.NegotiatedSerializer.EncoderForVersion(info.Serializer, rolloutPauseGroupVersionEncoder),
		Decoder: c.NegotiatedSerializer.DecoderToVersion(info.Serializer, rolloutPauseGroupVersionDecoder),
	}
	if info.StreamSerializer != nil {
		serializers.StreamingSerializer = info.StreamSerializer.Serializer
		serializers.Framer = info.StreamSerializer.Framer
	}
	return restclient.NewRequest(c, "GET", &url.URL{Host: "localhost"}, c.VersionedAPIPath, config, serializers, nil, nil)
}

func (c *RolloutPauseRESTClient) Patch(pt types.PatchType) *restclient.Request {
	config := restclient.ContentConfig{
		ContentType:          runtime.ContentTypeJSON,
		GroupVersion:         &rolloutPauseGroupVersionEncoder,
		NegotiatedSerializer: c.NegotiatedSerializer,
	}

	info, _ := runtime.SerializerInfoForMediaType(c.NegotiatedSerializer.SupportedMediaTypes(), runtime.ContentTypeJSON)
	serializers := restclient.Serializers{
		Encoder: c.NegotiatedSerializer.EncoderForVersion(info.Serializer, rolloutPauseGroupVersionEncoder),
		Decoder: c.NegotiatedSerializer.DecoderToVersion(info.Serializer, rolloutPauseGroupVersionDecoder),
	}
	if info.StreamSerializer != nil {
		serializers.StreamingSerializer = info.StreamSerializer.Serializer
		serializers.Framer = info.StreamSerializer.Framer
	}
	return restclient.NewRequest(c, "PATCH", &url.URL{Host: "localhost"}, c.VersionedAPIPath, config, serializers, nil, nil)
}

func defaultHeader() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}
