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
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

func TestValidateKubeProxyIPTablesConfiguration(t *testing.T) {
	valid := int32(5)
	successCases := []componentconfig.KubeProxyIPTablesConfiguration{
		{
			MasqueradeAll: true,
			SyncPeriod:    metav1.Duration{Duration: 5 * time.Second},
			MinSyncPeriod: metav1.Duration{Duration: 2 * time.Second},
		},
		{
			MasqueradeBit: &valid,
			MasqueradeAll: true,
			SyncPeriod:    metav1.Duration{Duration: 5 * time.Second},
			MinSyncPeriod: metav1.Duration{Duration: 2 * time.Second},
		},
	}
	newPath := field.NewPath("KubeProxyConfiguration")
	for _, successCase := range successCases {
		if errs := validateKubeProxyIPTablesConfiguration(successCase, newPath.Child("KubeProxyIPTablesConfiguration")); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}

	invalid := int32(-10)
	errorCases := []struct {
		config componentconfig.KubeProxyIPTablesConfiguration
		msg    string
	}{
		{
			config: componentconfig.KubeProxyIPTablesConfiguration{
				MasqueradeAll: true,
				SyncPeriod:    metav1.Duration{Duration: -5 * time.Second},
				MinSyncPeriod: metav1.Duration{Duration: 2 * time.Second},
			},
			msg: "must be greater than 0",
		},
		{
			config: componentconfig.KubeProxyIPTablesConfiguration{
				MasqueradeBit: &valid,
				MasqueradeAll: true,
				SyncPeriod:    metav1.Duration{Duration: 5 * time.Second},
				MinSyncPeriod: metav1.Duration{Duration: 0 * time.Second},
			},
			msg: "must be greater than 0",
		},
		{
			config: componentconfig.KubeProxyIPTablesConfiguration{
				MasqueradeBit: &invalid,
				MasqueradeAll: true,
				SyncPeriod:    metav1.Duration{Duration: 5 * time.Second},
				MinSyncPeriod: metav1.Duration{Duration: 2 * time.Second},
			},
			msg: "must be within the range [0, 31]",
		},
	}

	for _, errorCase := range errorCases {
		if errs := validateKubeProxyIPTablesConfiguration(errorCase.config, newPath.Child("KubeProxyIPTablesConfiguration")); len(errs) == 0 {
			t.Errorf("expected failure for %s", errorCase.msg)
		} else if !strings.Contains(errs[0].Error(), errorCase.msg) {
			t.Errorf("unexpected error: %v, expected: %s", errs[0], errorCase.msg)
		}
	}
}

func TestValidateKubeProxyConntrackConfiguration(t *testing.T) {
	successCases := []componentconfig.KubeProxyConntrackConfiguration{
		{
			Max:        int32(2),
			MaxPerCore: int32(1),
			Min:        int32(1),
			TCPEstablishedTimeout: metav1.Duration{Duration: 5 * time.Second},
			TCPCloseWaitTimeout:   metav1.Duration{Duration: 5 * time.Second},
		},
		{
			Max:        0,
			MaxPerCore: 0,
			Min:        0,
			TCPEstablishedTimeout: metav1.Duration{Duration: 5 * time.Second},
			TCPCloseWaitTimeout:   metav1.Duration{Duration: 60 * time.Second},
		},
	}
	newPath := field.NewPath("KubeProxyConfiguration")
	for _, successCase := range successCases {
		if errs := validateKubeProxyConntrackConfiguration(successCase, newPath.Child("KubeProxyConntrackConfiguration")); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}

	errorCases := []struct {
		config componentconfig.KubeProxyConntrackConfiguration
		msg    string
	}{
		{
			config: componentconfig.KubeProxyConntrackConfiguration{
				Max:        int32(-1),
				MaxPerCore: int32(1),
				Min:        int32(1),
				TCPEstablishedTimeout: metav1.Duration{Duration: 5 * time.Second},
				TCPCloseWaitTimeout:   metav1.Duration{Duration: 5 * time.Second},
			},
			msg: "cannot be less than 0",
		},
		{
			config: componentconfig.KubeProxyConntrackConfiguration{
				Max:        int32(2),
				MaxPerCore: int32(-1),
				Min:        int32(1),
				TCPEstablishedTimeout: metav1.Duration{Duration: 5 * time.Second},
				TCPCloseWaitTimeout:   metav1.Duration{Duration: 5 * time.Second},
			},
			msg: "cannot be less than 0",
		},
		{
			config: componentconfig.KubeProxyConntrackConfiguration{
				Max:        int32(2),
				MaxPerCore: int32(1),
				Min:        int32(-1),
				TCPEstablishedTimeout: metav1.Duration{Duration: 5 * time.Second},
				TCPCloseWaitTimeout:   metav1.Duration{Duration: 5 * time.Second},
			},
			msg: "cannot be less than 0",
		},
		{
			config: componentconfig.KubeProxyConntrackConfiguration{
				Max:        int32(2),
				MaxPerCore: int32(1),
				Min:        int32(3),
				TCPEstablishedTimeout: metav1.Duration{Duration: 5 * time.Second},
				TCPCloseWaitTimeout:   metav1.Duration{Duration: 5 * time.Second},
			},
			msg: "cannot be greater than KubeProxyConntrackConfiguration.Max",
		},
		{
			config: componentconfig.KubeProxyConntrackConfiguration{
				Max:        int32(4),
				MaxPerCore: int32(1),
				Min:        int32(3),
				TCPEstablishedTimeout: metav1.Duration{Duration: -5 * time.Second},
				TCPCloseWaitTimeout:   metav1.Duration{Duration: 5 * time.Second},
			},
			msg: "must be greater than 0",
		},
		{
			config: componentconfig.KubeProxyConntrackConfiguration{
				Max:        int32(4),
				MaxPerCore: int32(1),
				Min:        int32(3),
				TCPEstablishedTimeout: metav1.Duration{Duration: 5 * time.Second},
				TCPCloseWaitTimeout:   metav1.Duration{Duration: -5 * time.Second},
			},
			msg: "must be greater than 0",
		},
	}

	for _, errorCase := range errorCases {
		if errs := validateKubeProxyConntrackConfiguration(errorCase.config, newPath.Child("KubeProxyConntrackConfiguration")); len(errs) == 0 {
			t.Errorf("expected failure for %s", errorCase.msg)
		} else if !strings.Contains(errs[0].Error(), errorCase.msg) {
			t.Errorf("unexpected error: %v, expected: %s", errs[0], errorCase.msg)
		}
	}
}

func TestValidateProxyMode(t *testing.T) {
	newPath := field.NewPath("KubeProxyConfiguration")

	successCases := []componentconfig.ProxyMode{
		componentconfig.ProxyModeUserspace,
		componentconfig.ProxyModeIPTables,
		componentconfig.ProxyMode(""),
	}

	for _, successCase := range successCases {
		if errs := validateProxyMode(successCase, newPath.Child("ProxyMode")); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}

	errorCases := []struct {
		mode componentconfig.ProxyMode
		msg  string
	}{
		{
			mode: componentconfig.ProxyMode("non-existing"),
			msg:  "must be userspace, iptables or blank, blank means the best-available proxy (currently iptables)",
		},
	}

	for _, errorCase := range errorCases {
		if errs := validateProxyMode(errorCase.mode, newPath.Child("ProxyMode")); len(errs) == 0 {
			t.Errorf("expected failure for %s", errorCase.msg)
		} else if !strings.Contains(errs[0].Error(), errorCase.msg) {
			t.Errorf("unexpected error: %v, expected: %s", errs[0], errorCase.msg)
		}
	}
}
