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

package kubelet

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

type requestDecoratingRESTClient struct {
	client    rest.Interface
	decorator func(*rest.Request) *rest.Request
}

func (r *requestDecoratingRESTClient) GetRateLimiter() flowcontrol.RateLimiter {
	return r.client.GetRateLimiter()
}
func (r *requestDecoratingRESTClient) Verb(verb string) *rest.Request {
	return r.decorator(r.client.Verb(verb))
}
func (r *requestDecoratingRESTClient) Post() *rest.Request {
	return r.decorator(r.client.Post())
}
func (r *requestDecoratingRESTClient) Put() *rest.Request {
	return r.decorator(r.client.Put())
}
func (r *requestDecoratingRESTClient) Patch(pt types.PatchType) *rest.Request {
	return r.decorator(r.client.Patch(pt))
}
func (r *requestDecoratingRESTClient) Get() *rest.Request {
	return r.decorator(r.client.Get())
}
func (r *requestDecoratingRESTClient) Delete() *rest.Request {
	return r.decorator(r.client.Delete())
}
func (r *requestDecoratingRESTClient) APIVersion() schema.GroupVersion {
	return r.client.APIVersion()
}
