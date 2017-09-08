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

package deviceplugin

import (
	"fmt"

	"k8s.io/api/core/v1"
	v1helper "k8s.io/kubernetes/pkg/api/v1/helper"
)

// IsResourceNameValid returns an error if the resource is invalid or is not an
// extended resource name.
func IsResourceNameValid(resourceName string) error {
	if !v1helper.IsExtendedResourceName(v1.ResourceName(resourceName)) {
		return fmt.Errorf(errInvalidResourceName, resourceName)
	}
	return nil
}
