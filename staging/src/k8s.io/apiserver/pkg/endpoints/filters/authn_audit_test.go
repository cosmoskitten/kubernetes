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

package filters

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/audit/policy"
	"k8s.io/apiserver/pkg/authentication/user"
	// import to call webhook's init() function to register audit.Event to schema
	_ "k8s.io/apiserver/plugin/pkg/audit/webhook"
)

func TestConstructFailedAuthnResponseWriter(t *testing.T) {
	afa := &auditFailedAuthenticationDecorator{}
	actual := afa.Decorate(&simpleResponseWriter{}, nil, nil)
	switch v := actual.(type) {
	case *auditFailedAuthnResponseWriter:
	default:
		t.Errorf("Expected auditFailedAuthnResponseWriter, got %v", reflect.TypeOf(v))
	}

	actual = afa.Decorate(&fancyResponseWriter{}, nil, nil)
	switch v := actual.(type) {
	case *fancyFailedAuthnResponseWriterDelegator:
	default:
		t.Errorf("Expected fancyFailedAuthnResponseWriterDelegator, got %v", reflect.TypeOf(v))
	}
}

func TestFailedAuthnAudit(t *testing.T) {
	sink := &fakeAuditSink{}
	policyChecker := policy.FakeChecker(auditinternal.LevelRequestResponse)
	afa := &auditFailedAuthenticationDecorator{
		requestContextMapper: &fakeRequestContextMapper{},
		sink:                 sink,
		policy:               policyChecker,
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = afa.Decorate(w, &user.DefaultInfo{Name: "username"}, r)
		w.WriteHeader(http.StatusUnauthorized)
	})
	req, _ := http.NewRequest("GET", "/api/v1/namespaces/default/pods", nil)
	req.RemoteAddr = "127.0.0.1"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if len(sink.events) != 1 {
		t.Fatalf("Unexpected number of audit events generated, expected 1, got: %d", len(sink.events))
	}
	ev := sink.events[0]
	if ev.User.Username != "username" {
		t.Errorf("Unexpected user, expected username, got %s", ev.User)
	}
	if ev.ResponseStatus.Code != http.StatusUnauthorized {
		t.Errorf("Unexpected response code, expected unauthorized, got %d", ev.ResponseStatus.Code)
	}
	if ev.Verb != "list" {
		t.Errorf("Unexpected verb, expected list, got %s", ev.Verb)
	}
	if ev.RequestURI != "/api/v1/namespaces/default/pods" {
		t.Errorf("Unexpected user, expected /api/v1/namespaces/default/pods, got %s", ev.RequestURI)
	}
}

func TestFailedAuthnAuditWithoutAuthorization(t *testing.T) {
	sink := &fakeAuditSink{}
	policyChecker := policy.FakeChecker(auditinternal.LevelRequestResponse)
	afa := &auditFailedAuthenticationDecorator{
		requestContextMapper: &fakeRequestContextMapper{},
		sink:                 sink,
		policy:               policyChecker,
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = afa.Decorate(w, nil, r)
		w.WriteHeader(http.StatusUnauthorized)
	})
	req, _ := http.NewRequest("GET", "/api/v1/namespaces/default/pods", nil)
	req.RemoteAddr = "127.0.0.1"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if len(sink.events) != 1 {
		t.Fatalf("Unexpected number of audit events generated, expected 1, got: %d", len(sink.events))
	}
	ev := sink.events[0]
	if ev.User.Username != "" {
		t.Errorf("Unexpected user, expected <empty>, got %s", ev.User)
	}
	if ev.ResponseStatus.Code != http.StatusUnauthorized {
		t.Errorf("Unexpected response code, expected unauthorized, got %d", ev.ResponseStatus.Code)
	}
	if ev.Verb != "list" {
		t.Errorf("Unexpected verb, expected list, got %s", ev.Verb)
	}
	if ev.RequestURI != "/api/v1/namespaces/default/pods" {
		t.Errorf("Unexpected user, expected /api/v1/namespaces/default/pods, got %s", ev.RequestURI)
	}
}
