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
