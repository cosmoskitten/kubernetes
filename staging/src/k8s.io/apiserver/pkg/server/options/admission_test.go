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

package options_test

import (
	"testing"

	"k8s.io/apiserver/pkg/server/options"
)

func TestSetDefaultPlugins(t *testing.T) {
	scenarios := []struct {
		expectedPluginNames []string
		readFromCommandLine []string
	}{
		// scenario 1: no values were read from the command line
		// check if a call to SetDefaultPlugins sets expected values.
		{
			expectedPluginNames: []string{"NamespaceLifecycle"},
		},
		// scenario 2: some plugin names were read from the command line
		// check if a call to SetDefaultPlugins does not overwrite them.
		{
			expectedPluginNames: []string{"APluginName"},
			readFromCommandLine: []string{"APluginName"},
		},
	}

	// act
	for _, scenario := range scenarios {
		target := options.NewAdmissionOptions()

		if len(scenario.readFromCommandLine) > 0 {
			target.PluginNames = scenario.readFromCommandLine
		}

		target.SetDefaultPlugins()

		if len(target.PluginNames) != len(scenario.expectedPluginNames) {
			t.Errorf("incorrect number of items, got %d, expected = %d", len(target.PluginNames), len(scenario.expectedPluginNames))
		}
		for i, _ := range target.PluginNames {
			if scenario.expectedPluginNames[i] != target.PluginNames[i] {
				t.Errorf("missmatch at index = %d, got = %s, expected = %s", i, target.PluginNames[i], scenario.expectedPluginNames[i])
			}
		}
	}
}
