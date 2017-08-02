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
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
)

var scheme = runtime.NewScheme()

func init() {
	addKnownTypes(scheme)
	internalGV := schema.GroupVersion{Group: auditinternal.GroupName, Version: runtime.APIVersionInternal}
	scheme.AddKnownTypes(internalGV,
		&auditinternal.Event{},
	)
	RegisterConversions(scheme)
}

func TestConversion(t *testing.T) {
	scheme.Log(t)

	testcases := map[string]struct {
		old      *ObjectReference
		expected *auditinternal.ObjectReference
	}{
		"core group": {
			old: &ObjectReference{
				APIVersion: "/v1",
			},
			expected: &auditinternal.ObjectReference{
				APIVersion: "v1",
				APIGroup:   "",
			},
		},
		"other groups": {
			old: &ObjectReference{
				APIVersion: "rbac.authorization.k8s.io/v1beta1",
			},
			expected: &auditinternal.ObjectReference{
				APIVersion: "v1beta1",
				APIGroup:   "rbac.authorization.k8s.io",
			},
		},
		"all empty": {
			old:      &ObjectReference{},
			expected: &auditinternal.ObjectReference{},
		},
		"invalid apiversion should not cause painc": {
			old: &ObjectReference{
				APIVersion: "invalid version without slash",
			},
			expected: &auditinternal.ObjectReference{
				APIVersion: "invalid version without slash",
				APIGroup:   "",
			},
		},
	}
	for k, tc := range testcases {
		internal := &auditinternal.ObjectReference{}
		if err := scheme.Convert(tc.old, internal, nil); err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
		}
		if !reflect.DeepEqual(internal, tc.expected) {
			t.Errorf("%s: expected\n\t%#v, got \n\t%#v", k, tc.expected, internal)
		}
	}
}
