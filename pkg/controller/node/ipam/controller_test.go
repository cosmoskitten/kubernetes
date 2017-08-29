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

package ipam

import (
	"testing"

	"k8s.io/kubernetes/pkg/controller/node/ipam/cidrset"
	"k8s.io/kubernetes/pkg/controller/node/ipam/test"
)

func TestOccupyServiceCIDR(t *testing.T) {
	for _, tc := range []struct {
		clusterCIDR string
		serviceCIDR string
	}{
		{"10.1.0.0/16", "10.0.255.0/24"},
		{"10.1.0.0/16", "10.1.0.0/24"},
		{"10.1.0.0/16", "10.1.0.0/24"},
	} {
		set := cidrset.NewCIDRSet(test.MustParseCIDR(tc.clusterCIDR), 24)
		if err := occupyServiceCIDR(set, test.MustParseCIDR(tc.clusterCIDR), test.MustParseCIDR(tc.serviceCIDR)); err != nil {
			t.Errorf("test case %+v: occupyServiceCIDR() = %v, want nil", tc, err)
		}
		// XXX
	}
}
