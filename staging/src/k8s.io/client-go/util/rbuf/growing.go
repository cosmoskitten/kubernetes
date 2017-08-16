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

package rbuf

const (
	defaultBufferGrowStep = 4 * 1024
)

// Growing is a growing ring buffer.
// Not goroutine safe.
type Growing struct {
	data     []interface{}
	n        int // Size of Data
	beg      int // First available element
	readable int // Number of data items available
	growStep int // Size increment if buffer is full
}

// NewDefaultGrowing constructs a new Growing instance with default size and growth step.
func NewDefaultGrowing() *Growing {
	return NewGrowing(defaultBufferGrowStep, defaultBufferGrowStep)
}

// NewGrowing constructs a new Growing instance with provided parameters.
func NewGrowing(initialSize, growStep int) *Growing {
	return &Growing{
		data:     make([]interface{}, initialSize),
		n:        initialSize,
		growStep: growStep,
	}
}

// ReadOne reads (consumes) first item from the buffer if it is available, otherwise returns false.
func (r *Growing) ReadOne() (data interface{}, ok bool) {
	if r.readable == 0 {
		return nil, false
	}
	r.readable--
	element := r.data[r.beg]
	r.data[r.beg] = nil // Remove reference to the object to help GC
	if r.beg == r.n-1 {
		// Was the last element
		r.beg = 0
	} else {
		r.beg++
	}
	return element, true
}

// WriteOne adds an item to the end of the buffer, growing it if it is full.
func (r *Growing) WriteOne(data interface{}) {
	if r.readable == r.n {
		// Time to grow
		newN := r.n + r.growStep
		newData := make([]interface{}, newN)
		to := r.beg + r.readable
		if to <= r.n {
			copy(newData, r.data[r.beg:to])
		} else {
			copied := copy(newData, r.data[r.beg:])
			copy(newData[copied:], r.data[:(to%r.n)])
		}
		r.beg = 0
		r.data = newData
		r.n = newN
	}
	r.data[(r.readable+r.beg)%r.n] = data
	r.readable++
}
