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

// MapElement contains the recorded, local and remote values for a field
// of type map
type MapElement struct {
	FieldMetaImpl

	// Name of the field
	Name string

	RecordedSet bool
	LocalSet    bool
	RemoteSet   bool

	Recorded map[string]interface{}
	Local    map[string]interface{}
	Remote   map[string]interface{}

	// Values contains the values in mapElement.  Element must contain
	// a Name matching its key in Values
	Values map[string]Element
}

func (e MapElement) Accept(v Visitor) (Result, error) {
	return v.VisitMap(e)
}

func (e MapElement) GetRecorded() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Recorded == nil {
		return nil
	}
	return e.Recorded
}

func (e MapElement) GetLocal() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Local == nil {
		return nil
	}
	return e.Local
}

func (e MapElement) GetRemote() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Remote == nil {
		return nil
	}
	return e.Remote
}

func (e MapElement) GetValues() map[string]Element {
	return e.Values
}

func (e MapElement) HasRecorded() bool {
	return e.RecordedSet
}

func (e MapElement) HasLocal() bool {
	return e.LocalSet
}

func (e MapElement) HasRemote() bool {
	return e.RemoteSet
}

var _ Element = &MapElement{}
