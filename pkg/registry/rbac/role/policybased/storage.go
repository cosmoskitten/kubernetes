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

// Package policybased implements a standard storage for Role that prevents privilege escalation.
package policybased

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacregistry "k8s.io/kubernetes/pkg/registry/rbac"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
)

var groupResource = rbac.Resource("roles")

// Registry is an interface for things that know how to store Roles.
type Registry interface {
	New() runtime.Object
	NewList() runtime.Object
	List(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (runtime.Object, error)
	Create(ctx genericapirequest.Context, obj runtime.Object, includeUninitialized bool) (runtime.Object, error)
	Update(ctx genericapirequest.Context, name string, obj rest.UpdatedObjectInfo) (runtime.Object, bool, error)
	Get(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
	Watch(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
	Export(ctx genericapirequest.Context, name string, opts metav1.ExportOptions) (runtime.Object, error)
	Delete(ctx genericapirequest.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error)
	DeleteCollection(ctx genericapirequest.Context, options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error)
}

type Storage struct {
	Registry

	ruleResolver rbacregistryvalidation.AuthorizationRuleResolver
}

func NewStorage(r Registry, ruleResolver rbacregistryvalidation.AuthorizationRuleResolver) *Storage {
	return &Storage{r, ruleResolver}
}

func (s *Storage) New() runtime.Object {
	return s.Registry.New()
}

func (s *Storage) NewList() runtime.Object {
	return s.Registry.NewList()
}

func (s *Storage) List(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return s.Registry.List(ctx, options)
}

func (s *Storage) Create(ctx genericapirequest.Context, obj runtime.Object, includeUninitialized bool) (runtime.Object, error) {
	if rbacregistry.EscalationAllowed(ctx) {
		return s.Registry.Create(ctx, obj, includeUninitialized)
	}

	role := obj.(*rbac.Role)
	rules := role.Rules
	if err := rbacregistryvalidation.ConfirmNoEscalation(ctx, s.ruleResolver, rules); err != nil {
		return nil, errors.NewForbidden(groupResource, role.Name, err)
	}
	return s.Registry.Create(ctx, obj, includeUninitialized)
}

func (s *Storage) Update(ctx genericapirequest.Context, name string, obj rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	if rbacregistry.EscalationAllowed(ctx) {
		return s.Registry.Update(ctx, name, obj)
	}

	nonEscalatingInfo := rest.WrapUpdatedObjectInfo(obj, func(ctx genericapirequest.Context, obj runtime.Object, oldObj runtime.Object) (runtime.Object, error) {
		role := obj.(*rbac.Role)

		rules := role.Rules
		if err := rbacregistryvalidation.ConfirmNoEscalation(ctx, s.ruleResolver, rules); err != nil {
			return nil, errors.NewForbidden(groupResource, role.Name, err)
		}
		return obj, nil
	})

	return s.Registry.Update(ctx, name, nonEscalatingInfo)
}

func (s *Storage) Get(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return s.Registry.Get(ctx, name, options)
}

func (s *Storage) Watch(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return s.Registry.Watch(ctx, options)
}

func (s *Storage) Export(ctx genericapirequest.Context, name string, opts metav1.ExportOptions) (runtime.Object, error) {
	return s.Registry.Export(ctx, name, opts)
}

func (s *Storage) Delete(ctx genericapirequest.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return s.Registry.Delete(ctx, name, options)
}

func (s *Storage) DeleteCollection(ctx genericapirequest.Context, options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	return s.Registry.DeleteCollection(ctx, options, listOptions)
}
