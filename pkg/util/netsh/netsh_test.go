package netsh

import (
	"testing"
)

func TestGetIP(t *testing.T) {
	testcases := []struct {
		showAddress   string
		expectAddress string
	}{
		{
			showAddress:   "IP 地址:                           10.96.0.2",
			expectAddress: "10.96.0.2",
		},
		{
			showAddress:   "IP Address:                           10.96.0.3",
			expectAddress: "10.96.0.3",
		},
		{
			showAddress:   "IP Address:10.96.0.4",
			expectAddress: "10.96.0.4",
		},
	}

	for _, tc := range testcases {
		address := getIP(tc.showAddress)
		if address != tc.expectAddress {
			t.Errorf("expected address=%q, got %q", tc.expectAddress, address)
		}
	}
}
