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

package testing

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/googleapis/gnostic/OpenAPIv2"
	"github.com/googleapis/gnostic/compiler"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
)

func readOpenapiFromFile(openapiDir string) (*fakeOpenapiResources, error) {
	_, err := os.Stat(openapiDir)
	if err != nil {
		return nil, err
	}
	spec, err := ioutil.ReadFile(openapiDir)
	if err != nil {
		return nil, err
	}
	var info yaml.MapSlice
	err = yaml.Unmarshal(spec, &info)
	if err != nil {
		return nil, err
	}
	document, err := openapi_v2.NewDocument(info, compiler.NewContext("$root", nil))
	return NewOpenAPIData(document)
}

type fakeOpenapiResources struct {
	openapi.Definitions
	mergeItem          openapi.Schema
	retainKeyMergeItem openapi.Schema
	precisionItem      openapi.Schema
}

func NewOpenAPIData(doc *openapi_v2.Document) (*fakeOpenapiResources, error) {
	definitions := fakeOpenapiResources{
		Definitions: openapi.Definitions{
			Models:    map[string]openapi.Schema{},
			Resources: map[schema.GroupVersionKind]string{},
		},
	}

	// Save the list of all models first. This will allow us to
	// validate that we don't have any dangling reference.
	for _, namedSchema := range doc.GetDefinitions().GetAdditionalProperties() {
		definitions.Models[namedSchema.GetName()] = nil
	}

	// Now, parse each model. We can validate that references exists.
	for _, namedSchema := range doc.GetDefinitions().GetAdditionalProperties() {
		path := openapi.NewPath(namedSchema.GetName())
		schema, err := definitions.ParseSchema(namedSchema.GetValue(), &path)
		if err != nil {
			return nil, err
		}
		definitions.Models[namedSchema.GetName()] = schema
		if strings.Contains(namedSchema.GetName(), "mergeItem") {
			definitions.mergeItem = schema
		}
		if strings.Contains(namedSchema.GetName(), "retainKeyMergeItem") {
			definitions.retainKeyMergeItem = schema
		}
		if strings.Contains(namedSchema.GetName(), "precisionItem") {
			definitions.precisionItem = schema
		}
	}

	return &definitions, nil
}

func (r *fakeOpenapiResources) getMergeItem() (openapi.Schema, error) {
	if r.mergeItem == nil {
		return nil, errors.New("cannot find mergeItem in openapi schema.")
	}
	if k, ok := r.mergeItem.(*openapi.Kind); !ok {
		return nil, mergepatch.ErrBadArgType(k, r.mergeItem)
	}
	return r.mergeItem, nil
}

func (r *fakeOpenapiResources) getRetainKeysMergeItem() (openapi.Schema, error) {
	if r.retainKeyMergeItem == nil {
		return nil, errors.New("cannot find retainKeyMergeItem in openapi schema.")
	}
	if k, ok := r.retainKeyMergeItem.(*openapi.Kind); !ok {
		return nil, mergepatch.ErrBadArgType(k, r.retainKeyMergeItem)
	}
	return r.retainKeyMergeItem, nil
}

func (r *fakeOpenapiResources) getPrecisionItem() (openapi.Schema, error) {
	if r.precisionItem == nil {
		return nil, errors.New("cannot find precisionItem in openapi schema.")
	}
	if k, ok := r.precisionItem.(*openapi.Kind); !ok {
		return nil, mergepatch.ErrBadArgType(k, r.precisionItem)
	}
	return r.precisionItem, nil
}

func GetMergeItemOrDie(openapiDir string) openapi.Schema {
	oa, err := readOpenapiFromFile(openapiDir)
	if err != nil {
		panic(err)
	}
	mi, err := oa.getMergeItem()
	if err != nil {
		panic(err)
	}
	return mi
}

func GetRetainKeysMergeItemOrDie(openapiDir string) openapi.Schema {
	oa, err := readOpenapiFromFile(openapiDir)
	if err != nil {
		panic(err)
	}
	rkmi, err := oa.getRetainKeysMergeItem()
	if err != nil {
		panic(err)
	}
	return rkmi
}

func GetPrecisionItemOrDie(openapiDir string) openapi.Schema {
	oa, err := readOpenapiFromFile(openapiDir)
	if err != nil {
		panic(err)
	}
	pi, err := oa.getPrecisionItem()
	if err != nil {
		panic(err)
	}
	return pi
}
