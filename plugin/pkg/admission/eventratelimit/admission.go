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
	"sync"

	"github.com/golang/groupcache/lru"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/kubernetes/pkg/api"
	eventratelimitapi "k8s.io/kubernetes/plugin/pkg/admission/eventratelimit/apis/eventratelimit"
	"k8s.io/kubernetes/plugin/pkg/admission/eventratelimit/apis/eventratelimit/validation"
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
			return newEventRateLimit(configuration)
		})
}

// eventRateLimitAdmission implements an admission controller that can enforce event rate limits
type eventRateLimitAdmission struct {
	sync.RWMutex
	*admission.Handler
	limitEnforcers []*limitEnforcer
}

// limitEnforcer enforces a single type of event rate limit, such as server, namespace, or source+object
type limitEnforcer struct {
	// type of this limit
	limitType string
	// factory for creating a rate limiter
	newRateLimiter func() flowcontrol.RateLimiter
	// cache for holding the rate limiters
	cache cache
	// function for selecting the cache key with which an event is associated
	keySelector func(admission.Attributes) interface{}
}

func newLimitEnforcer(config eventratelimitapi.Limit, rlf rateLimiterFactory) (*limitEnforcer, error) {
	limitType := config.Type
	newRateLimiter := func() flowcontrol.RateLimiter {
		return rlf(config.QPS, config.Burst)
	}
	limitEnforcer := &limitEnforcer{
		limitType:      limitType,
		newRateLimiter: newRateLimiter,
	}
	switch t := config.Type; t {
	case eventratelimitapi.ServerLimitType:
		limitEnforcer.cache = newSingleCache(newRateLimiter())
		limitEnforcer.keySelector = getServerKey
	case eventratelimitapi.NamespaceLimitType:
		limitEnforcer.cache = newLRUCache(config.CacheSize)
		limitEnforcer.keySelector = getNamespaceKey
	case eventratelimitapi.UserLimitType:
		limitEnforcer.cache = newLRUCache(config.CacheSize)
		limitEnforcer.keySelector = getUserKey
	case eventratelimitapi.SourceObjectLimitType:
		limitEnforcer.cache = newLRUCache(config.CacheSize)
		limitEnforcer.keySelector = getSourceObjectKey
	default:
		return nil, errors.New(fmt.Sprintf("unknown event rate limit type: %v", t))
	}
	return limitEnforcer, nil
}

func (enforcer *limitEnforcer) accept(attr admission.Attributes) error {
	key := enforcer.keySelector(attr)

	// do we have a record of similar events in our cache?
	rateLimiter, found := enforcer.cache.get(key)

	// verify we have a rate limiter for this record
	if !found {
		rateLimiter = enforcer.newRateLimiter()
	}

	// ensure we have available rate
	filter := rateLimiter.TryAccept()

	// update the cache
	enforcer.cache.add(key, rateLimiter)

	if !filter {
		return apierrors.NewTooManyRequestsError(fmt.Sprintf("limit reached on type %v for key %v", enforcer.limitType, key))
	}

	return nil
}

// cache is an interface for caching the limits of a particular type
type cache interface {
	// get the rate limiter associated with the specified key
	get(key interface{}) (rateLimiter flowcontrol.RateLimiter, found bool)
	// add the specified rate limiter to the cache, associated with the specified key
	add(key interface{}, rateLimiter flowcontrol.RateLimiter)
}

// singleCache is a cache that only stores a single, constant item
type singleCache struct {
	// the single rate limiter held by the cache
	rateLimiter flowcontrol.RateLimiter
}

func newSingleCache(rateLimiter flowcontrol.RateLimiter) *singleCache {
	return &singleCache{
		rateLimiter: rateLimiter,
	}
}

func (c *singleCache) get(key interface{}) (flowcontrol.RateLimiter, bool) {
	return c.rateLimiter, true
}

func (c *singleCache) add(key interface{}, rateLimiter flowcontrol.RateLimiter) {
	// Do nothing as the rate limiter stored by this cache is constant
}

// lruCache is a least-recently-used cache
type lruCache struct {
	cache *lru.Cache
}

func newLRUCache(cacheSize int) *lruCache {
	return &lruCache{
		cache: lru.New(cacheSize),
	}
}

func (c *lruCache) get(key interface{}) (flowcontrol.RateLimiter, bool) {
	value, found := c.cache.Get(key)
	if !found {
		return nil, false
	}
	return value.(flowcontrol.RateLimiter), true
}

func (c *lruCache) add(key interface{}, rateLimiter flowcontrol.RateLimiter) {
	c.cache.Add(key, rateLimiter)
}

// factory for creating a rate limiter
type rateLimiterFactory func(qps float32, burst int) flowcontrol.RateLimiter

// newEventRateLimit configures an admission controller that can enforce event rate limits
func newEventRateLimit(config *eventratelimitapi.Configuration) (admission.Interface, error) {
	rlf := func(qps float32, burst int) flowcontrol.RateLimiter {
		return flowcontrol.NewTokenBucketRateLimiter(qps, burst)
	}
	return newEventRateLimitUsingRLF(config, rlf)
}

// newEventRateLimitWithClock configures an admission controller that can enforce event rate limits.
// It uses a clock for testing purposes.
func newEventRateLimitWithClock(config *eventratelimitapi.Configuration, clock flowcontrol.Clock) (admission.Interface, error) {
	rlf := func(qps float32, burst int) flowcontrol.RateLimiter {
		return flowcontrol.NewTokenBucketRateLimiterWithClock(qps, burst, clock)
	}
	return newEventRateLimitUsingRLF(config, rlf)
}

// newEventRateLimit configures an admission controller than can enforce event rate limits
func newEventRateLimitUsingRLF(config *eventratelimitapi.Configuration, rlf rateLimiterFactory) (admission.Interface, error) {
	eventRateLimitAdmission := &eventRateLimitAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}

	for _, limitConfig := range config.Limits {
		enforcer, err := newLimitEnforcer(limitConfig, rlf)
		if err != nil {
			return nil, err
		}
		eventRateLimitAdmission.limitEnforcers = append(eventRateLimitAdmission.limitEnforcers, enforcer)
	}

	return eventRateLimitAdmission, nil
}

// Admit makes admission decisions while enforcing event rate limits
func (a *eventRateLimitAdmission) Admit(attr admission.Attributes) (err error) {
	// ignore all operations that do not correspond to an Event kind
	if attr.GetKind().GroupKind() != api.Kind("Event") {
		return nil
	}

	a.Lock()
	defer a.Unlock()

	// give each limit enforcer a chance to reject the event
	for _, enforcer := range a.limitEnforcers {
		if err := enforcer.accept(attr); err != nil {
			return err
		}
	}

	return nil
}

func getServerKey(attr admission.Attributes) interface{} {
	return nil
}

// getNamespaceKey returns a key for a parceledGate that is based on the namespace of the event request
func getNamespaceKey(attr admission.Attributes) interface{} {
	return attr.GetNamespace()
}

func getUserKey(attr admission.Attributes) interface{} {
	userInfo := attr.GetUserInfo()
	if userInfo == nil {
		return nil
	}
	return userInfo.GetName()
}

// getSourceObjecttKey returns a key for a parceledGate that is based on the source+object of the event
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
