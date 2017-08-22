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
	"encoding/base64"
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
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// WithFailedAuthnAudit decorates a http.Handler with a fallback audit, logging
// information only when the main one was not triggered. It is meant only for
// logging failed authentication requests.
func WithFailedAuthnAudit(handler http.Handler, requestContextMapper request.RequestContextMapper, sink audit.Sink, policy policy.Checker) http.Handler {
	if sink == nil || policy == nil {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		respWriter := decorateFailedAuthnResponseWriter(w, getUsername(req), req, requestContextMapper, sink, policy)
		handler.ServeHTTP(respWriter, req)
	})
}

func decorateFailedAuthnResponseWriter(responseWriter http.ResponseWriter, username string, req *http.Request,
	requestContextMapper request.RequestContextMapper, sink audit.Sink, policy policy.Checker) http.ResponseWriter {
	delegate := &auditFailedAuthnResponseWriter{
		ResponseWriter:       responseWriter,
		username:             username,
		request:              req,
		requestContextMapper: requestContextMapper,
		sink:                 sink,
		policy:               policy,
	}
	// check if the ResponseWriter we're wrapping is the fancy one we need
	// or if the basic is sufficient
	_, cn := responseWriter.(http.CloseNotifier)
	_, fl := responseWriter.(http.Flusher)
	_, hj := responseWriter.(http.Hijacker)
	if cn && fl && hj {
		return &fancyFailedAuthnResponseWriterDelegator{delegate}
	}
	return delegate
}

var _ http.ResponseWriter = &auditFailedAuthnResponseWriter{}

// auditFailedAuthnResponseWriter intercepts WriteHeader, sets it in the event.
type auditFailedAuthnResponseWriter struct {
	http.ResponseWriter
	once                 sync.Once
	username             string
	request              *http.Request
	requestContextMapper request.RequestContextMapper
	sink                 audit.Sink
	policy               policy.Checker
}

func (a *auditFailedAuthnResponseWriter) Write(bs []byte) (int, error) {
	a.processCode(http.StatusOK) // the Go library calls WriteHeader internally if no code was written yet. But this will go unnoticed for us
	return a.ResponseWriter.Write(bs)
}

func (a *auditFailedAuthnResponseWriter) WriteHeader(code int) {
	a.processCode(code)
	a.ResponseWriter.WriteHeader(code)
}

func (a *auditFailedAuthnResponseWriter) processCode(code int) {
	a.once.Do(func() {
		ctx, ok := a.requestContextMapper.Get(a.request)
		if !ok {
			responsewriters.InternalError(a.ResponseWriter, a.request, errors.New("no context found for request"))
			return
		}

		// if there already exists an audit event in the context we don't need to do anything
		if ae := request.AuditEventFrom(ctx); ae != nil {
			return
		}

		// otherwise, we need to create the event by ourselves and log the auth error
		// the majority of this code is copied from upstream WithAudit filter
		ev, err := createEvent(ctx, a.policy, a.requestContextMapper, a.request, a.ResponseWriter)
		if err != nil {
			return
		}
		if ev == nil {
			return
		}

		// since user is not set at this point, we need to read it manually
		ev.User.Username = a.username
		ctx = request.WithAuditEvent(ctx, ev)
		if err := a.requestContextMapper.Update(a.request, ctx); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to attach audit event to the context: %v", err))
			responsewriters.InternalError(a.ResponseWriter, a.request, errors.New("failed to update context"))
			return
		}

		ev.ResponseStatus = &metav1.Status{}
		ev.ResponseStatus.Code = int32(code)
		ev.Stage = auditinternal.StageResponseStarted
		processEvent(a.sink, ev)
	})
}

// getUsername returns username or information on the authn method being used.
func getUsername(req *http.Request) string {
	auth := strings.TrimSpace(req.Header.Get("Authorization"))

	// check basic auth
	const basicScheme string = "Basic "
	if strings.HasPrefix(auth, basicScheme) {
		const basic = "<basic>"
		str, err := base64.StdEncoding.DecodeString(auth[len(basicScheme):])
		if err != nil {
			return basic
		}
		cred := strings.SplitN(string(str), ":", 2)
		if len(cred) < 2 {
			return basic
		}
		return cred[0]
	}

	// check bearer token
	parts := strings.Split(auth, " ")
	if len(parts) > 1 && strings.ToLower(parts[0]) == "bearer" {
		return "<bearer>"
	}

	// other tokens
	token := strings.TrimSpace(req.URL.Query().Get("access_token"))
	if len(token) > 0 {
		return "<token>"
	}

	// cert authn
	if req.TLS != nil && len(req.TLS.PeerCertificates) > 0 {
		return "<x509>"
	}

	// TODO: request headers decoding

	return ""
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
