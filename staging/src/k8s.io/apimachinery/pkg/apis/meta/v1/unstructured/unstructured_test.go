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

package unstructured

import (
	"reflect"
	"strconv"
	"testing"

	"k8s.io/apimachinery/pkg/conversion/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestUnstructuredList(t *testing.T) {
	list := &UnstructuredList{
		Object: map[string]interface{}{"kind": "List", "apiVersion": "v1"},
		Items: []Unstructured{
			{Object: map[string]interface{}{"kind": "Pod", "apiVersion": "v1", "metadata": map[string]interface{}{"name": "test"}}},
		},
	}
	content := list.UnstructuredContent()
	items := content["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("unexpected items: %#v", items)
	}
	if getNestedField(items[0].(map[string]interface{}), "metadata", "name") != "test" {
		t.Fatalf("unexpected fields: %#v", items[0])
	}
}

func TestConversionRoundtrip(t *testing.T) {
	testCases := []struct {
		obj runtime.Object
	}{
		{
			obj: &Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Foo",
					"metadata": map[string]interface{}{
						"name": "foo1",
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			doRoundTrip(t, tc.obj)
		})
	}
}

// copy from staging/src/k8s.io/apimachinery/pkg/conversion/unstructured/converter_test.go
// otherwise there is a cyclic dependency.
func doRoundTrip(t *testing.T, item runtime.Object) {
	data, err := json.Marshal(item)
	if err != nil {
		t.Errorf("Error when marshaling object: %v", err)
		return
	}

	unstr := make(map[string]interface{})
	err = json.Unmarshal(data, &unstr)
	if err != nil {
		t.Errorf("Error when unmarshaling to unstructured: %v", err)
		return
	}

	data, err = json.Marshal(unstr)
	if err != nil {
		t.Errorf("Error when marshaling unstructured: %v", err)
		return
	}
	unmarshalledObj := reflect.New(reflect.TypeOf(item).Elem()).Interface()
	err = json.Unmarshal(data, &unmarshalledObj)
	if err != nil {
		t.Errorf("Error when unmarshaling to object: %v", err)
		return
	}
	if !reflect.DeepEqual(item, unmarshalledObj) {
		t.Errorf("Object changed during JSON operations, diff: %v", diff.ObjectReflectDiff(item, unmarshalledObj))
		return
	}

	newUnstr, err := unstructured.DefaultConverter.ToUnstructured(item)
	if err != nil {
		t.Errorf("ToUnstructured failed: %v", err)
		return
	}
	newObj := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
	err = unstructured.DefaultConverter.FromUnstructured(newUnstr, newObj)
	if err != nil {
		t.Errorf("FromUnstructured failed: %v", err)
		return
	}

	if !reflect.DeepEqual(item, newObj) {
		t.Errorf("Object changed, diff: %v", diff.ObjectReflectDiff(item, newObj))
	}
}
