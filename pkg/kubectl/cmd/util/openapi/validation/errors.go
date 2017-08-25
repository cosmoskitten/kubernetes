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
	"fmt"
)

type ValidationError struct {
	Path string
	Err  error
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("ValidationError(%s): %v", e.Path, e.Err)
}

type MissingRequiredFieldError struct {
	Path  string
	Field string
}

func (e MissingRequiredFieldError) Error() string {
	return fmt.Sprintf("missing required field %q in %s", e.Field, e.Path)
}

type UnknownFieldError struct {
	Path  string
	Field string
}

func (e UnknownFieldError) Error() string {
	return fmt.Sprintf("unknown field %q in %s", e.Field, e.Path)
}

type InvalidObjectTypeError struct {
	Path string
	Type string
}

func (e InvalidObjectTypeError) Error() string {
	return fmt.Sprintf("unknown object type %q in %s", e.Type, e.Path)
}
