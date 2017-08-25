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

package hash

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
)

func TestConfigMapHash(t *testing.T) {
	cases := []struct {
		desc string
		cm   *api.ConfigMap
		hash string
	}{
		// empty map
		{"empty data", &api.ConfigMap{Data: map[string]string{}}, "d4k42785cm"},
		// one key
		{"one key", &api.ConfigMap{Data: map[string]string{"one": ""}}, "8f6fdm7ftg"},
		// three keys (tests sorting order)
		{"three keys", &api.ConfigMap{Data: map[string]string{"two": "2", "one": "", "three": "3"}}, "c4kg7t626d"},
	}

	for _, c := range cases {
		h := ConfigMapHash(c.cm)
		if c.hash != h {
			t.Errorf("case %q, expect hash %q but got %q", c.desc, c.hash, h)
		}
	}
}

func TestSecretHash(t *testing.T) {
	cases := []struct {
		desc   string
		secret *api.Secret
		hash   string
	}{
		// empty map
		{"empty data", &api.Secret{Type: "my-type", Data: map[string][]byte{}}, "2hffk95cm2"},
		// one key
		{"one key", &api.Secret{Type: "my-type", Data: map[string][]byte{"one": []byte("")}}, "h9b9479tkh"},
		// three keys (tests sorting order)
		{"three keys", &api.Secret{Type: "my-type", Data: map[string][]byte{"two": []byte("2"), "one": []byte(""), "three": []byte("3")}}, "tgf9tk729k"},
	}

	for _, c := range cases {
		h := SecretHash(c.secret)
		if c.hash != h {
			t.Errorf("case %q, expect hash %q but got %q", c.desc, c.hash, h)
		}
	}
}

func TestEncodeConfigMap(t *testing.T) {
	cases := []struct {
		desc   string
		cm     *api.ConfigMap
		expect string
	}{
		// empty map
		{"empty data", &api.ConfigMap{Data: map[string]string{}}, "{data:{},}"},
		// one key
		{"one key", &api.ConfigMap{Data: map[string]string{"one": ""}}, "{data:{one:,},}"},
		// three keys (tests sorting order)
		{"three keys", &api.ConfigMap{Data: map[string]string{"two": "2", "one": "", "three": "3"}}, "{data:{one:,three:3,two:2,},}"},
	}
	for _, c := range cases {
		s := encodeConfigMap(c.cm)
		if s != c.expect {
			t.Errorf("case %q, expect %q from encode %#v, but got %q", c.desc, c.expect, c.cm, s)
		}
	}
}

func TestEncodeSecret(t *testing.T) {
	cases := []struct {
		desc   string
		secret *api.Secret
		expect string
	}{
		// empty map
		{"empty data", &api.Secret{Type: "my-type", Data: map[string][]byte{}}, "{data:{},type:my-type,}"},
		// one key
		{"one key", &api.Secret{Type: "my-type", Data: map[string][]byte{"one": []byte("")}}, "{data:{one:,},type:my-type,}"},
		// three keys (tests sorting order)
		{"three keys", &api.Secret{Type: "my-type", Data: map[string][]byte{"two": []byte("2"), "one": []byte(""), "three": []byte("3")}}, "{data:{one:,three:3,two:2,},type:my-type,}"},
	}
	for _, c := range cases {
		s := encodeSecret(c.secret)
		if s != c.expect {
			t.Errorf("case %q, expect %q from encode %#v, but got %q", c.desc, c.expect, c.secret, s)
		}
	}
}

func TestEncodeMapStringString(t *testing.T) {
	cases := []struct {
		desc   string
		m      map[string]string
		expect string
	}{
		// empty map
		{"empty map", map[string]string{}, "{}"},
		// one key
		{"one key", map[string]string{"one": ""}, "{one:,}"},
		// three keys (tests sorting order)
		{"three keys", map[string]string{"two": "2", "one": "", "three": "3"}, "{one:,three:3,two:2,}"},
	}
	for _, c := range cases {
		s := encodeMapStringString(c.m)
		if s != c.expect {
			t.Errorf("case %q, expect %q from encode %#v, but got %q", c.desc, c.expect, c.m, s)
		}
	}
}

func TestEncodeMapStringBytes(t *testing.T) {
	cases := []struct {
		desc   string
		m      map[string][]byte
		expect string
	}{
		// empty map
		{"empty map", map[string][]byte{}, "{}"},
		// one key
		{"one key", map[string][]byte{"one": []byte("")}, "{one:,}"},
		// three keys (tests sorting order)
		{"three keys", map[string][]byte{"two": []byte("2"), "one": []byte(""), "three": []byte("3")}, "{one:,three:3,two:2,}"},
	}
	for _, c := range cases {
		s := encodeMapStringBytes(c.m)
		if s != c.expect {
			t.Errorf("case %q, expect %q from encode %#v, but got %q", c.desc, c.expect, c.m, s)
		}
	}
}

func TestHash(t *testing.T) {
	// hash the empty string to be sure that sha256 is being used
	expect := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	sum := hash("")
	if expect != sum {
		t.Errorf("expected hash %q but got %q", expect, sum)
	}
}

// warn devs who change types that they might have to update a hash function
// not perfect, as it only checks the number of top-level fields
func TestTypeStability(t *testing.T) {
	errfmt := `case %q, expected %d fields but got %d
Depending on the field you added, you may need to modify the hash function for this type.
To guide you: the hash function targets fields that comprise the contents of objects,
not their metadata (e.g. the Data of a ConfigMap, but nothing in ObjectMeta).
`
	cases := []struct {
		typeName string
		obj      interface{}
		expect   int
	}{
		{"ConfigMap", api.ConfigMap{}, 3},
		{"Secret", api.Secret{}, 4},
	}
	for _, c := range cases {
		val := reflect.ValueOf(c.obj)
		if num := val.NumField(); c.expect != num {
			t.Errorf(errfmt, c.typeName, c.expect, num)
		}
	}
}
