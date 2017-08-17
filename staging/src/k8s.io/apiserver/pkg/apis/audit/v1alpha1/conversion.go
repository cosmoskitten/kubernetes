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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apiserver/pkg/apis/audit"
)

// Convert_audit_GroupResources_To_v1alpha1_GroupResources is a manually created conversion
// function. This exists because v1alpha1 doesn't support the ResourceName field.
//
// The conversion simply ignores the ResourceName field when converting to v1alpha1.
func Convert_audit_GroupResources_To_v1alpha1_GroupResources(in *audit.GroupResources, out *GroupResources, s conversion.Scope) error {
	return autoConvert_audit_GroupResources_To_v1alpha1_GroupResources(in, out, s)
}
