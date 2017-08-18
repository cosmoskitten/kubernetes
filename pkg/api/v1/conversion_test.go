/*
Copyright 2015 The Kubernetes Authors.

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

package v1_test

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/kubernetes/pkg/api"
	k8s_api_v1 "k8s.io/kubernetes/pkg/api/v1"
)

func TestPodLogOptions(t *testing.T) {
	sinceSeconds := int64(1)
	sinceTime := metav1.NewTime(time.Date(2000, 1, 1, 12, 34, 56, 0, time.UTC).Local())
	tailLines := int64(2)
	limitBytes := int64(3)

	versionedLogOptions := &v1.PodLogOptions{
		Container:    "mycontainer",
		Follow:       true,
		Previous:     true,
		SinceSeconds: &sinceSeconds,
		SinceTime:    &sinceTime,
		Timestamps:   true,
		TailLines:    &tailLines,
		LimitBytes:   &limitBytes,
	}
	unversionedLogOptions := &api.PodLogOptions{
		Container:    "mycontainer",
		Follow:       true,
		Previous:     true,
		SinceSeconds: &sinceSeconds,
		SinceTime:    &sinceTime,
		Timestamps:   true,
		TailLines:    &tailLines,
		LimitBytes:   &limitBytes,
	}
	expectedParameters := url.Values{
		"container":    {"mycontainer"},
		"follow":       {"true"},
		"previous":     {"true"},
		"sinceSeconds": {"1"},
		"sinceTime":    {"2000-01-01T12:34:56Z"},
		"timestamps":   {"true"},
		"tailLines":    {"2"},
		"limitBytes":   {"3"},
	}

	codec := runtime.NewParameterCodec(api.Scheme)

	// unversioned -> query params
	{
		actualParameters, err := codec.EncodeParameters(unversionedLogOptions, v1.SchemeGroupVersion)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actualParameters, expectedParameters) {
			t.Fatalf("Expected\n%#v\ngot\n%#v", expectedParameters, actualParameters)
		}
	}

	// versioned -> query params
	{
		actualParameters, err := codec.EncodeParameters(versionedLogOptions, v1.SchemeGroupVersion)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actualParameters, expectedParameters) {
			t.Fatalf("Expected\n%#v\ngot\n%#v", expectedParameters, actualParameters)
		}
	}

	// query params -> versioned
	{
		convertedLogOptions := &v1.PodLogOptions{}
		err := codec.DecodeParameters(expectedParameters, v1.SchemeGroupVersion, convertedLogOptions)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(convertedLogOptions, versionedLogOptions) {
			t.Fatalf("Unexpected deserialization:\n%s", diff.ObjectGoPrintSideBySide(versionedLogOptions, convertedLogOptions))
		}
	}

	// query params -> unversioned
	{
		convertedLogOptions := &api.PodLogOptions{}
		err := codec.DecodeParameters(expectedParameters, v1.SchemeGroupVersion, convertedLogOptions)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(convertedLogOptions, unversionedLogOptions) {
			t.Fatalf("Unexpected deserialization:\n%s", diff.ObjectGoPrintSideBySide(unversionedLogOptions, convertedLogOptions))
		}
	}
}

// TestPodSpecConversion tests that v1.ServiceAccount is an alias for
// ServiceAccountName.
func TestPodSpecConversion(t *testing.T) {
	name, other := "foo", "bar"

	// Test internal -> v1. Should have both alias (DeprecatedServiceAccount)
	// and new field (ServiceAccountName).
	i := &api.PodSpec{
		ServiceAccountName: name,
	}
	v := v1.PodSpec{}
	if err := api.Scheme.Convert(i, &v, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.ServiceAccountName != name {
		t.Fatalf("want v1.ServiceAccountName %q, got %q", name, v.ServiceAccountName)
	}
	if v.DeprecatedServiceAccount != name {
		t.Fatalf("want v1.DeprecatedServiceAccount %q, got %q", name, v.DeprecatedServiceAccount)
	}

	// Test v1 -> internal. Either DeprecatedServiceAccount, ServiceAccountName,
	// or both should translate to ServiceAccountName. ServiceAccountName wins
	// if both are set.
	testCases := []*v1.PodSpec{
		// New
		{ServiceAccountName: name},
		// Alias
		{DeprecatedServiceAccount: name},
		// Both: same
		{ServiceAccountName: name, DeprecatedServiceAccount: name},
		// Both: different
		{ServiceAccountName: name, DeprecatedServiceAccount: other},
	}
	for k, v := range testCases {
		got := api.PodSpec{}
		err := api.Scheme.Convert(v, &got, nil)
		if err != nil {
			t.Fatalf("unexpected error for case %d: %v", k, err)
		}
		if got.ServiceAccountName != name {
			t.Fatalf("want api.ServiceAccountName %q, got %q", name, got.ServiceAccountName)
		}
	}
}

func TestResourceListConversion(t *testing.T) {
	bigMilliQuantity := resource.NewQuantity(resource.MaxMilliValue, resource.DecimalSI)
	bigMilliQuantity.Add(resource.MustParse("12345m"))

	tests := []struct {
		input    v1.ResourceList
		expected api.ResourceList
	}{
		{ // No changes necessary.
			input: v1.ResourceList{
				v1.ResourceMemory:  resource.MustParse("30M"),
				v1.ResourceCPU:     resource.MustParse("100m"),
				v1.ResourceStorage: resource.MustParse("1G"),
			},
			expected: api.ResourceList{
				api.ResourceMemory:  resource.MustParse("30M"),
				api.ResourceCPU:     resource.MustParse("100m"),
				api.ResourceStorage: resource.MustParse("1G"),
			},
		},
		{ // Nano-scale values should be rounded up to milli-scale.
			input: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("3.000023m"),
				v1.ResourceMemory: resource.MustParse("500.000050m"),
			},
			expected: api.ResourceList{
				api.ResourceCPU:    resource.MustParse("4m"),
				api.ResourceMemory: resource.MustParse("501m"),
			},
		},
		{ // Large values should still be accurate.
			input: v1.ResourceList{
				v1.ResourceCPU:     *bigMilliQuantity.Copy(),
				v1.ResourceStorage: *bigMilliQuantity.Copy(),
			},
			expected: api.ResourceList{
				api.ResourceCPU:     *bigMilliQuantity.Copy(),
				api.ResourceStorage: *bigMilliQuantity.Copy(),
			},
		},
	}

	for i, test := range tests {
		output := api.ResourceList{}

		// defaulting is a separate step from conversion that is applied when reading from the API or from etcd.
		// perform that step explicitly.
		k8s_api_v1.SetDefaults_ResourceList(&test.input)

		err := api.Scheme.Convert(&test.input, &output, nil)
		if err != nil {
			t.Fatalf("unexpected error for case %d: %v", i, err)
		}
		if !apiequality.Semantic.DeepEqual(test.expected, output) {
			t.Errorf("unexpected conversion for case %d: Expected\n%+v;\nGot\n%+v", i, test.expected, output)
		}
	}
}

func TestGenericFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector metadata.namespace",
			labelName: "metadata.namespace",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.GenericFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}
}

func TestPodFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector metadata.namespace",
			labelName: "metadata.namespace",
			value:     "bar",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector metadata.uid",
			labelName: "metadata.uid",
			value:     "baz",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector spec.nodeName",
			labelName: "spec.nodeName",
			value:     "node1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector spec.restartPolicy",
			labelName: "spec.restartPolicy",
			value:     "Always",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector spec.serviceAccountName",
			labelName: "spec.serviceAccountName",
			value:     "svc1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector status.hostIP",
			labelName: "status.hostIP",
			value:     "1.2.3.4",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector status.phase",
			labelName: "status.phase",
			value:     "ph1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector status.podIP",
			labelName: "status.podIP",
			value:     "4.5.6.7",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector spec.subDomain",
			labelName: "spec.subDomain",
			value:     "domain1",
			expectErr: true,
			errMsg:    `field label "spec.subDomain" not supported for "Pod"`,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.PodFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}

}

func TestNodeFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector spec.unschedulable",
			labelName: "spec.unschedulable",
			value:     "false",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.NodeFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}
}

func TestReplicationControllerFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector metadata.namespace",
			labelName: "metadata.namespace",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector status.replicas",
			labelName: "status.replicas",
			value:     "1",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.ReplicationControllerFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}
}

func TestPersistentVolumeFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector name",
			labelName: "name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.PersistentVolumeFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}
}

func TestEventFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector involvedObject.apiVersion",
			labelName: "involvedObject.apiVersion",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector involvedObject.fieldPath",
			labelName: "involvedObject.fieldPath",
			value:     "bar",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector involvedObject.kind",
			labelName: "involvedObject.kind",
			value:     "baz",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector involvedObject.name",
			labelName: "involvedObject.name",
			value:     "name1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector involvedObject.namespace",
			labelName: "involvedObject.namespace",
			value:     "ns1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector involvedObject.resourceVersion",
			labelName: "involvedObject.resourceVersion",
			value:     "1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector involvedObject.uid",
			labelName: "involvedObject.uid",
			value:     "uid1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector metadata.namespace",
			labelName: "metadata.namespace",
			value:     "ns1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector reason",
			labelName: "reason",
			value:     "reason1",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector type",
			labelName: "type",
			value:     "type1",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.EventFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}
}

func TestNamespaceFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector name",
			labelName: "name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector status.phase",
			labelName: "status.phase",
			value:     "ph1",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.NamespaceFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}
}

func TestSecretFieldLabelConversionFunc(t *testing.T) {
	testcases := []struct {
		name      string
		labelName string
		value     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid fieldSelector metadata.name",
			labelName: "metadata.name",
			value:     "foo",
			expectErr: false,
		},
		{
			name:      "valid fieldSelector type",
			labelName: "type",
			value:     "type1",
			expectErr: false,
		},
		{
			name:      "invalid fieldSelector abc.xyz",
			labelName: "abc.xyz",
			value:     "foo",
			expectErr: true,
			errMsg:    `field label "abc.xyz" not supported`,
		},
	}

	for _, tc := range testcases {
		label, value, err := k8s_api_v1.SecretFieldLabelConversionFunc(tc.labelName, tc.value)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%q : unexpected err for fieldSelector %q", tc.name, tc.labelName)
			} else if err.Error() != tc.errMsg {
				t.Errorf("%q : unexpected error message for fieldSelector %q", tc.name, tc.errMsg)
			}
		} else {
			if label != tc.labelName {
				t.Errorf("%q: epxected label %q, got %q", tc.name, tc.labelName, label)
			}
			if value != tc.value {
				t.Errorf("%q: epxected value %q, got %q", tc.name, tc.value, value)
			}
		}
	}
}
