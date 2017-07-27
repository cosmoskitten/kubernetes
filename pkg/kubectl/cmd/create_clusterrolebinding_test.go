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


	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
)

var clusterRoleBindingGroupVersion = schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1alpha1"}

func TestClusterRoleBinding(t *testing.T) {
	clusterRoleBinding :=&rbac.ClusterRoleBinding{}
	clusterRoleBinding.Name = "my-clusterrole"
	f, tf, _, ns := cmdtesting.NewAPIFactory()

    tf.Printer = &testPrinter{}
	info, _ := runtime.SerializerInfoForMediaType(ns.SupportedMediaTypes(), runtime.ContentTypeJSON)
	encoder := ns.EncoderForVersion(info.Serializer, clusterRoleBindingGroupVersion)

	tf.Client = &ClusterRoleBindingRESTClient{
		RESTClient: &fake.RESTClient{
		APIRegistry:          api.Registry,
		NegotiatedSerializer: ns,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			responseBinding := &rbac.ClusterRoleBinding{}
			responseBinding.Name = "fake-binding"
			return &http.Response{StatusCode: 201, Header: defaultHeader(), Body: ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(encoder, responseBinding))))}, nil
		}),
		},
	}

	buf := bytes.NewBuffer([]byte{})
	cmd := NewCmdCreateClusterRoleBinding(f,buf)
	cmd.Flags().Set("output","name")
	cmd.Flags().Set("clusterrole","cluster-admin")
	cmd.Run(cmd,[]string{clusterRoleBinding.Name})
	expectedOutput := "clusterrolebinding/" +  clusterRoleBinding.Name + "\n"
    if buf.String() != expectedOutput {
		t.Errorf("expected output: %s, but got: %s", expectedOutput, buf.String())
	}
}

type ClusterRoleBindingRESTClient struct {
	*fake.RESTClient
}

func (c *ClusterRoleBindingRESTClient) Post() *restclient.Request {
	config := restclient.ContentConfig{
		ContentType:          runtime.ContentTypeJSON,
		GroupVersion:         &clusterRoleBindingGroupVersion,
		NegotiatedSerializer: c.NegotiatedSerializer,
	}

	info, _ := runtime.SerializerInfoForMediaType(c.NegotiatedSerializer.SupportedMediaTypes(), runtime.ContentTypeJSON)
	serializers := restclient.Serializers{
		Encoder: c.NegotiatedSerializer.EncoderForVersion(info.Serializer, clusterRoleBindingGroupVersion),
		Decoder: c.NegotiatedSerializer.DecoderToVersion(info.Serializer, clusterRoleBindingGroupVersion),
	}
	if info.StreamSerializer != nil {
		serializers.StreamingSerializer = info.StreamSerializer.Serializer
		serializers.Framer = info.StreamSerializer.Framer
	}
	return restclient.NewRequest(c, "POST", &url.URL{Host: "localhost"}, c.VersionedAPIPath, config, serializers, nil, nil)
}

