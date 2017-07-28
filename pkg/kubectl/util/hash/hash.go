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
	"crypto/sha256"
	"fmt"
	"sort"

	"k8s.io/kubernetes/pkg/api"
)

// ConfigMapHash returns a hash of the EncodeConfigMap encoding of `cm`
func ConfigMapHash(cm *api.ConfigMap) string {
	return encodeHash(hash(encodeConfigMap(cm)))
}

// SecretHash returns a hash of the EncodeSecret encoding of `sec`
func SecretHash(sec *api.Secret) string {
	return encodeHash(hash(encodeSecret(sec)))
}

// encodeConfigMap encodes a ConfigMap by encoding the Data with EncodeMapStringString and wrapping with {} braces
func encodeConfigMap(cm *api.ConfigMap) string {
	return fmt.Sprintf("{data:%s,}", encodeMapStringString(cm.Data))
}

// encodeSecret encodes a Secret by encoding the Data with EncodeMapStringByte and
// appending ",type:"+sec.Type+"," to the encoded string, and wraps with {} braces
func encodeSecret(sec *api.Secret) string {
	s := encodeMapStringBytes(sec.Data)
	return fmt.Sprintf("{data:%s,type:%s,}", s, string(sec.Type))
}

// encodeMapStringString extracts the key-value pairs from `m`, sorts them in byte-alphabetic order by key,
// and encodes them in a string representation. Keys and values are separated with `:` and pairs are separated
// with `,`. If m is non-empty, there is a trailing comma in the pre-hash serialization. If m is empty,
// there is no trailing comma. The entire encoding starts with "{" and ends with "}".
func encodeMapStringString(m map[string]string) string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	// sort based on keys
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	// encode to a string
	s := ""
	for _, k := range keys {
		s = s + k + ":" + m[k] + ","
	}
	return fmt.Sprintf("{%s}", s)
}

// encodeMapStringBytes converts `m` to map[string]string and returns the EncodeMapStringString encoding of this map
func encodeMapStringBytes(mb map[string][]byte) string {
	m := map[string]string{}
	for k, v := range mb {
		m[k] = string(v)
	}
	return encodeMapStringString(m)
}

// encodeHash keeps it simple and just extracts the first 40 bits of the hash from
// the hex string representation (1 hex char represents 4 bits)
func encodeHash(hex string) string {
	return hex[:10]
}

// hash hashes `data` with sha256
func hash(data string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}
