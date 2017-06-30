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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/rule"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
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
	resourceInfo, nonResourceInfo, err := r.ruleResolver.RulesFor(user, namespace)

	ret := &authorizationapi.SelfSubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			ResourceRules:    getResourceRules(resourceInfo),
			NonResourceRules: getNonResourceRules(nonResourceInfo),
		},
	}

	if err != nil {
		ret.Status.EvaluationError = err.Error()
	}

	return ret, nil
}

func getResourceRules(infos []rule.ResourceInfo) []authorizationapi.ResourceRule {
	rules := []authorizationapi.ResourceRule{}
	for _, info := range infos {
		rule := authorizationapi.ResourceRule{
			Verbs:         info.GetVerbs(),
			APIGroups:     info.GetAPIGroups(),
			Resources:     info.GetResources(),
			ResourceNames: info.GetResourceNames(),
		}
		rules = append(rules, rule)
	}
	return rules
}

func getNonResourceRules(infos []rule.NonResourceInfo) []authorizationapi.NonResourceRule {
	rules := []authorizationapi.NonResourceRule{}
	for _, info := range infos {
		rule := authorizationapi.NonResourceRule{
			Verbs:           info.GetVerbs(),
			NonResourceURLs: info.GetNonResourceURLs(),
		}
		rules = append(rules, rule)
	}
	return rules
}
