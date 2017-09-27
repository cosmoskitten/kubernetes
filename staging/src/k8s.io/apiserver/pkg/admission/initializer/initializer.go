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

package initializer

import (
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type pluginInitializer struct {
	externalClient    kubernetes.Interface
	externalInformers informers.SharedInformerFactory
	authorizer        authorizer.Authorizer
	// proxyClientCert used to provide identity when calling out to admission plugins
	proxyClientCert []byte
	// proxyClientKey private key for the client certificate used when calling out to admission plugins
	proxyClientKey []byte
}

// New creates an instance of admission plugins initializer.
// TODO(p0lyn0mial): make the parameters public, this construction seems to be redundant.
func New(extClientset kubernetes.Interface, extInformers informers.SharedInformerFactory, authz authorizer.Authorizer, proxyClientCert, proxyClientKey []byte) (pluginInitializer, error) {
	return pluginInitializer{
		externalClient:    extClientset,
		externalInformers: extInformers,
		authorizer:        authz,
		proxyClientCert:   proxyClientCert,
		proxyClientKey:    proxyClientKey,
	}, nil
}

// Initialize checks the initialization interfaces implemented by a plugin
// and provide the appropriate initialization data
func (i pluginInitializer) Initialize(plugin admission.Interface) {
	if wants, ok := plugin.(WantsExternalKubeClientSet); ok {
		wants.SetExternalKubeClientSet(i.externalClient)
	}

	if wants, ok := plugin.(WantsExternalKubeInformerFactory); ok {
		wants.SetExternalKubeInformerFactory(i.externalInformers)
	}

	if wants, ok := plugin.(WantsAuthorizer); ok {
		wants.SetAuthorizer(i.authorizer)
	}

	if wants, ok := plugin.(WantsClientCert); ok {
		if i.proxyClientCert == nil || i.proxyClientKey == nil {
			panic("An admission plugin wants a client cert/key, but they were not provided.")
		}
		wants.SetClientCert(i.proxyClientCert, i.proxyClientKey)
	}
}

var _ admission.PluginInitializer = pluginInitializer{}
