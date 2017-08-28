package testing

import (
	"fmt"
	"testing"

	"k8s.io/apiserver/pkg/util/feature"
)

// SetFeatureGateDuringTest sets the specified gate to the specified value, and returns a function that restores the original value.
// Failures to set or restore cause the test to fail.
func SetFeatureGateDuringTest(t *testing.T, gate feature.FeatureGate, feature feature.Feature, value bool) func() {
	originalValue := gate.Enabled(feature)

	if err := gate.Set(fmt.Sprintf("%s=%v", feature, value)); err != nil {
		t.Errorf("error setting %s=%v: %v", feature, value, err)
	}

	return func() {
		if err := gate.Set(fmt.Sprintf("%s=%v", feature, originalValue)); err != nil {
			t.Errorf("error restoring %s=%v: %v", feature, originalValue, err)
		}
	}
}
