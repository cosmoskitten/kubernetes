/*
Copyright 2016 The Kubernetes Authors.

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

package selinux

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

func TestMustRunAsOptions(t *testing.T) {
	tests := map[string]struct {
		opts *extensions.SELinuxStrategyOptions
		pass bool
	}{
		"invalid opts": {
			opts: &extensions.SELinuxStrategyOptions{},
			pass: false,
		},
		"valid opts": {
			opts: &extensions.SELinuxStrategyOptions{SELinuxOptions: &api.SELinuxOptions{}},
			pass: true,
		},
	}
	for name, tc := range tests {
		_, err := NewMustRunAs(tc.opts)
		if err != nil && tc.pass {
			t.Errorf("%s expected to pass but received error %#v", name, err)
		}
		if err == nil && !tc.pass {
			t.Errorf("%s expected to fail but did not receive an error", name)
		}
	}
}

func TestMustRunAsGenerate(t *testing.T) {
	opts := &extensions.SELinuxStrategyOptions{
		SELinuxOptions: &api.SELinuxOptions{
			User:  "user",
			Role:  "role",
			Type:  "type",
			Level: "level",
		},
	}
	mustRunAs, err := NewMustRunAs(opts)
	if err != nil {
		t.Fatalf("unexpected error initializing NewMustRunAs %v", err)
	}
	generated, err := mustRunAs.Generate(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error generating selinux %v", err)
	}
	if !reflect.DeepEqual(generated, opts.SELinuxOptions) {
		t.Errorf("generated selinux does not equal configured selinux")
	}
}

func TestMustRunAsValidate(t *testing.T) {
	newValidOpts := func() *api.SELinuxOptions {
		return &api.SELinuxOptions{
			User:  "user",
			Role:  "role",
			Level: "s0:c6,c0",
			Type:  "type",
		}
	}

	newValidOptsWithLevel := func(level string) *api.SELinuxOptions {
		opts := newValidOpts()
		opts.Level = level
		return opts
	}

	role := newValidOpts()
	role.Role = "invalid"

	user := newValidOpts()
	user.User = "invalid"

	level := newValidOpts()
	level.Level = "invalid"

	seType := newValidOpts()
	seType.Type = "invalid"

	tests := map[string]struct {
		seLinux         *api.SELinuxOptions
		expectedSeLinux *api.SELinuxOptions
		expectedMsg     string
	}{
		"invalid role": {
			seLinux:         role,
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required role",
		},
		"invalid user": {
			seLinux:         user,
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required user",
		},
		"invalid level": {
			seLinux:         level,
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid type": {
			seLinux:         seType,
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required type",
		},
		"invalid level, expected sensitivity with a wrong prefix": {
			seLinux:         newValidOpts(),
			expectedSeLinux: newValidOptsWithLevel("s0-w2"),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual sensitivity with a wrong prefix": {
			seLinux:         newValidOptsWithLevel("s0-w2"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid level, sensitivity only vs sensitivity with a category": {
			seLinux:         newValidOptsWithLevel("s1"),
			expectedSeLinux: newValidOptsWithLevel("s1:c1"),
			expectedMsg:     "does not match required level",
		},
		"invalid level, expected category with a wrong prefix": {
			seLinux:         newValidOpts(),
			expectedSeLinux: newValidOptsWithLevel("s0:c1.w2"),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual category with a wrong prefix": {
			seLinux:         newValidOptsWithLevel("s0:c1.w2"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual sensitivity with an invalid an initial boundary": {
			seLinux:         newValidOptsWithLevel("sS"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual sensitivity with an invalid end of a boundary": {
			seLinux:         newValidOptsWithLevel("s0-sS"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual sensitivity with an invalid boundaries": {
			seLinux:         newValidOptsWithLevel("s6-s0"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual category with an invalid an initial boundary": {
			seLinux:         newValidOptsWithLevel("s0:cC.c0"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual category with an invalid end of a boundary": {
			seLinux:         newValidOptsWithLevel("s0:c0.cC"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"invalid level, actual category with an invalid boundaries": {
			seLinux:         newValidOptsWithLevel("s0:c6.c0"),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "does not match required level",
		},
		"valid": {
			seLinux:         newValidOpts(),
			expectedSeLinux: newValidOpts(),
			expectedMsg:     "",
		},
		"valid level with human-readable definition (single value)": {
			seLinux:         newValidOptsWithLevel("SystemLow"),
			expectedSeLinux: newValidOptsWithLevel("SystemLow"),
			expectedMsg:     "",
		},
		"valid level with human-readable definition (range)": {
			seLinux:         newValidOptsWithLevel("SystemLow-SystemHigh"),
			expectedSeLinux: newValidOptsWithLevel("SystemLow-SystemHigh"),
			expectedMsg:     "",
		},
		"valid level with sensitivity only": {
			seLinux:         newValidOptsWithLevel("s0"),
			expectedSeLinux: newValidOptsWithLevel("s0"),
			expectedMsg:     "",
		},
		"valid level with single sensitivity and category": {
			seLinux:         newValidOptsWithLevel("s0:c0"),
			expectedSeLinux: newValidOptsWithLevel("s0:c0"),
			expectedMsg:     "",
		},
		"valid level with identical sensitivity": {
			seLinux:         newValidOptsWithLevel("s0-s0:c6,c0"),
			expectedSeLinux: newValidOptsWithLevel("s0:c6,c0"),
			expectedMsg:     "",
		},
		"valid level with abbreviated sensitivity and categories": {
			seLinux:         newValidOptsWithLevel("s1-s3:c10.c12"),
			expectedSeLinux: newValidOptsWithLevel("s1,s2,s3:c10,c11,c12"),
			expectedMsg:     "",
		},
		"valid level with a multiple sensitivity ranges": {
			seLinux:         newValidOptsWithLevel("s0-s0,s3-s4,s1-s2"),
			expectedSeLinux: newValidOptsWithLevel("s0,s1,s2,s3,s4"),
			expectedMsg:     "",
		},
		"valid level with different order of categories": {
			seLinux:         newValidOptsWithLevel("s0:c0,c6"),
			expectedSeLinux: newValidOptsWithLevel("s0:c6,c0"),
			expectedMsg:     "",
		},
		"valid level with abbreviated categories (that starts from 0)": {
			seLinux:         newValidOptsWithLevel("s0:c0.c3"),
			expectedSeLinux: newValidOptsWithLevel("s0:c0,c1,c2,c3"),
			expectedMsg:     "",
		},
		"valid level with abbreviated categories (that starts from 1)": {
			seLinux:         newValidOptsWithLevel("s0:c1.c3"),
			expectedSeLinux: newValidOptsWithLevel("s0:c1,c2,c3"),
			expectedMsg:     "",
		},
		"valid level with multiple abbreviated and non-abbreviated categories": {
			seLinux:         newValidOptsWithLevel("s0:c0,c2.c5,c7,c9.c10"),
			expectedSeLinux: newValidOptsWithLevel("s0:c0,c2,c3,c4,c5,c7,c9,c10"),
			expectedMsg:     "",
		},
	}

	for name, tc := range tests {
		opts := &extensions.SELinuxStrategyOptions{
			SELinuxOptions: tc.expectedSeLinux,
		}
		mustRunAs, err := NewMustRunAs(opts)
		if err != nil {
			t.Errorf("unexpected error initializing NewMustRunAs for testcase %s: %#v", name, err)
			continue
		}
		container := &api.Container{
			Name: "selinux-testing-container",
			SecurityContext: &api.SecurityContext{
				SELinuxOptions: tc.seLinux,
			},
		}

		errs := mustRunAs.Validate(nil, container)
		//should've passed but didn't
		if len(tc.expectedMsg) == 0 && len(errs) > 0 {
			t.Errorf("%q expected no errors but received %v", name, errs)
		}
		//should've failed but didn't
		if len(tc.expectedMsg) != 0 && len(errs) == 0 {
			t.Errorf("%q expected error %s but received no errors", name, tc.expectedMsg)
		}
		//failed with additional messages
		if len(tc.expectedMsg) != 0 && len(errs) > 1 {
			t.Errorf("%q expected error %s but received multiple errors: %v", name, tc.expectedMsg, errs)
		}
		//check that we got the right message
		if len(tc.expectedMsg) != 0 && len(errs) == 1 {
			if !strings.Contains(errs[0].Error(), tc.expectedMsg) {
				t.Errorf("%q expected error to contain %s but it did not: %v", name, tc.expectedMsg, errs)
			}
		}
	}
}
