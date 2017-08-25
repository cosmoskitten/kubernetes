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

package strategicpatch

import (
	"strings"

	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
)

const (
	patchStrategyOpenapiextensionKey = "x-kubernetes-patch-strategy"
	patchMergeKeyOpenapiextensionKey = "x-kubernetes-patch-merge-key"
)

type LookupPatchItem interface {
	openapi.SchemaVisitor

	Errors() []error
	Path() *openapi.Path
}

type baseLookupPatchMetaItem struct {
	*openapi.BaseItem
}

func (item *baseLookupPatchMetaItem) AddLookupPatchMetaError(err error) {
	item.GetErrors().AppendErrors(LookupPatchMetaError{Path: item.GetPath().String(), Err: err})
}

type patchItem struct {
	baseLookupPatchMetaItem
	key       string
	patchmeta PatchMeta
	subschema openapi.Schema
}

func NewPatchItem(key string, path *openapi.Path) patchItem {
	return patchItem{
		baseLookupPatchMetaItem: baseLookupPatchMetaItem{
			BaseItem: openapi.NewBaseItem(*path),
		},
		key: key,
	}
}

var _ LookupPatchItem = &patchItem{}

func (item *patchItem) VisitPrimitive(schema *openapi.Primitive) {
	item.subschema = nil
	item.patchmeta = PatchMeta{
		PatchStrategies: []string{""},
		PatchMergeKey:   "",
	}
}

func (item *patchItem) VisitArray(schema *openapi.Array) {
	item.AddLookupPatchMetaError(openapi.InvalidTypeError{Path: schema.GetPath().String(), Expected: "array", Actual: "kind"})
}

func (item *patchItem) VisitMap(schema *openapi.Map) {
	item.subschema = schema.SubType
	item.patchmeta = PatchMeta{
		PatchStrategies: []string{""},
		PatchMergeKey:   "",
	}
}

func (item *patchItem) VisitReference(schema openapi.Reference) {
	// passthrough
	schema.SubSchema().Accept(item)
}

func (item *patchItem) VisitKind(schema *openapi.Kind) {
	subschema, ok := schema.Fields[item.key]
	if !ok {
		item.AddLookupPatchMetaError(FieldNotFoundError{Path: schema.GetPath().String(), Field: item.key})
		return
	}

	extensions := subschema.GetExtensions()
	ps := extensions[patchStrategyOpenapiextensionKey]
	patchStrategy, ok := ps.(string)
	var patchStrategies []string
	if ok {
		patchStrategies = strings.Split(patchStrategy, ",")
	}
	mk := extensions[patchMergeKeyOpenapiextensionKey]
	mergeKey, ok := mk.(string)
	item.patchmeta = PatchMeta{
		PatchStrategies: patchStrategies,
		PatchMergeKey:   mergeKey,
	}
	item.subschema = subschema
}
