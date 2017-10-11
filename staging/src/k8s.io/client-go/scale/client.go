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
	"fmt"
	"sync"

	autoscaling "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
)

var scaleConverter = NewScaleConverter()
var codecs = serializer.NewCodecFactory(scaleConverter.Scheme())

// restInterfaceProvider turns a restclient.Config into a restclient.Interface.
// It's overridable for the purposes of testing.
type restInterfaceProvider func(*restclient.Config) (restclient.Interface, error)

// scaleClient is an implementation of ScalesGetter
// which makes use of a RESTMapper and a generic REST
// client to support an discoverable resource.
// It behaves somewhat similarly to the dynamic ClientPool,
// but is more specifically scoped to Scale.
type scaleClient struct {
	mapper meta.RESTMapper
	config *restclient.Config

	clients             map[schema.GroupVersion]restclient.Interface
	clientsMu           sync.Mutex
	apiPathResolverFunc dynamic.APIPathResolverFunc
	scaleKindResolver   ScaleKindResolver

	clientProvider restInterfaceProvider
}

// NewForConfig creates a new ScalesGetter which resolves kinds
// to resources using the given RESTMapper, and API paths using
// the given dynamic.APIPathResolverFunc.
func NewForConfig(cfg *restclient.Config, mapper meta.RESTMapper, resolver dynamic.APIPathResolverFunc, scaleKindResolver ScaleKindResolver) ScalesGetter {
	return &scaleClient{
		config: cfg,
		mapper: mapper,

		clients:             make(map[schema.GroupVersion]restclient.Interface),
		apiPathResolverFunc: resolver,
		scaleKindResolver:   scaleKindResolver,
		clientProvider: func(c *restclient.Config) (restclient.Interface, error) {
			return restclient.RESTClientFor(c)
		},
	}
}

// clientAndMappingFor returns a cached rest client and the associated resource for the given GroupKind,
// populating the cache if necessary.
func (c *scaleClient) clientAndMappingFor(kind schema.GroupKind) (restclient.Interface, *meta.RESTMapping, error) {
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	mapping, err := c.mapper.RESTMapping(kind)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get rest mapping for %s: %v", kind.String(), err)
	}

	groupVer := mapping.GroupVersionKind.GroupVersion()

	// check if we have an existing client
	if existingClient, found := c.clients[groupVer]; found {
		return existingClient, mapping, nil
	}

	// avoid changing the original config
	confCopy := *c.config
	conf := &confCopy

	// we need to set the API path based on GroupVersion (defaulting to the legacy path if none is set)
	conf.APIPath = c.apiPathResolverFunc(mapping.GroupVersionKind)
	if conf.APIPath == "" {
		conf.APIPath = "/api"
	}
	// NB: the GV here is used to set the API path, but has little relevance to the
	// serialized group-versions
	conf.GroupVersion = &groupVer
	conf.NegotiatedSerializer = serializer.DirectCodecFactory{
		CodecFactory: codecs,
	}

	if len(conf.UserAgent) == 0 {
		conf.UserAgent = restclient.DefaultKubernetesUserAgent()
	}

	cl, err := c.clientProvider(conf)
	if err != nil {
		return nil, nil, err
	}

	c.clients[groupVer] = cl

	return cl, mapping, nil
}

// namespacedScaleClient is an ScaleInterface for fetching
// Scales in a given namespace.
type namespacedScaleClient struct {
	client    *scaleClient
	namespace string
}

func (c *scaleClient) Scales(namespace string) ScaleInterface {
	return &namespacedScaleClient{
		client:    c,
		namespace: namespace,
	}
}

func (c *namespacedScaleClient) Get(kind schema.GroupKind, name string) (*autoscaling.Scale, error) {
	// Currently, a /scale endpoint can return different scale types.
	// Until we have support for the alternative API representations proposal,
	// we need to deal with accepting different API versions.
	// In practice, this is autoscaling/v1.Scale and extensions/v1beta1.Scale

	client, mapping, err := c.client.clientAndMappingFor(kind)
	if err != nil {
		return nil, fmt.Errorf("unable to get client for %s: %v", kind.String(), err)
	}

	rawObj, err := client.Get().
		Namespace(c.namespace).
		Resource(mapping.Resource).
		Name(name).
		SubResource("scale").
		Do().
		Get()

	if err != nil {
		return nil, err
	}

	// convert whatever this is to autoscaling/v1.Scale
	scaleObj, err := scaleConverter.ConvertToVersion(rawObj, autoscaling.SchemeGroupVersion)
	if err != nil {
		return nil, fmt.Errorf("received an object from a /scale endpoint which was not convertible to autoscaling Scale: %v", err)
	}

	return scaleObj.(*autoscaling.Scale), nil
}

func (c *namespacedScaleClient) Update(kind schema.GroupKind, scale *autoscaling.Scale) (*autoscaling.Scale, error) {
	client, mapping, err := c.client.clientAndMappingFor(kind)
	if err != nil {
		return nil, fmt.Errorf("unable to get client for %s: %v", kind.String(), err)
	}

	// Currently, a /scale endpoint can receive and return different scale types.
	// Until we hvae support for the alternative API representations proposal,
	// we need to deal with sending and accepting differnet API versions.

	// figure out what scale we actually need here
	baseGVR := mapping.GroupVersionKind.GroupVersion().WithResource(mapping.Resource)
	desiredGVK, err := c.client.scaleKindResolver.ScaleForResource(baseGVR)
	if err != nil {
		return nil, fmt.Errorf("could not find proper group-version for scale subresource of %s: %v", kind.String(), err)
	}

	// convert this to whatever this endpoint wants
	scaleUpdate, err := scaleConverter.ConvertToVersion(scale, desiredGVK.GroupVersion())
	if err != nil {
		return nil, fmt.Errorf("could not convert scale update to internal Scale: %v", err)
	}

	rawObj, err := client.Put().
		Namespace(c.namespace).
		Resource(mapping.Resource).
		Name(scale.Name).
		SubResource("scale").
		Body(scaleUpdate).
		Do().
		Get()

	// convert whatever this is back to autoscaling/v1.Scale
	scaleObj, err := scaleConverter.ConvertToVersion(rawObj, autoscaling.SchemeGroupVersion)
	if err != nil {
		return nil, fmt.Errorf("received an object from a /scale endpoint which was not convertible to autoscaling Scale: %v", err)
	}

	return scaleObj.(*autoscaling.Scale), err
}
