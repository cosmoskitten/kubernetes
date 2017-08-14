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

package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientBackedDryRunGetter implements the DryRunGetter interface for use in NewDryRunClient() and proxies all GET and LIST requests to the backing API server reachable via rest.Config
type ClientBackedDryRunGetter struct {
	baseConfig    *rest.Config
	dynClientPool dynamic.ClientPool
}

// NewClientBackedDryRunGetter creates a new ClientBackedDryRunGetter instance based on the rest.Config object
func NewClientBackedDryRunGetter(config *rest.Config) *ClientBackedDryRunGetter {
	return &ClientBackedDryRunGetter{
		baseConfig:    config,
		dynClientPool: dynamic.NewDynamicClientPool(config),
	}
}

// NewClientBackedDryRunGetter creates a new ClientBackedDryRunGetter instance from the given KubeConfig file
func NewClientBackedDryRunGetterFromKubeconfig(file string) (*ClientBackedDryRunGetter, error) {
	config, err := clientcmd.LoadFromFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client configuration from kubeconfig: %v", err)
	}
	return NewClientBackedDryRunGetter(clientConfig), nil
}

func (clg *ClientBackedDryRunGetter) HandleGetAction(action core.GetAction) (bool, runtime.Object, error) {
	rc, err := clg.ActionToResourceClient(action)
	if err != nil {
		return true, nil, err
	}

	obj, err := rc.Get(action.GetName(), metav1.GetOptions{})
	if obj == nil {
		return true, nil, nil
	}
	newObj, err := decodeObjIntoDefaultObj(action, obj)
	if err != nil {
		return true, nil, err
	}

	return true, newObj, err
}

func (clg *ClientBackedDryRunGetter) HandleListAction(action core.ListAction) (bool, runtime.Object, error) {
	rc, err := clg.ActionToResourceClient(action)
	if err != nil {
		return true, nil, err
	}

	listOpts := metav1.ListOptions{
		LabelSelector: action.GetListRestrictions().Labels.String(),
		FieldSelector: action.GetListRestrictions().Fields.String(),
	}

	objs, err := rc.List(listOpts)
	if objs == nil {
		return true, nil, nil
	}
	newObj, err := decodeObjIntoDefaultObj(action, objs)
	if err != nil {
		return true, nil, err
	}
	return true, newObj, err
}

func (clg *ClientBackedDryRunGetter) ActionToResourceClient(action core.Action) (dynamic.ResourceInterface, error) {
	dynIface, err := clg.dynClientPool.ClientForGroupVersionResource(action.GetResource())
	if err != nil {
		return nil, err
	}

	apiResource := &metav1.APIResource{
		Name:       action.GetResource().Resource,
		Namespaced: action.GetNamespace() != "",
	}

	return dynIface.Resource(apiResource, action.GetNamespace()), nil
}

func decodeObjIntoDefaultObj(action core.Action, obj runtime.Object) (runtime.Object, error) {
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(objBytes, []byte("null")) {
		return nil, nil
	}
	newObj, err := kuberuntime.Decode(clientsetscheme.Codecs.UniversalDecoder(action.GetResource().GroupVersion()), objBytes)
	if err != nil {
		return nil, err
	}
	return newObj, nil
}
