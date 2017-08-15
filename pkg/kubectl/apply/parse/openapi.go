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
	"fmt"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
)

// Contains functions for casting openapi interfaces to their underlying types

// GetSchemaType returns the string type of the schema - e.g. array, primitive, map, kind, reference
func GetSchemaType(schema openapi.Schema) string {
	if schema == nil {
		return ""
	}
	visitor := &BaseSchemaVisitor{}
	schema.Accept(visitor)
	return visitor.Kind
}

// GetKind converts schema to an *openapi.Kind object
func GetKind(schema openapi.Schema) (*openapi.Kind, error) {
	if schema == nil {
		return nil, nil
	}
	visitor := &KindSchemaVisitor{}
	schema.Accept(visitor)
	return visitor.Result, visitor.Err
}

// GetArray converts schema to an *openapi.Array object
func GetArray(schema openapi.Schema) (*openapi.Array, error) {
	if schema == nil {
		return nil, nil
	}
	visitor := &ArraySchemaVisitor{}
	schema.Accept(visitor)
	return visitor.Result, visitor.Err
}

// GetMap converts schema to an *openapi.Map object
func GetMap(schema openapi.Schema) (*openapi.Map, error) {
	if schema == nil {
		return nil, nil
	}
	visitor := &MapSchemaVisitor{}
	schema.Accept(visitor)
	return visitor.Result, visitor.Err
}

// GetPrimitive converts schema to an *openapi.Primitive object
func GetPrimitive(schema openapi.Schema) (*openapi.Primitive, error) {
	if schema == nil {
		return nil, nil
	}
	visitor := &PrimitiveSchemaVisitor{}
	schema.Accept(visitor)
	return visitor.Result, visitor.Err
}

type BaseSchemaVisitor struct {
	Err  error
	Kind string
}

func (v *BaseSchemaVisitor) VisitArray(array *openapi.Array) {
	v.Kind = "array"
	v.Err = fmt.Errorf("Array type not expected")
}
func (v *BaseSchemaVisitor) VisitMap(*openapi.Map) {
	v.Kind = "map"
	v.Err = fmt.Errorf("Map type not expected")
}

func (v *BaseSchemaVisitor) VisitPrimitive(*openapi.Primitive) {
	v.Kind = "primitive"
	v.Err = fmt.Errorf("Primitive type not expected")
}
func (v *BaseSchemaVisitor) VisitKind(*openapi.Kind) {
	v.Kind = "kind"
	v.Err = fmt.Errorf("Kind type not expected")
}

func (v *BaseSchemaVisitor) VisitReference(reference openapi.Reference) {
	v.Kind = "reference"
	v.Err = fmt.Errorf("Reference type not expected")
}

type KindSchemaVisitor struct {
	BaseSchemaVisitor
	Result *openapi.Kind
}

func (v *KindSchemaVisitor) VisitKind(result *openapi.Kind) {
	v.Result = result
	v.Kind = "kind"
}

func (v *KindSchemaVisitor) VisitReference(reference openapi.Reference) {
	reference.SubSchema().Accept(v)
}

type MapSchemaVisitor struct {
	BaseSchemaVisitor
	Result *openapi.Map
}

func (v *MapSchemaVisitor) VisitMap(result *openapi.Map) {
	v.Result = result
	v.Kind = "map"
}

func (v *MapSchemaVisitor) VisitReference(reference openapi.Reference) {
	reference.SubSchema().Accept(v)
}

type ArraySchemaVisitor struct {
	BaseSchemaVisitor
	Result *openapi.Array
}

func (v *ArraySchemaVisitor) VisitArray(result *openapi.Array) {
	v.Result = result
	v.Kind = "array"
}

func (v *ArraySchemaVisitor) VisitReference(reference openapi.Reference) {
	reference.SubSchema().Accept(v)
}

type PrimitiveSchemaVisitor struct {
	BaseSchemaVisitor
	Result *openapi.Primitive
}

func (v *PrimitiveSchemaVisitor) VisitPrimitive(result *openapi.Primitive) {
	v.Result = result
	v.Kind = "primitive"
}

func (v *PrimitiveSchemaVisitor) VisitReference(reference openapi.Reference) {
	reference.SubSchema().Accept(v)
}
