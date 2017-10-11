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

package scale

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"

	"github.com/stretchr/testify/assert"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	coretesting "k8s.io/client-go/testing"
)

func bytesBody(bodyBytes []byte) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(bodyBytes))
}

func defaultHeaders() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}

func fakeScaleClient() (ScalesGetter, []schema.GroupKind) {
	fakeDiscoveryClient := &fakedisco.FakeDiscovery{Fake: &coretesting.Fake{}}
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
				{Name: "replicationcontrollers/scale", Namespaced: true, Kind: "Scale", Group: "autoscaling", Version: "v1"},
			},
		},
		{
			GroupVersion: extv1beta1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
				{Name: "replicasets/scale", Namespaced: true, Kind: "Scale"},
			},
		},
		{
			GroupVersion: appsv1beta2.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				{Name: "deployments/scale", Namespaced: true, Kind: "Scale", Group: "autoscaling", Version: "v1"},
			},
		},
		// test a resource that doesn't exist anywere to make sure we're not accidentally depending
		// on a static RESTMapper anywhere.
		{
			GroupVersion: "cheese.testing.k8s.io/v27alpha15",
			APIResources: []metav1.APIResource{
				{Name: "cheddars", Namespaced: true, Kind: "Cheddar"},
				{Name: "cheddars/scale", Namespaced: true, Kind: "Scale", Group: "extensions", Version: "v1beta1"},
			},
		},
	}

	restMapperRes, err := discovery.GetAPIGroupResources(fakeDiscoveryClient)
	if err != nil {
		panic(fmt.Errorf("unexpected error while constructing resource list from fake discovery client: %v"))
	}
	restMapper := discovery.NewRESTMapper(restMapperRes, apimeta.InterfacesForUnstructured)

	autoscalingScale := &autoscalingv1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: autoscalingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: autoscalingv1.ScaleSpec{Replicas: 10},
		Status: autoscalingv1.ScaleStatus{
			Replicas: 10,
			Selector: "foo=bar",
		},
	}
	extScale := &extv1beta1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: extv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: extv1beta1.ScaleSpec{Replicas: 10},
		Status: extv1beta1.ScaleStatus{
			Replicas:       10,
			TargetSelector: "foo=bar",
		},
	}

	resourcePaths := map[string]runtime.Object{
		"/api/v1/namespaces/default/replicationcontrollers/foo/scale":                  autoscalingScale,
		"/apis/extensions/v1beta1/namespaces/default/replicasets/foo/scale":            extScale,
		"/apis/apps/v1beta2/namespaces/default/deployments/foo/scale":                  autoscalingScale,
		"/apis/cheese.testing.k8s.io/v27alpha15/namespaces/default/cheddars/foo/scale": extScale,
	}

	resolver := NewDiscoveryScaleKindResolver(fakeDiscoveryClient)
	cfg := &restclient.Config{
		Host: "localhost",
	}
	client := NewForConfig(cfg, restMapper, dynamic.LegacyAPIPathResolverFunc, resolver)

	fakeReqHandler := func(req *http.Request) (*http.Response, error) {
		scale, isScalePath := resourcePaths[req.URL.Path]
		if !isScalePath {
			return nil, fmt.Errorf("unexpected request for URL %q with method %q", req.URL.String(), req.Method)
		}

		switch req.Method {
		case "GET":
			res, err := json.Marshal(scale)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(res)}, nil
		case "PUT":
			decoder := codecs.UniversalDeserializer()
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			newScale, newScaleGVK, err := decoder.Decode(body, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("unexpected request body: %v", err)
			}
			if *newScaleGVK != scale.GetObjectKind().GroupVersionKind() {
				return nil, fmt.Errorf("unexpected scale API version %s (expected %s)", newScaleGVK.String(), scale.GetObjectKind().GroupVersionKind().String())
			}
			res, err := json.Marshal(newScale)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Header: defaultHeaders(), Body: bytesBody(res)}, nil
		default:
			return nil, fmt.Errorf("unexpected request for URL %q with method %q", req.URL.String(), req.Method)
		}
	}

	client.(*scaleClient).clientProvider = func(c *restclient.Config) (restclient.Interface, error) {
		var groupVersion schema.GroupVersion
		if c.GroupVersion != nil {
			groupVersion = *c.GroupVersion
		}
		_, versionedAPIPath, err := restclient.DefaultServerURL(c.Host, c.APIPath, groupVersion, false)
		if err != nil {
			return nil, err
		}
		return &fakerest.RESTClient{
			Client:               fakerest.CreateHTTPClient(fakeReqHandler),
			NegotiatedSerializer: c.NegotiatedSerializer,
			GroupVersion:         groupVersion,
			InternalGroupName:    groupVersion.Group,
			VersionedAPIPath:     versionedAPIPath,
		}, nil
	}

	groupKinds := []schema.GroupKind{
		{Group: corev1.GroupName, Kind: "ReplicationController"},
		{Group: extv1beta1.GroupName, Kind: "ReplicaSet"},
		{Group: appsv1beta2.GroupName, Kind: "Deployment"},
		{Group: "cheese.testing.k8s.io", Kind: "Cheddar"},
	}

	return client, groupKinds
}

func TestGetScale(t *testing.T) {
	scaleClient, groupKinds := fakeScaleClient()
	expectedScale := &autoscalingv1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: autoscalingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: autoscalingv1.ScaleSpec{Replicas: 10},
		Status: autoscalingv1.ScaleStatus{
			Replicas: 10,
			Selector: "foo=bar",
		},
	}

	for _, groupKind := range groupKinds {
		scale, err := scaleClient.Scales("default").Get(groupKind, "foo")
		if !assert.NoError(t, err, "should have been able to fetch a scale for %s", groupKind.String()) {
			continue
		}
		assert.NotNil(t, scale, "should have returned a non-nil scale for %s", groupKind.String())

		assert.Equal(t, expectedScale, scale, "should have returned the expected scale for %s", groupKind.String())
	}
}

func TestUpdateScale(t *testing.T) {
	scaleClient, groupKinds := fakeScaleClient()
	expectedScale := &autoscalingv1.Scale{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scale",
			APIVersion: autoscalingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: autoscalingv1.ScaleSpec{Replicas: 10},
		Status: autoscalingv1.ScaleStatus{
			Replicas: 10,
			Selector: "foo=bar",
		},
	}

	for _, groupKind := range groupKinds {
		scale, err := scaleClient.Scales("default").Update(groupKind, expectedScale)
		if !assert.NoError(t, err, "should have been able to fetch a scale for %s", groupKind.String()) {
			continue
		}
		assert.NotNil(t, scale, "should have returned a non-nil scale for %s", groupKind.String())

		assert.Equal(t, expectedScale, scale, "should have returned the expected scale for %s", groupKind.String())
	}
}
