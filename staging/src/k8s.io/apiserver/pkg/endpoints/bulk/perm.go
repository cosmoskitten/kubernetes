/*
Copyright 2014 The Kubernetes Authors.

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

package bulk

import (
	"errors"
	"time"

	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bulkapi "k8s.io/apiserver/pkg/apis/bulk"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
)

//
type permCheckFunc func() (err error)

func newForbiddenError(s bulkapi.ResourceSelector, reason string) error {
	return apierrors.NewForbidden(schema.GroupResource{s.Group, s.Resource}, s.Name, errors.New(reason))
}

func (m *APIManager) newAuthorizationCheckerForWatch(ctx request.Context, s bulkapi.ResourceSelector) permCheckFunc {
	glog.V(8).Infof("make permission checker for %v", s)

	var attribs authorizer.AttributesRecord
	if user, ok := request.UserFrom(ctx); ok {
		attribs.User = user
	}
	attribs.ResourceRequest = true
	attribs.Path = "" // FIXME: generate selfLink ?
	attribs.Verb = "watch"
	attribs.APIGroup = s.Group
	attribs.APIVersion = s.Version
	attribs.Resource = s.Resource
	attribs.Subresource = ""
	attribs.Namespace = s.Namespace
	attribs.Name = s.Name

	var lastCheckAt *time.Time
	var lastResult error

	return func() error {
		now := time.Now()
		if lastCheckAt == nil || lastCheckAt.Add(m.PermissionRecheck).Before(now) {
			lastCheckAt = &now
			auth, reason, err := m.Authorizer.Authorize(attribs)
			if err != nil {
				lastResult = err
			} else if auth {
				lastResult = nil
			} else {
				lastResult = newForbiddenError(s, reason)
			}
		}
		return lastResult
	}
}
