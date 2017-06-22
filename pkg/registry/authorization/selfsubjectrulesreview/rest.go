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
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
)

type REST struct {
	ruleResolver rbacregistryvalidation.AuthorizationRuleResolver
}

func NewREST(ruleResolver rbacregistryvalidation.AuthorizationRuleResolver) *REST {
	return &REST{ruleResolver: ruleResolver}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SelfSubjectRulesReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx genericapirequest.Context, obj runtime.Object, includeUninitialized bool) (runtime.Object, error) {
	// the input object has no valuable input, so don't bother checking it.false
	user, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("no user present on request")
	}
	namespace, _ := genericapirequest.NamespaceFrom(ctx)
	policyRules, err := r.ruleResolver.RulesFor(user, namespace)

	resourceRules := []authorizationapi.ResourceRule{}
	nonResourceRules := []authorizationapi.NonResourceRule{}
	for _, rule := range policyRules {
		if len(rule.Resources) > 0 {
			rule := authorizationapi.ResourceRule{
				Verbs:         rule.Verbs,
				APIGroups:     rule.APIGroups,
				Resources:     rule.Resources,
				ResourceNames: rule.ResourceNames,
			}
			resourceRules = append(resourceRules, rule)
		}
		if len(rule.NonResourceURLs) > 0 {
			rule := authorizationapi.NonResourceRule{
				Verbs:           rule.Verbs,
				NonResourceURLs: rule.NonResourceURLs,
			}
			nonResourceRules = append(nonResourceRules, rule)
		}
	}

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
