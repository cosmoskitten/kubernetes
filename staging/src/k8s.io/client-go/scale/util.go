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
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	scalescheme "k8s.io/client-go/scale/scheme"
	scaleautoscaling "k8s.io/client-go/scale/scheme/autoscalingv1"
	scaleextint "k8s.io/client-go/scale/scheme/extensionsint"
	scaleext "k8s.io/client-go/scale/scheme/extensionsv1beta1"
)

// ScaleKindResolver knows about the relationship between
// resources and the GroupVersionKind of their scale subresources.
type ScaleKindResolver interface {
	// ScaleForResource returns the GroupVersionKind of the
	// scale subresource for the given GroupVersionResource.
	ScaleForResource(resource schema.GroupVersionResource) (scaleVersion schema.GroupVersionKind, err error)
}

// discoveryScaleResolver is a ScaleKindResolver that uses
// a DiscoveryInterface to associate resources with their
// scale-kinds
type discoveryScaleResolver struct {
	discoveryClient discovery.ServerResourcesInterface

	subresMap map[schema.GroupVersionResource]schema.GroupVersion
	mu        sync.RWMutex
}

func (r *discoveryScaleResolver) generateKindMap() error {
	resourceLists, err := r.discoveryClient.ServerResources()
	if err != nil {
		return fmt.Errorf("unable to update scale kinds: %v")
	}

	subresMap := make(map[schema.GroupVersionResource]schema.GroupVersion)

	// first, find the scale subresources that we care about
	for _, resourceList := range resourceLists {
		groupVer, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to update scale kinds for API group %s: %v", groupVer.String(), err))
			continue
		}

		for _, resource := range resourceList.APIResources {
			resourceParts := strings.SplitN(resource.Name, "/", 2)
			if resource.Kind != "Scale" || len(resourceParts) != 2 || resourceParts[1] != "scale" {
				// skip non-scale resources
				continue
			}

			mainResource := groupVer.WithResource(resourceParts[0])
			scaleGV := groupVer
			if resource.Group != "" && resource.Version != "" {
				scaleGV = schema.GroupVersion{
					Group:   resource.Group,
					Version: resource.Version,
				}
			}

			subresMap[mainResource] = scaleGV
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.subresMap = subresMap

	return nil
}

func (r *discoveryScaleResolver) ScaleForResource(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	r.mu.RLock()
	gv, exists := r.subresMap[resource]
	if !exists {
		// retry on misses in case we have an update
		r.mu.RUnlock()
		r.generateKindMap()
		r.mu.RLock()
		gv, exists = r.subresMap[resource]
	}
	defer r.mu.RUnlock()
	if !exists {
		return schema.GroupVersionKind{}, fmt.Errorf("resource %s has no known scale subresource", resource.String())
	}

	return gv.WithKind("Scale"), nil
}

func NewDiscoveryScaleKindResolver(client discovery.ServerResourcesInterface) ScaleKindResolver {
	return &discoveryScaleResolver{
		discoveryClient: client,
		subresMap:       make(map[schema.GroupVersionResource]schema.GroupVersion),
	}
}

// ScaleConverter knows how to convert between external scale versions.
type ScaleConverter struct {
	scheme            *runtime.Scheme
	internalVersioner runtime.GroupVersioner
}

func NewScaleConverter() *ScaleConverter {
	scheme := runtime.NewScheme()
	scaleautoscaling.AddToScheme(scheme)
	scalescheme.AddToScheme(scheme)
	scaleext.AddToScheme(scheme)
	scaleextint.AddToScheme(scheme)

	return &ScaleConverter{
		scheme: scheme,
		internalVersioner: runtime.NewMultiGroupVersioner(
			scalescheme.SchemeGroupVersion,
			schema.GroupKind{Group: scaleext.GroupName, Kind: "Scale"},
			schema.GroupKind{Group: scaleautoscaling.GroupName, Kind: "Scale"},
		),
	}
}

// Scheme returns the scheme used by this scale converter.
func (c *ScaleConverter) Scheme() *runtime.Scheme {
	return c.scheme
}

// ConvertToVersion converts the given *external* input object to the given output *external* output group-version.
func (c *ScaleConverter) ConvertToVersion(in runtime.Object, outVersion schema.GroupVersion) (runtime.Object, error) {
	scaleInt, err := c.scheme.ConvertToVersion(in, c.internalVersioner)
	if err != nil {
		return nil, err
	}

	return c.scheme.ConvertToVersion(scaleInt, outVersion)
}
