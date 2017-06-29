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

package selfsubjectrulesreview

import (
	authorizationapi "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

type REST struct {
	ruleResolver authorizer.RuleResolver
}

func NewREST(ruleResolver authorizer.RuleResolver) *REST {
	return &REST{ruleResolver}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SelfSubjectRulesReview{}
}

func (r *REST) Create(ctx genericapirequest.Context, obj runtime.Object, includeUninitialized bool) (runtime.Object, error) {
	// the input object has no valuable input, so don't bother checking it.false
	user, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("no user present on request")
	}
	namespace, _ := genericapirequest.NamespaceFrom(ctx)
	resourceRules, nonResourceRules, err := r.ruleResolver.RulesFor(user, namespace)

	ret := &authorizationapi.SelfSubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			ResourceRules:    resourceRules,
			NonResourceRules: nonResourceRules,
		},
	}

	if err != nil {
		ret.Status.EvaluationError = err.Error()
	}

	return ret, nil
}
