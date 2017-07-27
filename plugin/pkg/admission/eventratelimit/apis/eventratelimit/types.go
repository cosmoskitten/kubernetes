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

package eventratelimit

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	ServerLimitType       = "server"
	NamespaceLimitType    = "namespace"
	UserLimitType         = "user"
	SourceObjectLimitType = "source+object"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration provides configuration for the EventRateLimit admission controller.
type Configuration struct {
	metav1.TypeMeta `json:",inline"`

	// Limits to place on events received.
	// Limits can be placed on events received server-wide, per namespace,
	// per user, and per source+object.
	// At least one limit is required.
	Limits []Limit `json:"limits"`
}

type Limit struct {
	// Type of limit.
	// The following are valid values.
	// "server": limits are maintained against all events received by the server
	// "namespace": limits are maintained against events from each namespace
	// "user": limits are maintained against events from each user
	// "source+object": limits are maintained against events from each source+object
	Type string `json:"type"`

	// Maximum QPS of events for this limit
	QPS float32 `json:"qps"`

	// Maximum burst for throttle of events for this limit
	Burst int ` json:"burst"`

	// Maximum number of limits to maintain. If room is needed in the cache for a
	// new limit, then the least-recently used limit is evicted, resetting the
	// stats for that subset of the universe.
	//
	// For example, if the type of limit is "namespace" and the limit for
	// namespace "A" is evicted, then the next event received from namespace "A"
	// will use reset stats, enabling events from namespace "A" as though no
	// events from namespace "A" have yet been received.
	//
	// If the type of limit is "server", then CacheSize is ignored and can be
	// omitted.
	CacheSize int `json:"cacheSize"`
}
