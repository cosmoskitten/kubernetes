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

package v1

import (
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/api"
)

// PodToSelectableFields returns a field set that represents the object
// TODO: fields are not labels, and the validation rules for them do not apply.
func PodToSelectableFields(pod *api.Pod) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	labels, _ := json.Marshal(pod.ObjectMeta.Labels)
	annotations, _ := json.Marshal(pod.ObjectMeta.Annotations)
	specificFieldsSet := fields.Set{
		"metadata.uid":            string(pod.ObjectMeta.UID),
		"metadata.labels":         string(labels),
		"metadata.annotations":    string(annotations),
		"spec.nodeName":           string(pod.Spec.NodeName),
		"spec.restartPolicy":      string(pod.Spec.RestartPolicy),
		"spec.schedulerName":      string(pod.Spec.SchedulerName),
		"status.phase":            string(pod.Status.Phase),
		"status.podIP":            string(pod.Status.PodIP),
		"status.hostIP":           string(pod.Status.HostIP),
		"spec.serviceAccountName": string(pod.Spec.ServiceAccountName),
	}
	return generic.AddObjectMetaFieldsSet(specificFieldsSet, &pod.ObjectMeta, true)
}

// NodeToSelectableFields returns a field set that represents the object.
func NodeToSelectableFields(node *api.Node) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&node.ObjectMeta, false)
	specificFieldsSet := fields.Set{
		"spec.unschedulable": fmt.Sprint(node.Spec.Unschedulable),
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}

// ControllerToSelectableFields returns a field set that represents the object.
func ControllerToSelectableFields(controller *api.ReplicationController) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&controller.ObjectMeta, true)
	controllerSpecificFieldsSet := fields.Set{
		"status.replicas": strconv.Itoa(int(controller.Status.Replicas)),
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, controllerSpecificFieldsSet)
}

// PersistentVolumeToSelectableFields returns a field set that represents the object
func PersistentVolumeToSelectableFields(persistentvolume *api.PersistentVolume) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&persistentvolume.ObjectMeta, false)
	specificFieldsSet := fields.Set{
		// This is a bug, but we need to support it for backward compatibility.
		"name": persistentvolume.Name,
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}

// EventToSelectableFields returns a field set that represents the object
func EventToSelectableFields(event *api.Event) fields.Set {
	specificFieldsSet := fields.Set{
		"involvedObject.kind":            event.InvolvedObject.Kind,
		"involvedObject.namespace":       event.InvolvedObject.Namespace,
		"involvedObject.name":            event.InvolvedObject.Name,
		"involvedObject.uid":             string(event.InvolvedObject.UID),
		"involvedObject.apiVersion":      event.InvolvedObject.APIVersion,
		"involvedObject.resourceVersion": event.InvolvedObject.ResourceVersion,
		"involvedObject.fieldPath":       event.InvolvedObject.FieldPath,
		"reason":                         event.Reason,
		"source":                         event.Source.Component,
		"type":                           event.Type,
	}
	return generic.AddObjectMetaFieldsSet(specificFieldsSet, &event.ObjectMeta, true)
}

// NamespaceToSelectableFields returns a field set that represents the object
func NamespaceToSelectableFields(namespace *api.Namespace) fields.Set {
	specificFieldsSet := fields.Set{
		"status.phase": string(namespace.Status.Phase),
		// This is a bug, but we need to support it for backward compatibility.
		"name": namespace.Name,
	}
	return generic.AddObjectMetaFieldsSet(specificFieldsSet, &namespace.ObjectMeta, false)
}

// SecretToSelectableFields returns a field set that represents the object
func SecretToSelectableFields(secret *api.Secret) fields.Set {
	specificFieldsSet := fields.Set{
		"type": string(secret.Type),
	}
	return generic.AddObjectMetaFieldsSet(specificFieldsSet, &secret.ObjectMeta, false)
}

// PersistentVolumeClaimToSelectableFields returns a field set that represents the object
func PersistentVolumeClaimToSelectableFields(pvClaim *api.PersistentVolumeClaim) fields.Set {
	specificFieldsSet := fields.Set{
		// This is a bug, but we need to support it for backward compatibility.
		"name": pvClaim.Name,
	}
	return generic.AddObjectMetaFieldsSet(specificFieldsSet, &pvClaim.ObjectMeta, false)
}
