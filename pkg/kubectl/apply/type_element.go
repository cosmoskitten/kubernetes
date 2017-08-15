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

// TypeElement contains the recorded, local and remote values for a field
// that is a complex type
type TypeElement struct {
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

func (e TypeElement) Accept(v Visitor) (Result, error) {
	return v.VisitType(e)
}

// GetRecorded returns the field value from the recorded source
func (e TypeElement) GetRecorded() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Recorded == nil {
		return nil
	}
	return e.Recorded
}

func (e TypeElement) HasRecorded() bool {
	return e.RecordedSet
}

func (e TypeElement) GetLocal() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Local == nil {
		return nil
	}
	return e.Local
}

func (e TypeElement) GetRemote() interface{} {
	// https://golang.org/doc/faq#nil_error
	if e.Remote == nil {
		return nil
	}
	return e.Remote
}

// GetValues contains the subfields of this field.
// returns a map of subfield name to value for the union of
// subfields  observed in the recorded, local and remote values
func (e TypeElement) GetValues() map[string]Element {
	return e.Values
}

func (e TypeElement) HasLocal() bool {
	return e.LocalSet
}

func (e TypeElement) HasRemote() bool {
	return e.RemoteSet
}

var _ Element = &TypeElement{}
