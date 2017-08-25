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

// This file was automatically generated by lister-gen

package internalversion

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	authorization "k8s.io/kubernetes/pkg/apis/authorization"
)

// SelfSubjectRulesReviewLister helps list SelfSubjectRulesReviews.
type SelfSubjectRulesReviewLister interface {
	// List lists all SelfSubjectRulesReviews in the indexer.
	List(selector labels.Selector) (ret []*authorization.SelfSubjectRulesReview, err error)
	// SelfSubjectRulesReviews returns an object that can list and get SelfSubjectRulesReviews.
	SelfSubjectRulesReviews(namespace string) SelfSubjectRulesReviewNamespaceLister
	SelfSubjectRulesReviewListerExpansion
}

// selfSubjectRulesReviewLister implements the SelfSubjectRulesReviewLister interface.
type selfSubjectRulesReviewLister struct {
	indexer cache.Indexer
}

// NewSelfSubjectRulesReviewLister returns a new SelfSubjectRulesReviewLister.
func NewSelfSubjectRulesReviewLister(indexer cache.Indexer) SelfSubjectRulesReviewLister {
	return &selfSubjectRulesReviewLister{indexer: indexer}
}

// List lists all SelfSubjectRulesReviews in the indexer.
func (s *selfSubjectRulesReviewLister) List(selector labels.Selector) (ret []*authorization.SelfSubjectRulesReview, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*authorization.SelfSubjectRulesReview))
	})
	return ret, err
}

// SelfSubjectRulesReviews returns an object that can list and get SelfSubjectRulesReviews.
func (s *selfSubjectRulesReviewLister) SelfSubjectRulesReviews(namespace string) SelfSubjectRulesReviewNamespaceLister {
	return selfSubjectRulesReviewNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// SelfSubjectRulesReviewNamespaceLister helps list and get SelfSubjectRulesReviews.
type SelfSubjectRulesReviewNamespaceLister interface {
	// List lists all SelfSubjectRulesReviews in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*authorization.SelfSubjectRulesReview, err error)
	// Get retrieves the SelfSubjectRulesReview from the indexer for a given namespace and name.
	Get(name string) (*authorization.SelfSubjectRulesReview, error)
	SelfSubjectRulesReviewNamespaceListerExpansion
}

// selfSubjectRulesReviewNamespaceLister implements the SelfSubjectRulesReviewNamespaceLister
// interface.
type selfSubjectRulesReviewNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all SelfSubjectRulesReviews in the indexer for a given namespace.
func (s selfSubjectRulesReviewNamespaceLister) List(selector labels.Selector) (ret []*authorization.SelfSubjectRulesReview, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*authorization.SelfSubjectRulesReview))
	})
	return ret, err
}

// Get retrieves the SelfSubjectRulesReview from the indexer for a given namespace and name.
func (s selfSubjectRulesReviewNamespaceLister) Get(name string) (*authorization.SelfSubjectRulesReview, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(authorization.Resource("selfsubjectrulesreview"), name)
	}
	return obj.(*authorization.SelfSubjectRulesReview), nil
}
