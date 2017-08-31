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

package testing

import (
	"path"
	"testing"

	"github.com/pborman/uuid"

	"sync"

	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

var (
	lock             sync.Mutex
	sharedEtcd       *etcdtesting.EtcdTestServer
	sharedEtcdConfig *storagebackend.Config
)

// SharedInProcessEtcd creates a storage config for a shared in-process etcd server.
// The first call creates the server. It's never terminated.
func SharedInProcessEtcd(t *testing.T) *storagebackend.Config {
	lock.Lock()
	defer lock.Unlock()

	if sharedEtcd == nil {
		sharedEtcd, sharedEtcdConfig = etcdtesting.NewUnsecuredEtcd3TestClientServer(t)
	}
	cfg := *sharedEtcdConfig
	cfg.Prefix = path.Join(cfg.Prefix, uuid.New(), "registry")
	return &cfg
}
