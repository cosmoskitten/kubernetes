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

// Type of limit (e.g., per-namespace)
type LimitType string

const (
	// ServerLimitType limits are maintained against all events received by the server
	ServerLimitType LimitType = "server"
	// NamespaceLimitType limits are maintained against events from each namespace
	NamespaceLimitType LimitType = "namespace"
	// UserLimitType limits are maintained against events from each user
	UserLimitType LimitType = "user"
	// SourceObjectLimitType limits are maintained against events from each source+object
	SourceObjectLimitType LimitType = "source+object"
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
	// Type of limit
	Type LimitType `json:"type"`

	// Maximum QPS of events for this limit
	QPS float32 `json:"qps"`

	// Maximum burst for throttle of events for this limit
	Burst int64 ` json:"burst"`

	// Size of the LRU cache for this limit. If a bucket is evicted from the cache,
	// then the stats for that bucket are reset. If more events are later received
	// for that bucket, then that bucket will re-enter the cache with a clean slate,
	// giving that bucket a full Burst number of tokens to use.
	//
	// The default cache size is 4096.
	//
	// If LimitType is ServerLimitType, then CacheSize is ignored.
	// +optional
	CacheSize int64 `json:"cacheSize,omitempty"`
}
