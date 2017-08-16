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

/*
TODO!

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/util/version"
)

type fakePrepuller struct {
	t time.Duration
}

func (p *fakePrepuller) CreateFunc(component string) {
	return
}

func (p *fakePrepuller) WaitFunc(component string) {
	return
}

func (p *fakePrepuller) DeleteFunc(component string) {
	return
}

var _ prepuller = &fakePrepuller{}

func TestPrepullImagesInParallel(t *testing.T) {
	tests := []struct {
		p                *prepuller
	}{
		{ // everything ok
			vg: &fakeVersionGetter{
				clusterVersion: "v1.7.3",
				kubeletVersion: "v1.7.3",
				kubeadmVersion: "v1.7.5",
			},
			newK8sVersion: "v1.7.5",
		},
	}

	for _, rt := range tests {

		newK8sVer, err := version.ParseSemantic(rt.newK8sVersion)
		if err != nil {
			t.Fatalf("couldn't parse version %s: %v", rt.newK8sVersion, err)
		}

		actualSkewErrs := EnforceVersionPolicies(rt.vg, rt.newK8sVersion, newK8sVer, rt.allowExperimental, rt.allowRCs)
		if actualSkewErrs == nil {
			// No errors were seen. Report unit test failure if we expected to see errors
			if rt.expectedMandatoryErrs + rt.expectedSkippableErrs > 0 {
				t.Errorf("failed TestGetAvailableUpgrades\n\texpected errors but got none")
			}
			// Otherwise, just move on with the next test
			continue
		}

		if len(actualSkewErrs.Skippable) != rt.expectedSkippableErrs {
			t.Errorf("failed TestGetAvailableUpgrades\n\texpected skippable errors: %d\n\tgot skippable errors: %d %v", rt.expectedSkippableErrs, len(actualSkewErrs.Skippable), *rt.vg)
		}
		if len(actualSkewErrs.Mandatory) != rt.expectedMandatoryErrs {
			t.Errorf("failed TestGetAvailableUpgrades\n\texpected mandatory errors: %d\n\tgot mandatory errors: %d %v", rt.expectedMandatoryErrs, len(actualSkewErrs.Mandatory), *rt.vg)
		}
	}
}

func TestWaitForItemsFromChan(t *testing.T) {
	tests := []struct {
		timeout
	}{
		{ // everything ok
			vg: &fakeVersionGetter{
				clusterVersion: "v1.7.3",
				kubeletVersion: "v1.7.3",
				kubeadmVersion: "v1.7.5",
			},
			newK8sVersion: "v1.7.5",
		},
	}

	for _, rt := range tests {

	}
}
*/
