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

package apply

// EmptyElement is a placeholder for when no value is set for a field so its type is unknown
type EmptyElement struct {
	FieldMetaImpl

	// Name of the field
	Name string
}

func (e EmptyElement) Accept(v Visitor) (Result, error) {
	return v.VisitEmpty(e)
}

func (e EmptyElement) IsAdd() bool {
	return false
}

func (e EmptyElement) IsDelete() bool {
	return false
}

func (e EmptyElement) GetRecorded() interface{} {
	return nil
}

func (e EmptyElement) GetLocal() interface{} {
	return nil
}

func (e EmptyElement) GetRemote() interface{} {
	return nil
}

func (e EmptyElement) HasRecorded() bool {
	return false
}

func (e EmptyElement) HasLocal() bool {
	return false
}

func (e EmptyElement) HasRemote() bool {
	return false
}

var _ Element = &EmptyElement{}
