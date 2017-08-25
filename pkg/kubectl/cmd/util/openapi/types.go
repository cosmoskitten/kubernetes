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

package openapi

type BaseItem struct {
	errors Errors
	path   Path
}

func NewBaseItem(p Path) *BaseItem {
	return &BaseItem{path: p}
}

func (item *BaseItem) GetErrors() *Errors {
	return &item.errors
}

func (item *BaseItem) GetPath() *Path {
	return &item.path
}

// Errors returns the list of errors found for this item.
func (item *BaseItem) Errors() []error {
	return item.errors.Errors()
}

// AddError adds a regular (non-validation related) error to the list.
func (item *BaseItem) AddError(err error) {
	item.errors.AppendErrors(err)
}

// CopyErrors adds a list of errors to this item. This is useful to copy
// errors from subitems.
func (item *BaseItem) CopyErrors(errs []error) {
	item.errors.AppendErrors(errs...)
}

// Path returns the path of this item, helps print useful errors.
func (item *BaseItem) Path() *Path {
	return &item.path
}
