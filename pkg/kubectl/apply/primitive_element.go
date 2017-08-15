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

import (
	"fmt"
)

// PrimitiveElement contains the recorded, local and remote values for a field
// of type primitive
type PrimitiveElement struct {
	FieldMetaImpl

	// Name of the field
	Name string

	RecordedSet bool
	LocalSet    bool
	RemoteSet   bool

	Recorded interface{}
	Local    interface{}
	Remote   interface{}
}

func (e PrimitiveElement) Accept(v Visitor) (Result, error) {
	return v.VisitPrimitive(e)
}

func (e PrimitiveElement) String() string {
	return fmt.Sprintf("name: %s recorded: %v local: %v remote: %v", e.Name, e.Recorded, e.Local, e.Remote)
}

func (e PrimitiveElement) GetRecorded() interface{} {
	return e.Recorded
}

func (e PrimitiveElement) GetLocal() interface{} {
	return e.Local
}

func (e PrimitiveElement) GetRemote() interface{} {
	return e.Remote
}

func (e PrimitiveElement) HasRecorded() bool {
	return e.RecordedSet
}

func (e PrimitiveElement) HasLocal() bool {
	return e.LocalSet
}

func (e PrimitiveElement) HasRemote() bool {
	return e.RemoteSet
}

var _ Element = &PrimitiveElement{}
