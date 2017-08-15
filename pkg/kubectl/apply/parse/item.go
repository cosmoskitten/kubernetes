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

package parse

import (
	"k8s.io/kubernetes/pkg/kubectl/apply"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
)

// Item wraps values from 3 sources (recorded, local, remote).
// The values are not collated
type Item interface {
	Accept(ItemVisitor) (apply.Element, error)
}

// PrimitiveItem contains a recorded, local, and remote value
type PrimitiveItem struct {
	Name      string
	Primitive *openapi.Primitive

	RecordedSet bool
	LocalSet    bool
	RemoteSet   bool

	Recorded interface{}
	Local    interface{}
	Remote   interface{}
}

func (i *PrimitiveItem) Accept(v ItemVisitor) (apply.Element, error) {
	return v.VisitPrimitive(i)
}

func (i *PrimitiveItem) GetMeta() openapi.Schema {
	// https://golang.org/doc/faq#nil_error
	if i.Primitive != nil {
		return i.Primitive
	}
	return nil
}

// ListItem contains a recorded, local, and remote list
type ListItem struct {
	Name  string
	Array *openapi.Array

	RecordedSet bool
	LocalSet    bool
	RemoteSet   bool

	Recorded []interface{}
	Local    []interface{}
	Remote   []interface{}
}

func (i *ListItem) Accept(v ItemVisitor) (apply.Element, error) {
	return v.VisitList(i)
}

func (i *ListItem) GetMeta() openapi.Schema {
	// https://golang.org/doc/faq#nil_error
	if i.Array != nil {
		return i.Array
	}
	return nil
}

// MapItem contains a recorded, local, and remote map
type MapItem struct {
	Name string
	Map  *openapi.Map

	RecordedSet bool
	LocalSet    bool
	RemoteSet   bool

	Recorded map[string]interface{}
	Local    map[string]interface{}
	Remote   map[string]interface{}
}

func (i *MapItem) Accept(v ItemVisitor) (apply.Element, error) {
	return v.VisitMap(i)
}

func (i *MapItem) GetMeta() openapi.Schema {
	// https://golang.org/doc/faq#nil_error
	if i.Map != nil {
		return i.Map
	}
	return nil
}

// MapItem contains a recorded, local, and remote map
type TypeItem struct {
	Name string
	Type *openapi.Kind

	RecordedSet bool
	LocalSet    bool
	RemoteSet   bool

	Recorded map[string]interface{}
	Local    map[string]interface{}
	Remote   map[string]interface{}
}

func (i *TypeItem) GetMeta() openapi.Schema {
	// https://golang.org/doc/faq#nil_error
	if i.Type != nil {
		return i.Type
	}
	return nil
}

func (i *TypeItem) Accept(v ItemVisitor) (apply.Element, error) {
	return v.VisitType(i)
}

// EmptyItem contains no values
type EmptyItem struct {
	Name string
}

func (i *EmptyItem) Accept(v ItemVisitor) (apply.Element, error) {
	return &apply.EmptyElement{
		Name: i.Name,
	}, nil
}
