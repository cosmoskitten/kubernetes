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

package validation

import (
	"reflect"
	"sort"

	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
)

type ValidationItem interface {
	openapi.SchemaVisitor

	Errors() []error
	Path() *openapi.Path
}

type baseValidationItem struct {
	*openapi.BaseItem
}

func NewBaseValidationItem(path openapi.Path) baseValidationItem {
	return baseValidationItem{BaseItem: openapi.NewBaseItem(path)}
}

// AddValidationError wraps the given error into a ValidationError and
// attaches it to this item.
func (item *baseValidationItem) AddValidationError(err error) {
	item.GetErrors().AppendErrors(ValidationError{Path: item.GetPath().String(), Err: err})
}

// mapItem represents a map entry in the yaml.
type mapItem struct {
	baseValidationItem

	Map map[string]interface{}
}

func (item *mapItem) sortedKeys() []string {
	sortedKeys := []string{}
	for key := range item.Map {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	return sortedKeys
}

var _ ValidationItem = &mapItem{}

func (item *mapItem) VisitPrimitive(schema *openapi.Primitive) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: schema.Type, Actual: "map"})
}

func (item *mapItem) VisitArray(schema *openapi.Array) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: "array", Actual: "map"})
}

func (item *mapItem) VisitMap(schema *openapi.Map) {
	for _, key := range item.sortedKeys() {
		subItem, err := itemFactory(item.Path().FieldPath(key), item.Map[key])
		if err != nil {
			item.AddError(err)
			continue
		}
		schema.SubType.Accept(subItem)
		item.CopyErrors(subItem.Errors())
	}
}

func (item *mapItem) VisitKind(schema *openapi.Kind) {
	// Verify each sub-field.
	for _, key := range item.sortedKeys() {
		if item.Map[key] == nil {
			continue
		}
		subItem, err := itemFactory(item.Path().FieldPath(key), item.Map[key])
		if err != nil {
			item.AddError(err)
			continue
		}
		if _, ok := schema.Fields[key]; !ok {
			item.AddValidationError(UnknownFieldError{Path: schema.GetPath().String(), Field: key})
			continue
		}
		schema.Fields[key].Accept(subItem)
		item.CopyErrors(subItem.Errors())
	}

	// Verify that all required fields are present.
	for _, required := range schema.RequiredFields {
		if v, ok := item.Map[required]; !ok || v == nil {
			item.AddValidationError(MissingRequiredFieldError{Path: schema.GetPath().String(), Field: required})
		}
	}
}

func (item *mapItem) VisitReference(schema openapi.Reference) {
	// passthrough
	schema.SubSchema().Accept(item)
}

// arrayItem represents a yaml array.
type arrayItem struct {
	baseValidationItem

	Array []interface{}
}

var _ ValidationItem = &arrayItem{}

func (item *arrayItem) VisitPrimitive(schema *openapi.Primitive) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: schema.Type, Actual: "array"})
}

func (item *arrayItem) VisitArray(schema *openapi.Array) {
	for i, v := range item.Array {
		path := item.Path().ArrayPath(i)
		if v == nil {
			item.AddValidationError(InvalidObjectTypeError{Type: "nil", Path: path.String()})
			continue
		}
		subItem, err := itemFactory(path, v)
		if err != nil {
			item.AddError(err)
			continue
		}
		schema.SubType.Accept(subItem)
		item.CopyErrors(subItem.Errors())
	}
}

func (item *arrayItem) VisitMap(schema *openapi.Map) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: "array", Actual: "map"})
}

func (item *arrayItem) VisitKind(schema *openapi.Kind) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: "array", Actual: "map"})
}

func (item *arrayItem) VisitReference(schema openapi.Reference) {
	// passthrough
	schema.SubSchema().Accept(item)
}

// primitiveItem represents a yaml value.
type primitiveItem struct {
	baseValidationItem

	Value interface{}
	Kind  string
}

var _ ValidationItem = &primitiveItem{}

func (item *primitiveItem) VisitPrimitive(schema *openapi.Primitive) {
	// Some types of primitives can match more than one (a number
	// can be a string, but not the other way around). Return from
	// the switch if we have a valid possible type conversion
	// NOTE(apelisse): This logic is blindly copied from the
	// existing swagger logic, and I'm not sure I agree with it.
	switch schema.Type {
	case openapi.Boolean:
		switch item.Kind {
		case openapi.Boolean:
			return
		}
	case openapi.Integer:
		switch item.Kind {
		case openapi.Integer, openapi.Number:
			return
		}
	case openapi.Number:
		switch item.Kind {
		case openapi.Number:
			return
		}
	case openapi.String:
		return
	}

	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: schema.Type, Actual: item.Kind})
}

func (item *primitiveItem) VisitArray(schema *openapi.Array) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: "array", Actual: item.Kind})
}

func (item *primitiveItem) VisitMap(schema *openapi.Map) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: "map", Actual: item.Kind})
}

func (item *primitiveItem) VisitKind(schema *openapi.Kind) {
	item.AddValidationError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: "map", Actual: item.Kind})
}

func (item *primitiveItem) VisitReference(schema openapi.Reference) {
	// passthrough
	schema.SubSchema().Accept(item)
}

// itemFactory creates the relevant item type/visitor based on the current yaml type.
func itemFactory(path openapi.Path, v interface{}) (ValidationItem, error) {
	// We need to special case for no-type fields in yaml (e.g. empty item in list)
	if v == nil {
		return nil, InvalidObjectTypeError{Type: "nil", Path: path.String()}
	}
	kind := reflect.TypeOf(v).Kind()
	switch kind {
	case reflect.Bool:
		return &primitiveItem{
			baseValidationItem: NewBaseValidationItem(path),
			Value:              v,
			Kind:               openapi.Boolean,
		}, nil
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return &primitiveItem{
			baseValidationItem: NewBaseValidationItem(path),
			Value:              v,
			Kind:               openapi.Integer,
		}, nil
	case reflect.Float32,
		reflect.Float64:
		return &primitiveItem{
			baseValidationItem: NewBaseValidationItem(path),
			Value:              v,
			Kind:               openapi.Number,
		}, nil
	case reflect.String:
		return &primitiveItem{
			baseValidationItem: NewBaseValidationItem(path),
			Value:              v,
			Kind:               openapi.String,
		}, nil
	case reflect.Array,
		reflect.Slice:
		return &arrayItem{
			baseValidationItem: NewBaseValidationItem(path),
			Array:              v.([]interface{}),
		}, nil
	case reflect.Map:
		return &mapItem{
			baseValidationItem: NewBaseValidationItem(path),
			Map:                v.(map[string]interface{}),
		}, nil
	}
	return nil, InvalidObjectTypeError{Type: kind.String(), Path: path.String()}
}
