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
	if !equalLevels(s.opts.SELinuxOptions.Level, seLinux.Level) {
		detail := fmt.Sprintf("seLinuxOptions.level on %s does not match required level.  Found %s, wanted %s", container.Name, seLinux.Level, s.opts.SELinuxOptions.Level)
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

func equalLevels(expectedLevel, actualLevel string) bool {
	if expectedLevel == actualLevel {
		return true
	}

	// "s0:c6,c0" => [ "s0", "c6,c0" ]
	expectedParts := strings.SplitN(expectedLevel, ":", 2)
	actualParts := strings.SplitN(actualLevel, ":", 2)
	if len(expectedParts) != 2 || len(expectedParts) != len(actualParts) {
		return false
	}

	// "s0-s0" => [ "s0" ]
	expectedSensitivity := parseSensitivity(expectedParts[0])
	actualSensitivity := parseSensitivity(actualParts[0])
	if !reflect.DeepEqual(expectedSensitivity, actualSensitivity) {
		return false
	}

	// "c6,c0" => [ "c0", "c6" ]
	expectedCategories := parseCategories(expectedParts[1])
	actualCategories := parseCategories(actualParts[1])

	return reflect.DeepEqual(expectedCategories, actualCategories)
}

func parseSensitivity(sensitivity string) []string {
	// "s0-s0" => [ "s0" ]
	if strings.IndexByte(sensitivity, '-') > -1 {
		sensitivityRange := strings.SplitN(sensitivity, "-", 2)
		if sensitivityRange[0] == sensitivityRange[1] {
			return []string{sensitivityRange[0]}
		}
	}

	return []string{sensitivity}
}

func parseCategories(categories string) []string {
	parts := strings.Split(categories, ",")

	// "c0.c3" => [ "c0", "c1", "c2", c3" ]
	if len(parts) == 1 && strings.IndexByte(categories, '.') > -1 {
		categoryRange := strings.SplitN(categories, ".", 2)
		if len(categoryRange) == 2 && categoryRange[0][0] == 'c' && categoryRange[1][0] == 'c' {
			begin := strings.TrimPrefix(categoryRange[0], "c")
			end := strings.TrimPrefix(categoryRange[1], "c")

			// bitSize 16 because we expect that categories will be in a range [0, 1024)
			from, err1 := strconv.ParseInt(begin, 10, 16)
			to, err2 := strconv.ParseInt(end, 10, 16)
			if err1 == nil && err2 == nil && from < to {
				parts = make([]string, to-from+1)
				for i := from; i <= to; i++ {
					parts[i] = fmt.Sprintf("c%d", i)
				}
			}
		}
	}

	// although sorting digits as strings is leading to wrong order,
	// it doesn't matter because we only need to sort both parts in a similar way
	sort.Strings(parts)

	return parts
}
