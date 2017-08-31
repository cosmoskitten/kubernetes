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
	"testing"

	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

// EtcdServer represents an etcd server, possibly shared among tests.
type EtcdServer interface {
	Release()
}

// EtcdProvider is a factory for EtcdServer.
type EtcdProvider func(t *testing.T) (EtcdServer, *storagebackend.Config)

type etcdTestingServer struct {
	t      *testing.T
	server *etcdtesting.EtcdTestServer
}

func (s *etcdTestingServer) Release() {
	s.server.Terminate(s.t)
}

// NewInProcessEtcd create an independent etcd server instance in-process.
func NewInProcessEtcd(t *testing.T) (EtcdServer, *storagebackend.Config) {
	s, cfg := etcdtesting.NewUnsecuredEtcd3TestClientServer(t)
	return &etcdTestingServer{t: t, server: s}, cfg
}
