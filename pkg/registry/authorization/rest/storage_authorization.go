/*
Copyright 2016 The Kubernetes Authors.

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

package rest

import (
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"
	authorizationv1 "k8s.io/kubernetes/pkg/apis/authorization/v1"
	authorizationv1beta1 "k8s.io/kubernetes/pkg/apis/authorization/v1beta1"
	"k8s.io/kubernetes/pkg/registry/authorization/localsubjectaccessreview"
	"k8s.io/kubernetes/pkg/registry/authorization/selfsubjectaccessreview"
	"k8s.io/kubernetes/pkg/registry/authorization/selfsubjectrulesreview"
	"k8s.io/kubernetes/pkg/registry/authorization/subjectaccessreview"
	"k8s.io/kubernetes/pkg/registry/rbac/clusterrole"
	clusterrolestore "k8s.io/kubernetes/pkg/registry/rbac/clusterrole/storage"
	"k8s.io/kubernetes/pkg/registry/rbac/clusterrolebinding"
	clusterrolebindingstore "k8s.io/kubernetes/pkg/registry/rbac/clusterrolebinding/storage"
	"k8s.io/kubernetes/pkg/registry/rbac/role"
	rolestore "k8s.io/kubernetes/pkg/registry/rbac/role/storage"
	"k8s.io/kubernetes/pkg/registry/rbac/rolebinding"
	rolebindingstore "k8s.io/kubernetes/pkg/registry/rbac/rolebinding/storage"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
)

type RESTStorageProvider struct {
	Authorizer                authorizer.Authorizer
	AuthorizationRuleResolver rbacregistryvalidation.AuthorizationRuleResolver
}

func (p RESTStorageProvider) NewRESTStorage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (genericapiserver.APIGroupInfo, bool) {
	if p.Authorizer == nil {
		return genericapiserver.APIGroupInfo{}, false
	}

	if p.AuthorizationRuleResolver == nil {
		p.AuthorizationRuleResolver = rbacregistryvalidation.NewDefaultRuleResolver(
			role.AuthorizerAdapter{Registry: role.NewRegistry(rolestore.NewREST(restOptionsGetter))},
			rolebinding.AuthorizerAdapter{Registry: rolebinding.NewRegistry(rolebindingstore.NewREST(restOptionsGetter))},
			clusterrole.AuthorizerAdapter{Registry: clusterrole.NewRegistry(clusterrolestore.NewREST(restOptionsGetter))},
			clusterrolebinding.AuthorizerAdapter{Registry: clusterrolebinding.NewRegistry(clusterrolebindingstore.NewREST(restOptionsGetter))},
		)
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(authorization.GroupName, api.Registry, api.Scheme, api.ParameterCodec, api.Codecs)
	// If you add a version here, be sure to add an entry in `k8s.io/kubernetes/cmd/kube-apiserver/app/aggregator.go with specific priorities.
	// TODO refactor the plumbing to provide the information in the APIGroupInfo

	if apiResourceConfigSource.AnyResourcesForVersionEnabled(authorizationv1beta1.SchemeGroupVersion) {
		apiGroupInfo.VersionedResourcesStorageMap[authorizationv1beta1.SchemeGroupVersion.Version] = p.v1beta1Storage(apiResourceConfigSource, restOptionsGetter)
		apiGroupInfo.GroupMeta.GroupVersion = authorizationv1beta1.SchemeGroupVersion
	}

	if apiResourceConfigSource.AnyResourcesForVersionEnabled(authorizationv1.SchemeGroupVersion) {
		apiGroupInfo.VersionedResourcesStorageMap[authorizationv1.SchemeGroupVersion.Version] = p.v1Storage(apiResourceConfigSource, restOptionsGetter)
		apiGroupInfo.GroupMeta.GroupVersion = authorizationv1.SchemeGroupVersion
	}

	return apiGroupInfo, true
}

func (p RESTStorageProvider) v1beta1Storage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	version := authorizationv1beta1.SchemeGroupVersion

	storage := map[string]rest.Storage{}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("subjectaccessreviews")) {
		storage["subjectaccessreviews"] = subjectaccessreview.NewREST(p.Authorizer)
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("selfsubjectaccessreviews")) {
		storage["selfsubjectaccessreviews"] = selfsubjectaccessreview.NewREST(p.Authorizer)
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("localsubjectaccessreviews")) {
		storage["localsubjectaccessreviews"] = localsubjectaccessreview.NewREST(p.Authorizer)
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("selfsubjectrulesreviews")) {
		storage["selfsubjectrulesreviews"] = selfsubjectrulesreview.NewREST(p.AuthorizationRuleResolver)
	}

	return storage
}

func (p RESTStorageProvider) v1Storage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	version := authorizationv1beta1.SchemeGroupVersion

	storage := map[string]rest.Storage{}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("subjectaccessreviews")) {
		storage["subjectaccessreviews"] = subjectaccessreview.NewREST(p.Authorizer)
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("selfsubjectaccessreviews")) {
		storage["selfsubjectaccessreviews"] = selfsubjectaccessreview.NewREST(p.Authorizer)
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("localsubjectaccessreviews")) {
		storage["localsubjectaccessreviews"] = localsubjectaccessreview.NewREST(p.Authorizer)
	}
	if apiResourceConfigSource.ResourceEnabled(version.WithResource("selfsubjectrulesreviews")) {
		storage["selfsubjectrulesreviews"] = selfsubjectrulesreview.NewREST(p.AuthorizationRuleResolver)
	}

	return storage
}

func (p RESTStorageProvider) GroupName() string {
	return authorization.GroupName
}
