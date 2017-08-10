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

package gce

import "fmt"

const (
	allAlphaFeatures = "all"
	noneAlphaFeature = "none"
)

// All supported alpha features
var supportedAlphaFeatures = map[string]bool{}

type AlphaFeatureGate struct {
	features map[string]bool
}

func (af *AlphaFeatureGate) Enabled(key string) bool {
	return af.features[key]
}

func NewAlphaFeatureGate(alphaApis []string) (*AlphaFeatureGate, error) {
	alphaApiMap := make(map[string]bool)
	for _, apiName := range alphaApis {
		if apiName == allAlphaFeatures {
			alphaApiMap = supportedAlphaFeatures
			break
		}
		if apiName == noneAlphaFeature {
			alphaApiMap = make(map[string]bool)
			break
		}
		if _, ok := supportedAlphaFeatures[apiName]; !ok {
			return nil, fmt.Errorf("Alpha feature %q is not supported.", apiName)
		}
		alphaApiMap[apiName] = true
	}
	return &AlphaFeatureGate{alphaApiMap}, nil
}
