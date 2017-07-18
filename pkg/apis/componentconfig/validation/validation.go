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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

// ValidateKubeProxyConfiguration validates the configuration of kube-proxy
func ValidateKubeProxyConfiguration(config *componentconfig.KubeProxyConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	newPath := field.NewPath("KubeProxyConfiguration")

	allErrs = append(allErrs, validateKubeProxyIPTablesConfiguration(config.IPTables, newPath.Child("KubeProxyIPTablesConfiguration"))...)
	allErrs = append(allErrs, validateKubeProxyConntrackConfiguration(config.Conntrack, newPath.Child("KubeProxyConntrackConfiguration"))...)
	allErrs = append(allErrs, validateProxyMode(config.Mode, newPath.Child("Mode"))...)

	if config.OOMScoreAdj != nil && (*config.OOMScoreAdj < -1000 || *config.OOMScoreAdj > 1000) {
		allErrs = append(allErrs, field.Invalid(newPath.Child("OOMScoreAdj"), *config.OOMScoreAdj, "must be within the range [-1000, 1000]"))
	}

	if config.UDPIdleTimeout.Duration <= 0 {
		allErrs = append(allErrs, field.Invalid(newPath.Child("UDPIdleTimeout"), config.UDPIdleTimeout, "must be greater than 0"))
	}

	if config.ConfigSyncPeriod.Duration <= 0 {
		allErrs = append(allErrs, field.Invalid(newPath.Child("ConfigSyncPeriod"), config.ConfigSyncPeriod, "must be greater than 0"))
	}

	return allErrs
}

func validateKubeProxyIPTablesConfiguration(config componentconfig.KubeProxyIPTablesConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.MasqueradeBit != nil && (*config.MasqueradeBit < 0 || *config.MasqueradeBit > 31) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("MasqueradeBit"), config.MasqueradeBit, "must be within the range [0, 31]"))
	}

	if config.SyncPeriod.Duration <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("SyncPeriod"), config.SyncPeriod, "must be greater than 0"))
	}

	if config.MinSyncPeriod.Duration <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("MinSyncPeriod"), config.MinSyncPeriod, "must be greater than 0"))
	}

	return allErrs
}

func validateKubeProxyConntrackConfiguration(config componentconfig.KubeProxyConntrackConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.Max < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Max"), config.Max, "cannot be less than 0"))
	}

	if config.MaxPerCore < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("MaxPerCore"), config.MaxPerCore, "cannot be less than 0"))
	}

	if config.Min < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Min"), config.Min, "cannot be less than 0"))
	}

	if config.Max < config.Min {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Min"), config.Min, "cannot be greater than KubeProxyConntrackConfiguration.Max"))
	}

	if config.TCPEstablishedTimeout.Duration <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("TCPEstablishedTimeout"), config.TCPEstablishedTimeout, "must be greater than 0"))
	}

	if config.TCPCloseWaitTimeout.Duration <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("TCPCloseWaitTimeout"), config.TCPCloseWaitTimeout, "must be greater than 0"))
	}

	return allErrs
}

func validateProxyMode(mode componentconfig.ProxyMode, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if mode != componentconfig.ProxyModeUserspace && mode != componentconfig.ProxyModeIPTables && string(mode) != "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ProxyMode"), string(mode), "must be userspace, iptables or blank, blank means the best-available proxy (currently iptables)"))
	}
	return allErrs
}
