/*
Copyright 2015 The Kubernetes Authors.

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

package job

import (
	"math"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/util/integer"
)

type dynamicBackoffEntry struct {
	// backoff duration in second
	backoff time.Duration
	retry   int32

	lastUpdate time.Time
}

// DynamicBackoff used to
// TODO: Move DynamicBackoff in go-client/utils/controlflow package or merge it with
// controlflow.Backoff. Current difference is that we can for each entry different values
// of the backoff duration, and we store the number of retry for each entry
type DynamicBackoff struct {
	sync.Mutex
	Clock clock.Clock

	maxDuration    time.Duration
	perItemBackoff map[string]*dynamicBackoffEntry
}

// NewFakeDynamicBackOff return new fake DynamicBackOff instance
func NewFakeDynamicBackOff(max time.Duration, tc *clock.FakeClock) *DynamicBackoff {
	return &DynamicBackoff{
		perItemBackoff: map[string]*dynamicBackoffEntry{},
		Clock:          tc,
		maxDuration:    max,
	}
}

// NewDynamicBackOff return new DynamicBackOff instance
func NewDynamicBackOff(max time.Duration) *DynamicBackoff {
	return &DynamicBackoff{
		perItemBackoff: map[string]*dynamicBackoffEntry{},
		Clock:          clock.RealClock{},
		maxDuration:    max,
	}
}

// Get the current backoff Duration and the number of retry
func (p *DynamicBackoff) Get(id string) (time.Duration, int32) {
	p.Lock()
	defer p.Unlock()
	var delay time.Duration
	var nbRetry int32
	entry, ok := p.perItemBackoff[id]
	if ok {
		delay = entry.backoff
		nbRetry = entry.retry
	}
	return delay, nbRetry
}

// Next move backoff to the next mark and increment the number of retry, capping at maxDuration
func (p *DynamicBackoff) Next(id string, backoffDuration time.Duration, eventTime time.Time) {
	p.Lock()
	defer p.Unlock()
	entry, ok := p.perItemBackoff[id]
	if !ok || hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		entry = p.initEntryUnsafe(id, backoffDuration)
	} else {
		delay := entry.backoff * 2 // exponential
		entry.retry++
		entry.backoff = time.Duration(integer.Int64Min(int64(delay), int64(p.maxDuration)))
	}
	entry.lastUpdate = p.Clock.Now()
}

// Reset forces clearing of all backoff data for a given key.
func (p *DynamicBackoff) Reset(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.perItemBackoff, id)
}

// IsInBackOffSince returns True if the elapsed time since eventTime is smaller than the current backoff window
func (p *DynamicBackoff) IsInBackOffSince(id string, eventTime time.Time) bool {
	p.Lock()
	defer p.Unlock()
	entry, ok := p.perItemBackoff[id]
	if !ok {
		return false
	}
	if hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		return false
	}
	return p.Clock.Now().Sub(eventTime) < entry.backoff
}

// IsInBackOffSinceUpdate returns True if time since lastupdate is less than the current backoff window.
func (p *DynamicBackoff) IsInBackOffSinceUpdate(id string, eventTime time.Time) bool {
	p.Lock()
	defer p.Unlock()
	entry, ok := p.perItemBackoff[id]
	if !ok {
		return false
	}
	if hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		return false
	}
	return eventTime.Sub(entry.lastUpdate) < entry.backoff
}

// GC garbage collect records that have aged past maxDuration. Backoff users are expected
// to invoke this periodically.
func (p *DynamicBackoff) GC() {
	p.Lock()
	defer p.Unlock()
	now := p.Clock.Now()
	for id, entry := range p.perItemBackoff {
		if now.Sub(entry.lastUpdate) > p.maxDuration*2 {
			// GC when entry has not been updated for 2*maxDuration
			delete(p.perItemBackoff, id)
		}
	}
}

// DeleteEntry deletes the entry
func (p *DynamicBackoff) DeleteEntry(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.perItemBackoff, id)
}

// Take a lock on *Backoff, before calling initEntryUnsafe
func (p *DynamicBackoff) initEntryUnsafe(id string, backoff time.Duration) *dynamicBackoffEntry {
	entry := &dynamicBackoffEntry{
		backoff: backoff,
	}
	p.perItemBackoff[id] = entry
	return entry
}

// StartDynamicBackoffGC used to start the Backoff garbage collection mechanism
// DynamicBackoff.GC() will be executed each minute
func StartDynamicBackoffGC(backoff *DynamicBackoff, stopCh <-chan struct{}) {
	go func() {
		for {
			select {
			case <-time.After(time.Minute):
				backoff.GC()
			case <-stopCh:
				return
			}
		}
	}()
}

func computeDelay(nbRetry int32, backoff int64) time.Duration {
	return time.Duration(float64(backoff)*math.Pow(2, float64(nbRetry))) * time.Second
}

// After 2*maxDuration we restart the backoff factor to the beginning
func hasExpired(eventTime time.Time, lastUpdate time.Time, maxDuration time.Duration) bool {
	return eventTime.Sub(lastUpdate) > maxDuration*2 // consider stable if it's ok for twice the maxDuration
}
