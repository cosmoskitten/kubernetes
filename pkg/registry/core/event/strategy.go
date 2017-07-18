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

package event

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
)

type eventStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that pplies when creating and updating
// Event objects via the REST API.
var Strategy = eventStrategy{api.Scheme, names.SimpleNameGenerator}

func (eventStrategy) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (eventStrategy) NamespaceScoped() bool {
	return true
}

func (eventStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
}

func (eventStrategy) PrepareForUpdate(ctx genericapirequest.Context, obj, old runtime.Object) {
}

func (eventStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	event := obj.(*api.Event)
	return validation.ValidateEvent(event)
}

// Canonicalize normalizes the object after validation.
func (eventStrategy) Canonicalize(obj runtime.Object) {
}

func (eventStrategy) AllowCreateOnUpdate() bool {
	return true
}

func (eventStrategy) ValidateUpdate(ctx genericapirequest.Context, obj, old runtime.Object) field.ErrorList {
	event := obj.(*api.Event)
	return validation.ValidateEvent(event)
}

func (eventStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	event, ok := obj.(*api.Event)
	if !ok {
		return nil, nil, false, fmt.Errorf("not an event")
	}
	return labels.Set(event.Labels), EventToSelectableFields(event), event.Initializers != nil, nil
}

func MatchEvent(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// EventToSelectableFields returns a field set that represents the object
func EventToSelectableFields(event *api.Event) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&event.ObjectMeta, true)
	specificFieldsSet := fields.Set{
		"action.action": event.Action.Action,
		"source":        event.Source.Component,
		"type":          event.Type,
	}
	if event.Object != nil {
		objectSpecificFieldsSet := fields.Set{
			"object.kind":            event.Object.Kind,
			"object.namespace":       event.Object.Namespace,
			"object.name":            event.Object.Name,
			"object.uid":             string(event.Object.UID),
			"object.apiVersion":      event.Object.APIVersion,
			"object.resourceVersion": event.Object.ResourceVersion,
			"object.fieldPath":       event.Object.FieldPath,
		}
		specificFieldsSet = generic.MergeFieldsSets(specificFieldsSet, objectSpecificFieldsSet)
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}
