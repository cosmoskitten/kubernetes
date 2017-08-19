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

package upgrade

import (
	"testing"
)

func TestValidateFlags(t *testing.T) {
	var tests = []struct {
		newVersionStr string
		expectedErr   bool
	}{
		{ // newVersionStr can't be empty
			newVersionStr: "",
			expectedErr:   true,
		},
		{ // newVersionStr must be a valid semver version
			newVersionStr: "foo",
			expectedErr:   true,
		},
		{ // newVersionStr is now a valid version; should succeed
			newVersionStr: "v1.8.1",
			expectedErr:   false,
		},
		{ // newVersionStr is now a valid prerelease version; should succeed
			newVersionStr: "v1.9.0-alpha.3",
			expectedErr:   false,
		},
	}
	for _, rt := range tests {
		actualErr := ValidateFlags(&applyFlags{
			newK8sVersionStr: rt.newVersionStr,
		})
		if (actualErr != nil) != rt.expectedErr {
			t.Errorf(
				"failed ValidateFlags:\n\texpected: %t\n\t  actual: %t",
				rt.expectedErr,
				(actualErr != nil),
			)
		}
	}
}

func TestSetImplicitFlags(t *testing.T) {
	var tests = []struct {
		dryRun, force, nonInteractiveMode bool
		expectedNonInteractiveMode        bool
	}{
		{ // if not dryRun or force is set; the nonInteractiveMode field should not be touched
			dryRun:                     false,
			force:                      false,
			nonInteractiveMode:         false,
			expectedNonInteractiveMode: false,
		},
		{ // if not dryRun or force is set; the nonInteractiveMode field should not be touched
			dryRun:                     false,
			force:                      false,
			nonInteractiveMode:         true,
			expectedNonInteractiveMode: true,
		},
		{ // if dryRun or force is set; the nonInteractiveMode field should be set to true
			dryRun:                     true,
			force:                      false,
			nonInteractiveMode:         false,
			expectedNonInteractiveMode: true,
		},
		{ // if dryRun or force is set; the nonInteractiveMode field should be set to true
			dryRun:                     false,
			force:                      true,
			nonInteractiveMode:         false,
			expectedNonInteractiveMode: true,
		},
		{ // if dryRun or force is set; the nonInteractiveMode field should be set to true
			dryRun:                     true,
			force:                      true,
			nonInteractiveMode:         false,
			expectedNonInteractiveMode: true,
		},
		{ // if dryRun or force is set; the nonInteractiveMode field should be set to true
			dryRun:                     true,
			force:                      true,
			nonInteractiveMode:         true,
			expectedNonInteractiveMode: true,
		},
	}
	for _, rt := range tests {
		flags := &applyFlags{
			dryRun:             rt.dryRun,
			force:              rt.force,
			nonInteractiveMode: rt.nonInteractiveMode,
		}
		SetImplicitFlags(flags)
		if flags.nonInteractiveMode != rt.expectedNonInteractiveMode {
			t.Errorf(
				"failed SetImplicitFlags:\n\texpected nonInteractiveMode: %t\n\t  actual: %t",
				rt.expectedNonInteractiveMode,
				flags.nonInteractiveMode,
			)
		}
	}
}
