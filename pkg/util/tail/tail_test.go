/*
Copyright 2016 The Kubernetes Authors.

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

package tail

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestReadAtMost(t *testing.T) {
	fakeFile, _ := ioutil.TempFile("", "")
	defer os.Remove(fakeFile.Name())
	fakeData := []byte("this is fake data")
	ioutil.WriteFile(fakeFile.Name(), fakeData, 0600)

	// Test read subset of file
	s, more_unread, err := ReadAtMost(fakeFile.Name(), 5)
	if err != nil {
		t.Error(err)
	}
	expected := fakeData[len(fakeData)-5:]
	if bytes.Compare(s, expected) != 0 {
		t.Error("%s != %s", s, expected)
	}
	if more_unread == false {
		t.Error("more_unread == false")
	}

	// Test read exactly file size
	s, more_unread, err = ReadAtMost(fakeFile.Name(), int64(len(fakeData)))
	if err != nil {
		t.Error(err)
	}
	expected = fakeData
	if bytes.Compare(s, expected) != 0 {
		t.Error("%s != %s", s, expected)
	}
	if more_unread == true {
		t.Error("more_unread == true")
	}

	// Test read past end of file
	s, more_unread, err = ReadAtMost(fakeFile.Name(), int64(len(fakeData) + 1))
	if err != nil {
		t.Error(err)
	}
	expected = fakeData
	if bytes.Compare(s, expected) != 0 {
		t.Error("%s != %s", s, expected)
	}
	if more_unread == true {
		t.Error("more_unread == true")
	}
}

func TestTail(t *testing.T) {
	line := strings.Repeat("a", blockSize)
	testBytes := []byte(line + "\n" +
		line + "\n" +
		line + "\n" +
		line + "\n" +
		line[blockSize/2:]) // incomplete line

	for c, test := range []struct {
		n     int64
		start int64
	}{
		{n: -1, start: 0},
		{n: 0, start: int64(len(line)+1) * 4},
		{n: 1, start: int64(len(line)+1) * 3},
		{n: 9999, start: 0},
	} {
		t.Logf("TestCase #%d: %+v", c, test)
		r := bytes.NewReader(testBytes)
		s, err := FindTailLineStartIndex(r, test.n)
		if err != nil {
			t.Error(err)
		}
		if s != test.start {
			t.Errorf("%d != %d", s, test.start)
		}
	}
}
