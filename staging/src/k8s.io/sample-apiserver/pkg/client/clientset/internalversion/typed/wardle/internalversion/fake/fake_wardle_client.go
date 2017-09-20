/*
Copyright 2017 The Kubernetes sample-apiserver Authors.

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

package fake

import (
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
	internalversion "k8s.io/sample-apiserver/pkg/client/clientset/internalversion/typed/wardle/internalversion"
)

type FakeWardle struct {
	*testing.Fake
}

func (c *FakeWardle) Fischers() internalversion.FischerInterface {
	return &FakeFischers{c}
}

func (c *FakeWardle) Flunders(namespace string) internalversion.FlunderInterface {
	return &FakeFlunders{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeWardle) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
