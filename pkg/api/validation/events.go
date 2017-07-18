/*
Copyright 2014 The Kubernetes Authors.

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
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api"
	apiutil "k8s.io/kubernetes/pkg/api/util"
)

// ValidateEvent makes sure that the event makes sense.
func ValidateEvent(event *api.Event) field.ErrorList {
	allErrs := field.ErrorList{}
	zeroTime := time.Time{}

	// "New" Events need to have EventTime set, so it's validating old object.
	if event.EventTime.Time == zeroTime {
		// Make sure event.Namespace and the involvedObject.Namespace agree
		if len(event.InvolvedObject.Namespace) == 0 {
			// event.Namespace must also be empty (or "default", for compatibility with old clients)
			if event.Namespace != metav1.NamespaceNone && event.Namespace != metav1.NamespaceDefault {
				allErrs = append(allErrs, field.Invalid(field.NewPath("involvedObject", "namespace"), event.InvolvedObject.Namespace, "does not match event.namespace"))
			}
		} else {
			// event namespace must match
			if event.Namespace != event.InvolvedObject.Namespace {
				allErrs = append(allErrs, field.Invalid(field.NewPath("involvedObject", "namespace"), event.InvolvedObject.Namespace, "does not match event.namespace"))
			}
		}

		// For kinds we recognize, make sure involvedObject.Namespace is set for namespaced kinds
		if namespaced, err := isNamespacedKind(event.InvolvedObject.Kind, event.InvolvedObject.APIVersion); err == nil {
			if namespaced && len(event.InvolvedObject.Namespace) == 0 {
				allErrs = append(allErrs, field.Required(field.NewPath("involvedObject", "namespace"), fmt.Sprintf("required for kind %s", event.InvolvedObject.Kind)))
			}
			if !namespaced && len(event.InvolvedObject.Namespace) > 0 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("involvedObject", "namespace"), event.InvolvedObject.Namespace, fmt.Sprintf("not allowed for kind %s", event.InvolvedObject.Kind)))
			}
		}

		for _, msg := range validation.IsDNS1123Subdomain(event.Namespace) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("namespace"), event.Namespace, msg))
		}
	} else {
		// Check if basic fields match
		if event.Object != nil {
			if !apiequality.Semantic.DeepEqual(*event.Object, event.InvolvedObject) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("object"), event.Object, "does not match event.involvedObject"))
			}
		}
		if event.Series != nil && event.Series.Count != event.Count {
			allErrs = append(allErrs, field.Invalid(field.NewPath("series", "count"), event.Series.Count, "does not match event.count"))
		}
		if event.Origin.Component != event.Source.Component || event.Origin.Host != event.Source.Host {
			allErrs = append(allErrs, field.Invalid(field.NewPath("origin"), event.Origin, "does not match event.source"))
		}
		if event.Action.Action != event.Reason {
			allErrs = append(allErrs, field.Invalid(field.NewPath("action", "action"), event.Action.Action, "does not match event.reason"))
		}
	}
	return allErrs
}

// Check whether the kind in groupVersion is scoped at the root of the api hierarchy
func isNamespacedKind(kind, groupVersion string) (bool, error) {
	group := apiutil.GetGroup(groupVersion)
	g, err := api.Registry.Group(group)
	if err != nil {
		return false, err
	}
	restMapping, err := g.RESTMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind}, apiutil.GetVersion(groupVersion))
	if err != nil {
		return false, err
	}
	scopeName := restMapping.Scope.Name()
	if scopeName == meta.RESTScopeNameNamespace {
		return true, nil
	}
	return false, nil
}
