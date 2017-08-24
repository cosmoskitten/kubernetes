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
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"
)

func TestSlowDynamicBackoff(t *testing.T) {
	id := "_idSlow"
	tc := clock.NewFakeClock(time.Now())
	step := time.Second
	maxDuration := 50 * step

	b := NewFakeDynamicBackOff(maxDuration, tc)
	cases := []struct {
		//expected
		expectedRetry int32
		expectedDelay time.Duration
	}{
		{
			0,
			time.Duration(0) * time.Second,
		},
		{
			0,
			time.Duration(1) * time.Second,
		},
		{
			1,
			time.Duration(2) * time.Second,
		},
		{
			2,
			time.Duration(4) * time.Second,
		},
		{
			3,
			time.Duration(8) * time.Second,
		},
		{
			4,
			time.Duration(16) * time.Second,
		},
		{
			5,
			time.Duration(32) * time.Second,
		},
		{
			6,
			time.Duration(50) * time.Second,
		},
		{
			7,
			time.Duration(50) * time.Second,
		},
		{
			8,
			time.Duration(50) * time.Second,
		},
	}
	for ix, c := range cases {
		tc.Step(step)
		w, retry := b.Get(id)
		if retry != c.expectedRetry {
			t.Errorf("input: '%d': retry expected %d, got %d", ix, ix, retry)
		}
		if w != c.expectedDelay {
			t.Errorf("input: '%d': expected %s, got %s", ix, c.expectedDelay, w)
		}
		b.Next(id, step, tc.Now())
	}

	//Now confirm that the Reset cancels backoff.
	b.Next(id, step, tc.Now())
	b.Reset(id)
	backoff, counter := b.Get(id)
	if backoff != 0 || counter != 0 {
		t.Errorf("Reset didn't clear the backoff.")
	}

}

func TestBackoffReset(t *testing.T) {
	id := "_idReset"
	tc := clock.NewFakeClock(time.Now())
	step := time.Second
	maxDuration := step * 5
	b := NewFakeDynamicBackOff(maxDuration, tc)
	startTime := tc.Now()

	// get to backoff = maxDuration
	for i := 0; i <= int(maxDuration/step); i++ {
		tc.Step(step)
		b.Next(id, step, tc.Now())
	}

	// backoff should be capped at maxDuration
	if !b.IsInBackOffSince(id, tc.Now()) {
		delay, _ := b.Get(id)
		t.Errorf("expected to be in Backoff got %s", delay)
	}

	lastUpdate := tc.Now()
	tc.Step(2*maxDuration + step) // time += 11s, 11 > 2*maxDuration
	if b.IsInBackOffSince(id, lastUpdate) {
		delay, _ := b.Get(id)
		t.Errorf("expected to not be in Backoff after reset (start=%s, now=%s, lastUpdate=%s), got %s", startTime, tc.Now(), lastUpdate, delay)
	}
}

func TestBackoffHightWaterMark(t *testing.T) {
	id := "_idHiWaterMark"
	tc := clock.NewFakeClock(time.Now())
	step := time.Second
	maxDuration := 5 * step
	b := NewFakeDynamicBackOff(maxDuration, tc)

	// get to backoff = maxDuration
	for i := 0; i <= int(maxDuration/step); i++ {
		tc.Step(step)
		b.Next(id, step, tc.Now())
	}

	// backoff high watermark expires after 2*maxDuration
	tc.Step(maxDuration + step)
	b.Next(id, step, tc.Now())

	delay, _ := b.Get(id)
	if delay != maxDuration {
		t.Errorf("expected Backoff to stay at high watermark %s got %s", maxDuration, delay)
	}
}

func TestBackoffGC(t *testing.T) {
	id := "_idGC"
	tc := clock.NewFakeClock(time.Now())
	step := time.Second
	maxDuration := 5 * step

	b := NewFakeDynamicBackOff(maxDuration, tc)

	for i := 0; i <= int(maxDuration/step); i++ {
		tc.Step(step)
		b.Next(id, step, tc.Now())
	}
	lastUpdate := tc.Now()
	tc.Step(maxDuration + step)
	b.GC()
	_, found := b.perItemBackoff[id]
	if !found {
		t.Errorf("expected GC to skip entry, elapsed time=%s maxDuration=%s", tc.Now().Sub(lastUpdate), maxDuration)
	}

	tc.Step(maxDuration + step)
	b.GC()
	r, found := b.perItemBackoff[id]
	if found {
		t.Errorf("expected GC of entry after %s got entry %v", tc.Now().Sub(lastUpdate), r)
	}
}

func TestIsInBackOffSinceUpdate(t *testing.T) {
	id := "_idIsInBackOffSinceUpdate"
	tc := clock.NewFakeClock(time.Now())
	step := time.Second
	maxDuration := 10 * step
	b := NewFakeDynamicBackOff(maxDuration, tc)
	startTime := tc.Now()

	cases := []struct {
		tick      time.Duration
		inBackOff bool
		value     int
	}{
		{tick: 0, inBackOff: false, value: 0},
		{tick: 1, inBackOff: false, value: 1},
		{tick: 2, inBackOff: true, value: 2},
		{tick: 3, inBackOff: false, value: 2},
		{tick: 4, inBackOff: true, value: 4},
		{tick: 5, inBackOff: true, value: 4},
		{tick: 6, inBackOff: true, value: 4},
		{tick: 7, inBackOff: false, value: 4},
		{tick: 8, inBackOff: true, value: 8},
		{tick: 9, inBackOff: true, value: 8},
		{tick: 10, inBackOff: true, value: 8},
		{tick: 11, inBackOff: true, value: 8},
		{tick: 12, inBackOff: true, value: 8},
		{tick: 13, inBackOff: true, value: 8},
		{tick: 14, inBackOff: true, value: 8},
		{tick: 15, inBackOff: false, value: 8},
		{tick: 16, inBackOff: true, value: 10},
		{tick: 17, inBackOff: true, value: 10},
		{tick: 18, inBackOff: true, value: 10},
		{tick: 19, inBackOff: true, value: 10},
		{tick: 20, inBackOff: true, value: 10},
		{tick: 21, inBackOff: true, value: 10},
		{tick: 22, inBackOff: true, value: 10},
		{tick: 23, inBackOff: true, value: 10},
		{tick: 24, inBackOff: true, value: 10},
		{tick: 25, inBackOff: false, value: 10},
		{tick: 26, inBackOff: true, value: 10},
		{tick: 27, inBackOff: true, value: 10},
		{tick: 28, inBackOff: true, value: 10},
		{tick: 29, inBackOff: true, value: 10},
		{tick: 30, inBackOff: true, value: 10},
		{tick: 31, inBackOff: true, value: 10},
		{tick: 32, inBackOff: true, value: 10},
		{tick: 33, inBackOff: true, value: 10},
		{tick: 34, inBackOff: true, value: 10},
		{tick: 35, inBackOff: false, value: 10},
		{tick: 56, inBackOff: false, value: 0},
		{tick: 57, inBackOff: false, value: 1},
	}

	for _, c := range cases {
		tc.SetTime(startTime.Add(c.tick * step))
		if c.inBackOff != b.IsInBackOffSinceUpdate(id, tc.Now()) {
			t.Errorf("expected IsInBackOffSinceUpdate %v got %v at tick %s", c.inBackOff, b.IsInBackOffSinceUpdate(id, tc.Now()), c.tick*step)
		}
		delay, _ := b.Get(id)
		if c.inBackOff && (time.Duration(c.value)*step != delay) {
			t.Errorf("expected backoff value=%s got %s at tick %s", time.Duration(c.value)*step, delay, c.tick*step)
		}

		if !c.inBackOff {
			b.Next(id, step, tc.Now())
		}
	}
}

func Test_computeDelay(t *testing.T) {
	type args struct {
		nbRetry         int32
		backoffDuration int64
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			"0 retry",
			args{0, 10},
			time.Duration(10 * time.Second),
		},
		{
			"1 retry",
			args{1, 10},
			time.Duration(20 * time.Second),
		},
		{
			"2 retry",
			args{2, 10},
			time.Duration(40 * time.Second),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeDelay(tt.args.nbRetry, tt.args.backoffDuration); got != tt.want {
				t.Errorf("computeDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}
