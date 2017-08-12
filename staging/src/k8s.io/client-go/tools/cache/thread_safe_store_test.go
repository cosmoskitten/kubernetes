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

package cache

import (
	"errors"
	"strconv"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorFromIndex(t *testing.T) {
	t.Parallel()
	boomIndex := "boomIndex"
	namespaceIndex := "namespaceIndex"
	namespaceVal := "namespace1"
	n := 10 // Enough objects to trigger random map iteration order
	m := threadSafeMap{
		items: map[string]interface{}{},
		indexers: Indexers{
			boomIndex:      boomIndexFunc,
			namespaceIndex: MetaNamespaceIndexFunc,
		},
		indices: Indices{},
	}
	for i := 0; i < n; i++ {
		m.Add(strconv.Itoa(i), &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceVal,
			},
		})
	}

	indexItems, err := m.ByIndex(boomIndex, namespaceVal)
	require.NoError(t, err)
	assert.Empty(t, indexItems)

	indexItems, err = m.ByIndex(namespaceIndex, namespaceVal)
	require.NoError(t, err)
	assert.Len(t, indexItems, n)

	for i := 0; i < n; i++ {
		m.Delete(strconv.Itoa(i))
	}

	indexItems, err = m.ByIndex(boomIndex, namespaceVal)
	require.NoError(t, err)
	assert.Empty(t, indexItems)

	indexItems, err = m.ByIndex(namespaceIndex, namespaceVal)
	require.NoError(t, err)
	assert.Empty(t, indexItems)
}

func boomIndexFunc(obj interface{}) ([]string, error) {
	return nil, errors.New("boom")
}
