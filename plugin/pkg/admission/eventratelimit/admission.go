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

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang/groupcache/lru"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/kubernetes/pkg/api"
	eventratelimitapi "k8s.io/kubernetes/plugin/pkg/admission/eventratelimit/apis/eventratelimit"
	"k8s.io/kubernetes/plugin/pkg/admission/eventratelimit/apis/eventratelimit/validation"
)

const (
	defaultCacheSize = 4096
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register("EventRateLimit",
		func(config io.Reader) (admission.Interface, error) {
			// load the configuration provided (if any)
			configuration, err := LoadConfiguration(config)
			if err != nil {
				return nil, err
			}
			// validate the configuration (if any)
			if configuration != nil {
				if errs := validation.ValidateConfiguration(configuration); len(errs) != 0 {
					return nil, errs.ToAggregate()
				}
			}
			return newEventRateLimit(configuration, realClock{})
		})
}

// eventRateLimitAdmission implements an admission controller that can enforce event rate limits
type eventRateLimitAdmission struct {
	*admission.Handler
	// limitEnforcers is the collection of limit enforcers. There is one limit enforcer for each
	// active limit type. As there are 4 limit types, the length of the array will be at most 4.
	// The array is read-only after construction.
	limitEnforcers []*limitEnforcer
}

// realClock implements flowcontrol.Clock in terms of standard time functions.
type realClock struct{}

// Now is identical to time.Now.
func (realClock) Now() time.Time {
	return time.Now()
}

// Sleep is identical to time.Sleep.
func (realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// limitEnforcer enforces a single type of event rate limit, such as server, namespace, or source+object
type limitEnforcer struct {
	// type of this limit
	limitType eventratelimitapi.LimitType
	// cache for holding the rate limiters
	cache cache
	// function for selecting the cache key with which an event is associated
	keySelector func(admission.Attributes) interface{}
}

func newLimitEnforcer(config eventratelimitapi.Limit, clock flowcontrol.Clock) (*limitEnforcer, error) {
	rateLimiterFactory := func() flowcontrol.RateLimiter {
		return flowcontrol.NewTokenBucketRateLimiterWithClock(config.QPS, int(config.Burst), clock)
	}

	limitEnforcer := &limitEnforcer{
		limitType: config.Type,
	}

	if config.Type == eventratelimitapi.ServerLimitType {
		limitEnforcer.cache = &singleCache{
			rateLimiter: rateLimiterFactory(),
		}
	} else {
		cacheSize := int(config.CacheSize)
		if cacheSize == 0 {
			cacheSize = defaultCacheSize
		}
		limitEnforcer.cache = &lruCache{
			rateLimiterFactory: rateLimiterFactory,
			cache:              lru.New(cacheSize),
		}
	}

	switch t := config.Type; t {
	case eventratelimitapi.ServerLimitType:
		limitEnforcer.keySelector = getServerKey
	case eventratelimitapi.NamespaceLimitType:
		limitEnforcer.keySelector = getNamespaceKey
	case eventratelimitapi.UserLimitType:
		limitEnforcer.keySelector = getUserKey
	case eventratelimitapi.SourceObjectLimitType:
		limitEnforcer.keySelector = getSourceObjectKey
	default:
		return nil, errors.New(fmt.Sprintf("unknown event rate limit type: %v", t))
	}

	return limitEnforcer, nil
}

func (enforcer *limitEnforcer) accept(attr admission.Attributes) error {
	key := enforcer.keySelector(attr)
	rateLimiter := enforcer.cache.get(key)

	// ensure we have available rate
	filter := rateLimiter.TryAccept()

	if !filter {
		return apierrors.NewTooManyRequestsError(fmt.Sprintf("limit reached on type %v for key %v", enforcer.limitType, key))
	}

	return nil
}

// newEventRateLimit configures an admission controller that can enforce event rate limits
func newEventRateLimit(config *eventratelimitapi.Configuration, clock flowcontrol.Clock) (admission.Interface, error) {
	limitEnforcers := make([]*limitEnforcer, 0, len(config.Limits))
	for _, limitConfig := range config.Limits {
		enforcer, err := newLimitEnforcer(limitConfig, clock)
		if err != nil {
			return nil, err
		}
		limitEnforcers = append(limitEnforcers, enforcer)
	}

	eventRateLimitAdmission := &eventRateLimitAdmission{
		Handler:        admission.NewHandler(admission.Create, admission.Update),
		limitEnforcers: limitEnforcers,
	}

	return eventRateLimitAdmission, nil
}

// Admit makes admission decisions while enforcing event rate limits
func (a *eventRateLimitAdmission) Admit(attr admission.Attributes) (err error) {
	// ignore all operations that do not correspond to an Event kind
	if attr.GetKind().GroupKind() != api.Kind("Event") {
		return nil
	}

	var rejectionError error
	// give each limit enforcer a chance to reject the event
	for _, enforcer := range a.limitEnforcers {
		if err := enforcer.accept(attr); err != nil {
			rejectionError = err
		}
	}

	return rejectionError
}

func getServerKey(attr admission.Attributes) interface{} {
	return nil
}

// getNamespaceKey returns a cache key that is based on the namespace of the event request
func getNamespaceKey(attr admission.Attributes) interface{} {
	return attr.GetNamespace()
}

// getUserKey returns a cache key that is based on the user of the event request
func getUserKey(attr admission.Attributes) interface{} {
	userInfo := attr.GetUserInfo()
	if userInfo == nil {
		return nil
	}
	return userInfo.GetName()
}

// getSourceObjectKey returns a cache key that is based on the source+object of the event
func getSourceObjectKey(attr admission.Attributes) interface{} {
	object := attr.GetObject()
	if object == nil {
		return nil
	}
	event, ok := object.(*api.Event)
	if !ok {
		return nil
	}
	return strings.Join([]string{
		event.Source.Component,
		event.Source.Host,
		event.InvolvedObject.Kind,
		event.InvolvedObject.Namespace,
		event.InvolvedObject.Name,
		string(event.InvolvedObject.UID),
		event.InvolvedObject.APIVersion,
	}, "")
}
