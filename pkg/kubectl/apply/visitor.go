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

// Visitor implements a strategy for merging recorded, local and remote values contained
// in an element and returns the merged result.
type Visitor interface {
	// VisitList is invoked by ListElements when Accept is called
	VisitList(ListElement) (Result, error)

	// VisitMap is invoked by MapElements when Accept is called
	VisitMap(MapElement) (Result, error)

	// VisitMap is invoked by MapElements when Accept is called
	VisitType(TypeElement) (Result, error)
	// VisitPrimitive is invoked by PrimitiveElements when Accept is called

	VisitPrimitive(PrimitiveElement) (Result, error)

	// VisitEmpty is invoked by EmptyElements when Accept is called
	VisitEmpty(EmptyElement) (Result, error)
}

// Operation records whether a field should be set or dropped
type Operation int

const (
	// ERROR is an error during merge
	ERROR Operation = iota
	// SET sets the field on an object
	SET
	// DROP drops the field from an object
	DROP
)

// Result is the result of merging fields
type Result struct {
	// Operation is the operation that should be performed for the merged field
	Operation Operation
	// MergedResult is the new merged value
	MergedResult interface{}
}

type MapValuesElement interface {
	Element
	GetValues() map[string]Element
}
