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

import "testing"

func TestGrowth(t *testing.T) {
	t.Parallel()
	X := 10
	g := NewGrowing(1, 2)
	for i := 0; i < X; i++ {
		if g.Readable() != i {
			t.Errorf("readable %d != %d", g.Readable(), i)
		}
		g.WriteOne(i)
	}
	read := 0
	for g.Readable() > 0 {
		v := g.ReadOne().(int)
		if read != v {
			t.Errorf("%d != %d", read, v)
		}
		read++
	}
	if read != X {
		t.Errorf("expected to have read %d items: %d", X, read)
	}
	if g.readable != 0 {
		t.Errorf("expected readable to be zero: %d", g.readable)
	}
	if g.n != 11 {
		t.Errorf("expected N to be 11: %d", g.n)
	}
}

func TestPanic(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if err, ok := r.(error); !ok || err.Error() != "no elements available in the buffer" {
			t.Errorf("expected error, got: %v", err)
		}
	}()
	g := NewGrowing(1, 2)
	g.ReadOne()
}
