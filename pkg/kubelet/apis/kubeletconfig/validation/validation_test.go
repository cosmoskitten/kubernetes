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

package validation

import (
	"testing"

	"k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig"
)

func TestValidateKubeletConfiguration(t *testing.T) {
	successCase := &kubeletconfig.KubeletConfiguration{
		CgroupsPerQOS:               true,
		EnforceNodeAllocatable:      []string{"pods"},
		SystemCgroups:               "",
		CgroupRoot:                  "",
		CAdvisorPort:                4194,
		EventBurst:                  10,
		EventRecordQPS:              5,
		HealthzPort:                 10248,
		ImageGCHighThresholdPercent: 85,
		ImageGCLowThresholdPercent:  80,
		IPTablesDropBit:             15,
		IPTablesMasqueradeBit:       14,
		KubeAPIBurst:                10,
		KubeAPIQPS:                  5,
		MaxOpenFiles:                1000000,
		MaxPods:                     110,
		OOMScoreAdj:                 -999,
		PodsPerCore:                 100,
		Port:                        10250,
		ReadOnlyPort:                10255,
		RegistryBurst:               10,
		RegistryPullQPS:             5,
	}
	if err := ValidateKubeletConfiguration(successCase); err != nil {
		t.Errorf("expect no errors got %s", err)
	}

	errorCase := &kubeletconfig.KubeletConfiguration{
		CgroupsPerQOS:               false,
		EnforceNodeAllocatable:      []string{"pods"},
		SystemCgroups:               "/",
		CgroupRoot:                  "",
		CAdvisorPort:                -10,
		EventBurst:                  -10,
		EventRecordQPS:              -10,
		HealthzPort:                 -10,
		ImageGCHighThresholdPercent: 101,
		ImageGCLowThresholdPercent:  101,
		IPTablesDropBit:             -10,
		IPTablesMasqueradeBit:       -10,
		KubeAPIBurst:                -10,
		KubeAPIQPS:                  -10,
		MaxOpenFiles:                -10,
		MaxPods:                     -10,
		OOMScoreAdj:                 -1001,
		PodsPerCore:                 -10,
		Port:                        -10,
		ReadOnlyPort:                -10,
		RegistryBurst:               -10,
		RegistryPullQPS:             -10,
	}
	if err := ValidateKubeletConfiguration(errorCase); err == nil {
		t.Errorf("expect errors got %v", err)
	}
}
