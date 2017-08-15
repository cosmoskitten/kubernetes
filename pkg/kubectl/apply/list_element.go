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

// ListElement contains the recorded, local and remote values for a field
// of type list
type ListElement struct {
	FieldMetaImpl

	// Name of the field
	Name string

	RecordedSet bool
	LocalSet    bool
	RemoteSet   bool

	Recorded []interface{}
	Local    []interface{}
	Remote   []interface{}

	// Present for lists that can be merged only.  Contains the items
	// from each of the 3 lists merged into single Elements using
	// the merge-key.
	Values []Element
}

func (e ListElement) Accept(v Visitor) (Result, error) {
	return v.VisitList(e)
}

func (e ListElement) GetRecorded() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Recorded == nil {
		return nil
	}
	return e.Recorded
}

func (e ListElement) GetLocal() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Local == nil {
		return nil
	}
	return e.Local
}

func (e ListElement) GetRemote() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Remote == nil {
		return nil
	}
	return e.Remote
}

func (e ListElement) HasRecorded() bool {
	return e.RecordedSet
}

func (e ListElement) HasLocal() bool {
	return e.LocalSet
}

func (e ListElement) HasRemote() bool {
	return e.RemoteSet
}

var _ Element = &ListElement{}
