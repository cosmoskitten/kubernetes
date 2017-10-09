/*
Copyright 2014 The Kubernetes Authors.

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

package resource

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/watch"
)

// Selector is a Visitor for resources that match a label selector.
type Selector struct {
	Client               RESTClient
	Mapping              *meta.RESTMapping
	Namespace            string
	LabelSelector        string
	FieldSelector        string
	Export               bool
	IncludeUninitialized bool
}

// NewSelector creates a resource selector which hides details of getting items by their label selector.
func NewSelector(client RESTClient, mapping *meta.RESTMapping, namespace, labelSelector, fieldSelector string, export, includeUninitialized bool) *Selector {
	return &Selector{
		Client:               client,
		Mapping:              mapping,
		Namespace:            namespace,
		LabelSelector:        labelSelector,
		FieldSelector:        fieldSelector,
		Export:               export,
		IncludeUninitialized: includeUninitialized,
	}
}

// Visit implements Visitor
func (r *Selector) Visit(fn VisitorFunc) error {
	list, err := NewHelper(r.Client, r.Mapping).List(r.Namespace, r.ResourceMapping().GroupVersionKind.GroupVersion().String(), r.LabelSelector, r.FieldSelector, r.Export, r.IncludeUninitialized)
	if err != nil {
		if errors.IsBadRequest(err) || errors.IsNotFound(err) {
			if se, ok := err.(*errors.StatusError); ok {
				// modify the message without hiding this is an API error
				if len(r.LabelSelector) == 0 && len(r.FieldSelector) == 0 {
					se.ErrStatus.Message = fmt.Sprintf("Unable to list %q: %v", r.Mapping.Resource, se.ErrStatus.Message)
				} else {
					se.ErrStatus.Message = fmt.Sprintf("Unable to find %q that match the selector %q, %q: %v", r.Mapping.Resource, r.LabelSelector, r.FieldSelector, se.ErrStatus.Message)
				}
				return se
			}
			if len(r.LabelSelector) == 0 && len(r.FieldSelector) == 0 {
				return fmt.Errorf("Unable to list %q: %v", r.Mapping.Resource, err)
			} else {
				return fmt.Errorf("Unable to find %q that match the selector %q, %q: %v", r.Mapping.Resource, r.LabelSelector, r.FieldSelector, err)
			}
		}
		return err
	}
	accessor := r.Mapping.MetadataAccessor
	resourceVersion, _ := accessor.ResourceVersion(list)
	info := &Info{
		Client:    r.Client,
		Mapping:   r.Mapping,
		Namespace: r.Namespace,

		Object:          list,
		ResourceVersion: resourceVersion,
	}
	return fn(info, nil)
}

func (r *Selector) Watch(resourceVersion string) (watch.Interface, error) {
	return NewHelper(r.Client, r.Mapping).Watch(r.Namespace, resourceVersion, r.ResourceMapping().GroupVersionKind.GroupVersion().String(), r.LabelSelector, r.FieldSelector)
}

// ResourceMapping returns the mapping for this resource and implements ResourceMapping
func (r *Selector) ResourceMapping() *meta.RESTMapping {
	return r.Mapping
}
