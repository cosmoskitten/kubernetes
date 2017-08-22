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

// FailedAuthenticationDecorator decorates a response writer with a fallback audit,
// logging failed authentication requests only when the main audit was not triggered.
type FailedAuthenticationDecorator interface {
	Decorate(rw http.ResponseWriter, user user.Info, req *http.Request) http.ResponseWriter
}

// NewAuditFailedAuthenticationDecorator returns decorator responsible for
// logging failed authentication attempts.
func NewAuditFailedAuthenticationDecorator(requestContextMapper request.RequestContextMapper, sink audit.Sink, policy policy.Checker) *auditFailedAuthenticationDecorator {
	if sink == nil || policy == nil {
		return nil
	}
	return &auditFailedAuthenticationDecorator{
		requestContextMapper: requestContextMapper,
		sink:                 sink,
		policy:               policy,
	}
}

var _ FailedAuthenticationDecorator = &auditFailedAuthenticationDecorator{}

type auditFailedAuthenticationDecorator struct {
	requestContextMapper request.RequestContextMapper
	sink                 audit.Sink
	policy               policy.Checker
}

func (f *auditFailedAuthenticationDecorator) Decorate(rw http.ResponseWriter, user user.Info, req *http.Request) http.ResponseWriter {
	delegate := &auditFailedAuthnResponseWriter{
		ResponseWriter:       rw,
		user:                 user,
		request:              req,
		requestContextMapper: f.requestContextMapper,
		sink:                 f.sink,
		policy:               f.policy,
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

		// if there already exists an audit event in the context we don't need to do anything
		if ae := request.AuditEventFrom(ctx); ae != nil {
			return
		}

		// otherwise, we need to create the event and log the authn error
		ev, err := createAuditEvent(ctx, a.policy, a.requestContextMapper, a.request, a.ResponseWriter)
		if err != nil {
			return
		}
		if ev == nil {
			return
		}

		// since user is not set at this point, we need to read it manually
		if a.user != nil {
			if len(a.user.GetName()) != 0 {
				// return username if available
				ev.User.Username = a.user.GetName()
			} else {
				// or at least return auth method tried
				if authMethod, ok := a.user.GetExtra()["auth-name"]; ok {
					ev.User.Username = fmt.Sprintf("<%s>", authMethod)
				}
			}
		}
		ctx = request.WithAuditEvent(ctx, ev)
		if err := a.requestContextMapper.Update(a.request, ctx); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to attach audit event to the context: %v", err))
			responsewriters.InternalError(a.ResponseWriter, a.request, errors.New("failed to update context"))
			return
		}

		ev.ResponseStatus = &metav1.Status{}
		ev.ResponseStatus.Code = int32(code)
		ev.Stage = auditinternal.StageResponseStarted
		processAuditEvent(a.sink, ev)
		a.event = ev
	})
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
