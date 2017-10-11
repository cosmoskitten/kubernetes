/*
Copyright 2016 The Kubernetes Authors.

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

package selinux

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

type mustRunAs struct {
	opts *extensions.SELinuxStrategyOptions
}

var _ SELinuxStrategy = &mustRunAs{}

func NewMustRunAs(options *extensions.SELinuxStrategyOptions) (SELinuxStrategy, error) {
	if options == nil {
		return nil, fmt.Errorf("MustRunAs requires SELinuxContextStrategyOptions")
	}
	if options.SELinuxOptions == nil {
		return nil, fmt.Errorf("MustRunAs requires SELinuxOptions")
	}
	return &mustRunAs{
		opts: options,
	}, nil
}

// Generate creates the SELinuxOptions based on constraint rules.
func (s *mustRunAs) Generate(pod *api.Pod, container *api.Container) (*api.SELinuxOptions, error) {
	return s.opts.SELinuxOptions, nil
}

// Validate ensures that the specified values fall within the range of the strategy.
func (s *mustRunAs) Validate(pod *api.Pod, container *api.Container) field.ErrorList {
	allErrs := field.ErrorList{}

	if container.SecurityContext == nil {
		detail := fmt.Sprintf("unable to validate nil security context for %s", container.Name)
		allErrs = append(allErrs, field.Invalid(field.NewPath("securityContext"), container.SecurityContext, detail))
		return allErrs
	}
	if container.SecurityContext.SELinuxOptions == nil {
		detail := fmt.Sprintf("unable to validate nil seLinuxOptions for %s", container.Name)
		allErrs = append(allErrs, field.Invalid(field.NewPath("seLinuxOptions"), container.SecurityContext.SELinuxOptions, detail))
		return allErrs
	}
	seLinuxOptionsPath := field.NewPath("seLinuxOptions")
	seLinux := container.SecurityContext.SELinuxOptions
	if result, err := equalLevels(s.opts.SELinuxOptions.Level, seLinux.Level); err != nil || !result {
		detail := fmt.Sprintf("seLinuxOptions.level on %s does not match required level.  Found %s, wanted %s", container.Name, seLinux.Level, s.opts.SELinuxOptions.Level)
		if err != nil {
			detail = fmt.Sprintf("%s, error: %v", detail, err)
		}
		allErrs = append(allErrs, field.Invalid(seLinuxOptionsPath.Child("level"), seLinux.Level, detail))
	}
	if seLinux.Role != s.opts.SELinuxOptions.Role {
		detail := fmt.Sprintf("seLinuxOptions.role on %s does not match required role.  Found %s, wanted %s", container.Name, seLinux.Role, s.opts.SELinuxOptions.Role)
		allErrs = append(allErrs, field.Invalid(seLinuxOptionsPath.Child("role"), seLinux.Role, detail))
	}
	if seLinux.Type != s.opts.SELinuxOptions.Type {
		detail := fmt.Sprintf("seLinuxOptions.type on %s does not match required type.  Found %s, wanted %s", container.Name, seLinux.Type, s.opts.SELinuxOptions.Type)
		allErrs = append(allErrs, field.Invalid(seLinuxOptionsPath.Child("type"), seLinux.Type, detail))
	}
	if seLinux.User != s.opts.SELinuxOptions.User {
		detail := fmt.Sprintf("seLinuxOptions.user on %s does not match required user.  Found %s, wanted %s", container.Name, seLinux.User, s.opts.SELinuxOptions.User)
		allErrs = append(allErrs, field.Invalid(seLinuxOptionsPath.Child("user"), seLinux.User, detail))
	}

	return allErrs
}

func equalLevels(expectedLevel, actualLevel string) (bool, error) {
	if expectedLevel == actualLevel {
		return true, nil
	}

	// Level format: https://selinuxproject.org/page/MLSStatements#level

	// "s0:c6,c0" => [ "s0", "c6,c0" ]
	expectedParts := strings.SplitN(expectedLevel, ":", 2)
	actualParts := strings.SplitN(actualLevel, ":", 2)

	// "s0-s0" => [ "s0" ]
	expectedSensitivity, err := canonicalizeSensitivity(expectedParts[0])
	if err != nil {
		return false, err
	}

	actualSensitivity, err := canonicalizeSensitivity(actualParts[0])
	if err != nil {
		return false, err
	}

	if !reflect.DeepEqual(expectedSensitivity, actualSensitivity) {
		return false, nil
	}

	expectedPartsLen := len(expectedParts)
	actualPartsLen := len(actualParts)

	// both levels don't have categories and equal ("s0" and "s0")
	if expectedPartsLen == 1 && actualPartsLen == 1 {
		return true, nil
	}

	// "s0" != "s0:c1"
	if expectedPartsLen != actualPartsLen {
		return false, nil
	}

	// "c6,c0" => [ "c0", "c6" ]
	expectedCategories, err := canonicalizeCategories(expectedParts[1])
	if err != nil {
		return false, err
	}
	actualCategories, err := canonicalizeCategories(actualParts[1])
	if err != nil {
		return false, err
	}

	// TODO: add support for dominance
	// See: https://selinuxproject.org/page/NB_MLS#Managing_Security_Levels_via_Dominance_Rules
	return reflect.DeepEqual(expectedCategories, actualCategories), nil
}

// Parses and canonicalize a sensitivity. Performs expansion (s0-s1 => s0,s1)
// and simplification (s0-s0 => s0) if needed.
func canonicalizeSensitivity(sensitivity string) ([]string, error) {
	return canonicalizeItems(sensitivity, ",", "-", "s")
}

// Parses and canonicalize a categories. Performs expansion (c0-c1 => c0,c1) if needed.
func canonicalizeCategories(categories string) ([]string, error) {
	return canonicalizeItems(categories, ",", ".", "c")
}

func canonicalizeItems(str, itemsSeparator, rangeSeparator, boundaryPrefix string) ([]string, error) {
	parts := strings.Split(str, itemsSeparator)
	result := make([]string, 0, len(parts))

	for _, value := range parts {
		if strings.Index(value, rangeSeparator) == -1 {
			// it's not a range, add it as-is
			result = append(result, value)
			continue
		}

		from, to, err := parseBoundaries(value, rangeSeparator, boundaryPrefix)
		if err != nil {
			return nil, fmt.Errorf("could not parse %q: %v", value, err)
		}

		for from <= to {
			result = append(result, fmt.Sprintf("%s%d", boundaryPrefix, from))
			from++
		}
	}

	// although sorting digits as strings is leading to a wrong order,
	// it doesn't matter because we only need to sort both parts in a similar way
	sort.Strings(result)

	return result, nil
}

// Parses a string with a range in a format "$prefix$begin$separator$prefix$end" by extracting begin and end of a range.
func parseBoundaries(str, separator, prefix string) (int64, int64, error) {
	strRange := strings.SplitN(str, separator, 2)

	begin, err := parseBoundary(strRange[0], prefix)
	if err != nil {
		return -1, -1, fmt.Errorf("invalid initial of a boundary: %v", err)
	}

	end, err := parseBoundary(strRange[1], prefix)
	if err != nil {
		return -1, -1, fmt.Errorf("invalid end of a boundary: %v", err)
	}

	if begin > end {
		return -1, -1, fmt.Errorf("initial of a boundary must be less than end of a boundary")
	}

	return begin, end, nil
}

// Parses a string with a range boundary in a format "$prefix$value" by extracting value and converting it to an integer.
func parseBoundary(str, prefix string) (int64, error) {
	if !strings.HasPrefix(str, prefix) {
		return -1, fmt.Errorf("must be prefixed with %q", prefix)
	}

	value := strings.TrimPrefix(str, prefix)
	return strconv.ParseInt(value, 10, 16)
}
