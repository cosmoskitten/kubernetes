/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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

package events

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Event is a report of an event somewhere in the cluster. It generally denotes some state change in the system.
type EventTwo struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// Required. Time when this Event was first observed.
	EventTime metav1.MicroTime
	// Optional. Data about the Event series this event represents or nil if it's a singleton Event.
	Series *EventSeries
	// Required. Kubernetes component that generated the Event.
	Origin EventSource
	// Required. What Origin did/failed to do.
	Action EventAction
	// Optional. On what Origin acted upon.
	Object *ObjectReference
	// Optional secondary Object for more complex actions.
	SecondaryObject *ObjectReference
	// Required. Severity of the Event.
	Severity EventSeverity

	// Optional. A human-readable description of the status of this operation.
	// TODO: decide on maximum length.
	Message string
}

type EventSeries struct {
	// Unique identifier of the Event series.
	UID string
	// Number of Events in this series until last heartbeat.
	Count int32
	// Time when last Event from the series was seen before last heartbeat.
	LastObservedTime metav1.MicroTime
	// Last time when seried data was updated.
	LastHeartbeat metav1.MicroTime
	// Information whether this series is ongoing or finished.
	Status string
}

// Information about component that emits Event
type EventSource struct {
	// Component from which the event is generated.
	Component string
	// String that identifies the component emitting the Event
	ID string
}

// Information about action that was taken
type EventAction struct {
	Action string
	Reason string
}

type EventSeverity string

const (
	// e.g. Pod scheduled
	EventSeverityInfo EventSeverity = "Info"

	// e.g. Pod unable to schedule, cluster autoscaler adding Node
	EventSeverityWarning EventSeverity = "Warning"

	// e.g. Node unreachable - something is wrong, but there's a
	// possibility it will recover by itself
	EventSeverityError EventSeverity = "Error"

	// e.g. ClusterAutoscaler out of quota - something needs user
	// action
	EventSeverityFatal EventSeverity = "Fatal"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventList is a list of Event objects.
type EventTwoList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	Items []EventTwo
}
