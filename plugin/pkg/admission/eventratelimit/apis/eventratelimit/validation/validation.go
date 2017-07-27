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
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/validation/field"

	eventratelimitapi "k8s.io/kubernetes/plugin/pkg/admission/eventratelimit/apis/eventratelimit"
)

var LimitTypes map[eventratelimitapi.LimitType]bool = map[eventratelimitapi.LimitType]bool{
	eventratelimitapi.ServerLimitType:       true,
	eventratelimitapi.NamespaceLimitType:    true,
	eventratelimitapi.UserLimitType:         true,
	eventratelimitapi.SourceObjectLimitType: true,
}

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *eventratelimitapi.Configuration) field.ErrorList {
	allErrs := field.ErrorList{}
	limitsPath := field.NewPath("limits")
	if len(config.Limits) == 0 {
		allErrs = append(allErrs, field.Invalid(limitsPath, config.Limits, "must not be empty"))
	}
	for i, limit := range config.Limits {
		idxPath := limitsPath.Index(i)
		if !LimitTypes[limit.Type] {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("type"), limit.Type, "unknown type"))
		}
		if limit.Burst <= 0 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("burst"), limit.Burst, "must be positive"))
		}
		if limit.Burst > math.MaxInt32 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("burst"), limit.Burst, fmt.Sprintf("must not be greater than %v", math.MaxInt32)))
		}
		if limit.QPS <= 0 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("qps"), limit.QPS, "must be positive"))
		}
		if limit.Type != eventratelimitapi.ServerLimitType {
			if limit.CacheSize < 0 {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("cacheSize"), limit.CacheSize, "must be not be negative"))
			}
			if limit.CacheSize > math.MaxInt32 {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("cacheSize"), limit.CacheSize, fmt.Sprintf("must not be greater than %v", math.MaxInt32)))
			}
		}
	}
	return allErrs
}
