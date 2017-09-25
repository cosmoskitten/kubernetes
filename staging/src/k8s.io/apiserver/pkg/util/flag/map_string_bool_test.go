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

package flag

import (
	"reflect"
	"testing"
)

func TestStringMapStringBool(t *testing.T) {
	cases := []struct {
		desc   string
		m      MapStringBool
		expect string
	}{

		{"empty", MapStringBool{}, ""},
		{"one key", MapStringBool{"one": true}, "one=true"},
		{"two keys", MapStringBool{"one": true, "two": false}, "one=true,two=false"},
	}
	for _, c := range cases {
		str := c.m.String()
		if str != c.expect {
			t.Errorf("case %q, expected %q but got %q", c.desc, c.expect, str)
		}
	}
}

func TestSetMapStringBool(t *testing.T) {
	cases := []struct {
		desc   string
		val    string
		expect MapStringBool
		err    string
	}{
		{"empty", "", MapStringBool{}, ""},
		{"one key", "one=true", MapStringBool{"one": true}, ""},
		{"two keys", "one=true,two=false", MapStringBool{"one": true, "two": false}, ""},
		{"two keys with space", "one=true, two=false", MapStringBool{"one": true, "two": false}, ""},
		{"empty key", "=true", MapStringBool{"": true}, ""},
		{"missing value", "one", MapStringBool{}, "malformed pair, expect string=bool"},
		{"non-boolean value", "one=foo", MapStringBool{}, `invalid value of one: foo, err: strconv.ParseBool: parsing "foo": invalid syntax`},
	}
	for _, c := range cases {
		m := MapStringBool{}
		err := m.Set(c.val)
		if err != nil {
			if err.Error() != c.err {
				t.Errorf("case %q, unexpected error: %v", c.desc, err)
			}
			continue
		}
		if !reflect.DeepEqual(m, c.expect) {
			t.Errorf("case %q, expected %#v but got %#v", c.desc, c.expect, m)
		}
	}
}
