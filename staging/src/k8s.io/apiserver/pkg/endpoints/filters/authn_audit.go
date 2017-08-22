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
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/apiserver/pkg/audit/policy"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// WithFailedAuthenticationAudit decorates a http.Handler with a fallback audit,
// logging information only when the original one did was not triggered.
// This needs to be used with WithAuditTriggeredMarker, which wraps the original
// audit filter.
func WithFailedAuthenticationAudit(handler http.Handler, requestContextMapper request.RequestContextMapper, sink audit.Sink, policy policy.Checker) http.Handler {
	if sink == nil || policy == nil {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rw := decorateFailedAuthnResponseWriter(w, req, requestContextMapper, sink, policy)
		handler.ServeHTTP(rw, req)
	})
}

func decorateFailedAuthnResponseWriter(rw http.ResponseWriter, req *http.Request, requestContextMapper request.RequestContextMapper, sink audit.Sink, policy policy.Checker) http.ResponseWriter {
	delegate := &auditFailedAuthnResponseWriter{
		ResponseWriter:       rw,
		request:              req,
		requestContextMapper: requestContextMapper,
		sink:                 sink,
		policy:               policy,
	}
	// check if the ResponseWriter we're wrapping is the fancy one we need
	// or if the basic is sufficient
	_, cn := rw.(http.CloseNotifier)
	_, fl := rw.(http.Flusher)
	_, hj := rw.(http.Hijacker)
	if cn && fl && hj {
		return &fancyFailedAuthnResponseWriterDelegator{delegate}
	}
	return delegate
}

var _ http.ResponseWriter = &auditFailedAuthnResponseWriter{}

// auditFailedAuthnResponseWriter intercepts WriteHeader, sets it in the event.
type auditFailedAuthnResponseWriter struct {
	http.ResponseWriter
	event                *auditinternal.Event
	once                 sync.Once
	user                 user.Info
	request              *http.Request
	requestContextMapper request.RequestContextMapper
	sink                 audit.Sink
	policy               policy.Checker
}

func (a *auditFailedAuthnResponseWriter) setHttpHeader() {
	if a.event != nil {
		a.ResponseWriter.Header().Set(auditinternal.HeaderAuditID, string(a.event.AuditID))
	}
}

func (a *auditFailedAuthnResponseWriter) Write(bs []byte) (int, error) {
	// the Go library calls WriteHeader internally if no code was written yet. But this will go unnoticed for us
	a.processCode(http.StatusOK)
	a.setHttpHeader()
	return a.ResponseWriter.Write(bs)
}

func (a *auditFailedAuthnResponseWriter) WriteHeader(code int) {
	a.processCode(code)
	a.setHttpHeader()
	a.ResponseWriter.WriteHeader(code)
}

func (a *auditFailedAuthnResponseWriter) processCode(code int) {
	a.once.Do(func() {
		ctx, ok := a.requestContextMapper.Get(a.request)
		if !ok {
			responsewriters.InternalError(a.ResponseWriter, a.request, errors.New("no context found for request"))
			return
		}

		// we need to create the event and log the authn error
		ev, err := createAuditEvent(ctx, a.policy, a.requestContextMapper, a.request, a.ResponseWriter)
		if err != nil {
			return
		}
		if ev == nil {
			return
		}

		ctx = request.WithAuditEvent(ctx, ev)
		if err := a.requestContextMapper.Update(a.request, ctx); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to attach audit event to the context: %v", err))
			responsewriters.InternalError(a.ResponseWriter, a.request, errors.New("failed to update context"))
			return
		}

		ev.ResponseStatus = &metav1.Status{}
		ev.ResponseStatus.Code = int32(code)
		ev.ResponseStatus.Message = getAuthMethods(a.request)
		ev.Stage = auditinternal.StageResponseStarted
		processAuditEvent(a.sink, ev)
		a.event = ev
	})
}

func getAuthMethods(req *http.Request) string {
	authMethods := []string{}

	if _, _, ok := req.BasicAuth(); ok {
		authMethods = append(authMethods, "basic")
	}

	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	parts := strings.Split(auth, " ")
	if len(parts) > 1 && strings.ToLower(parts[0]) == "bearer" {
		authMethods = append(authMethods, "bearer")
	}

	token := strings.TrimSpace(req.URL.Query().Get("access_token"))
	if len(token) > 0 {
		authMethods = append(authMethods, "access_token")
	}

	if req.TLS != nil && len(req.TLS.PeerCertificates) > 0 {
		authMethods = append(authMethods, "x509")
	}

	if len(authMethods) > 0 {
		return fmt.Sprintf("Authentication failed, attempted: %s", strings.Join(authMethods, ", "))
	}
	return "Authentication failed, no credentials provided"
}

// fancyFailedAuthnResponseWriterDelegator implements http.CloseNotifier, http.Flusher and
// http.Hijacker which are needed to make certain http operation (e.g. watch, rsh, etc)
// working.
type fancyFailedAuthnResponseWriterDelegator struct {
	*auditFailedAuthnResponseWriter
}

func (f *fancyFailedAuthnResponseWriterDelegator) CloseNotify() <-chan bool {
	return f.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (f *fancyFailedAuthnResponseWriterDelegator) Flush() {
	f.ResponseWriter.(http.Flusher).Flush()
}

func (f *fancyFailedAuthnResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}

var _ http.CloseNotifier = &fancyFailedAuthnResponseWriterDelegator{}
var _ http.Flusher = &fancyFailedAuthnResponseWriterDelegator{}
var _ http.Hijacker = &fancyFailedAuthnResponseWriterDelegator{}
