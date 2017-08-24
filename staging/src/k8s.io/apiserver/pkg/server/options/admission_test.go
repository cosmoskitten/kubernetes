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

package options

import "testing"

func TestPreparePluginNamesIfNotProvidedMethod(t *testing.T) {
	scenarios := []struct {
		expectedPluginNames       []string
		readFromCommandLine       []string
		setDefaultOffPluginNames  []string
		setRecommendedPluginOrder []string
	}{
		// scenario 1: no values were read from the command line
		// check if a call to preparePluginNamesIfNotProvided sets expected values.
		{
			expectedPluginNames: []string{"NamespaceLifecycle"},
		},

		// scenario 2: some plugin names were read from the command line
		// check if a call to preparePluginNamesIfNotProvided does not overwrite them.
		{
			expectedPluginNames: []string{"APluginName"},
			readFromCommandLine: []string{"APluginName"},
		},

		// scenario 3: overwrite RecommendedPluginOrder and set DefaultOffPluginNames
		// make sure that plugins which are on DefaultOffPluginNames list do not get to PluginNames list.
		{
			expectedPluginNames:       []string{"pluginA"},
			setRecommendedPluginOrder: []string{"pluginA", "pluginB"},
			setDefaultOffPluginNames:  []string{"pluginB"},
		},

		// scenario 4: plugin names read from command line take precedence.
		{
			readFromCommandLine:       []string{"pluginD"},
			expectedPluginNames:       []string{"pluginD"},
			setRecommendedPluginOrder: []string{"pluginA", "pluginB"},
			setDefaultOffPluginNames:  []string{"pluginB"},
		},
	}

	// act
	for _, scenario := range scenarios {
		target := NewAdmissionOptions()

		if len(scenario.readFromCommandLine) > 0 {
			target.PluginNames = scenario.readFromCommandLine
		}
		if len(scenario.setDefaultOffPluginNames) > 0 {
			target.DefaultOffPlugins = scenario.setDefaultOffPluginNames
		}
		if len(scenario.setRecommendedPluginOrder) > 0 {
			target.RecommendedPluginOrder = scenario.setRecommendedPluginOrder
		}

		target.preparePluginNamesIfNotProvided()

		if len(target.PluginNames) != len(scenario.expectedPluginNames) {
			t.Errorf("incorrect number of items, got %d, expected = %d", len(target.PluginNames), len(scenario.expectedPluginNames))
		}
		for i := range target.PluginNames {
			if scenario.expectedPluginNames[i] != target.PluginNames[i] {
				t.Errorf("missmatch at index = %d, got = %s, expected = %s", i, target.PluginNames[i], scenario.expectedPluginNames[i])
			}
		}
	}
}
