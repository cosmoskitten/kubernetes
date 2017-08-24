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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/websocket"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	bulkapi "k8s.io/apiserver/pkg/apis/bulk"
	bulkvalidation "k8s.io/apiserver/pkg/apis/bulk/validation"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/server/httplog"
	"k8s.io/apiserver/pkg/util/wsstream"
)

// HTTP handler for bulk-watch, stateless.
type watchHTTPHandler struct {
	*APIManager
}

// Handles single bulkwatch connection, keeps connection state.
type bulkWatchHandler struct {

	// Protect mutable state (watches map etc)
	sync.Mutex
	*APIManager

	// Active request conext
	Context request.Context

	serializerInfo runtime.SerializerInfo
	encoder        runtime.Encoder
	decoder        runtime.Decoder

	quitOnce  sync.Once
	quit      chan struct{}
	responses chan *bulkapi.BulkResponse

	watches map[string]*singleWatchState
}

// Holds state for single watch.
type singleWatchState struct {

	// Recheck authorization/admission
	permChecker permCheckFunc

	// Selector
	selector bulkapi.ResourceSelector

	// User provided id (unique per connection)
	id string

	// Underlying watcher
	watcher watch.Interface

	// Encoder for embedded RawExtension/Object
	encoder runtime.Encoder

	// Non-nil when watch was stopped by explicit request.
	stopWatchRequest *bulkapi.BulkRequest

	stopper sync.Once
}

func (h *watchHTTPHandler) responseError(err error, w http.ResponseWriter, req *http.Request) {
	if ctx, ok := h.Mapper.Get(req); !ok {
		panic("request context required")
	} else {
		responsewriters.ErrorNegotiated(ctx, err, h.NegotiatedSerializer, h.GroupVersion, w, req)
	}
}

// ServeHTTP ... DOCME
func (h watchHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	m := h.APIManager
	ctx, ok := m.Mapper.Get(req)
	if !ok {
		responsewriters.InternalError(w, req, errors.New("no context found for request"))
		return
	}

	// negotiate for the stream serializerInfo - TODO: should we create new fn?
	si, err := negotiation.NegotiateOutputStreamSerializer(req, m.NegotiatedSerializer)
	if err != nil {
		h.responseError(apierrors.NewInternalError(err), w, req)
		return
	}

	// TODO: use correct mediaType here
	// - Respect 'Accept' header?
	// - Return err NewNotAcceptableError([]string{"**;stream=bulkwatch"})
	mediaType := si.MediaType
	if mediaType != runtime.ContentTypeJSON {
		mediaType += ";stream=bulk-watch"
	}

	if !wsstream.IsWebSocketRequest(req) {
		err := fmt.Errorf("bulk watch supports only websocket") // FIXME: msg
		h.responseError(apierrors.NewInternalError(err), w, req)
		return
	}

	w = httplog.Unlogged(w)
	w.Header().Set("Content-Type", mediaType)

	wh := &bulkWatchHandler{
		APIManager: m,
		Context:    ctx,

		serializerInfo: si,
		encoder:        m.NegotiatedSerializer.EncoderForVersion(si.StreamSerializer, m.GroupVersion),
		decoder:        m.NegotiatedSerializer.DecoderToVersion(si.StreamSerializer, m.GroupVersion),

		watches:   make(map[string]*singleWatchState),
		responses: make(chan *bulkapi.BulkResponse, 100), // FIXME: buffer size?
		quit:      make(chan struct{}),
	}

	handler := websocket.Handler(wh.HandleWS)
	websocket.Server{Handler: handler}.ServeHTTP(w, req)
}

func (s *bulkWatchHandler) abort(err error) {
	if err != nil {
		utilruntime.HandleError(err)
	}
	s.quitOnce.Do(func() { close(s.quit) })
}

func (s *bulkWatchHandler) resetTimeout(ws *websocket.Conn) {
	if s.WebsocketTimeout > 0 {
		if err := ws.SetDeadline(time.Now().Add(s.WebsocketTimeout)); err != nil {
			utilruntime.HandleError(err)
		}
	}
}

func (s *bulkWatchHandler) sendResponse(req *bulkapi.BulkRequest, resp *bulkapi.BulkResponse) bool {
	if req != nil && req.RequestID != "" {
		resp.RequestID = &req.RequestID
	}
	select {
	case s.responses <- resp:
		return true
	case <-s.quit:
		return false
	}
}

func (s *bulkWatchHandler) sendResponseError(req *bulkapi.BulkRequest, err error) bool {
	status := responsewriters.ErrorToAPIStatus(err)
	return s.sendResponse(req, &bulkapi.BulkResponse{Failure: status})
}

func (s *bulkWatchHandler) readRequestsLoop(ws *websocket.Conn) {
	defer utilruntime.HandleCrash()
	groupKind := bulkapi.Kind("BulkRequest")
	defaultGVK := s.GroupVersion.WithKind("BulkRequest")
	var data []byte

	for {
		s.resetTimeout(ws)
		if err := websocket.Message.Receive(ws, &data); err != nil {
			if err == io.EOF {
				s.abort(nil)
				return
			}
			s.abort(fmt.Errorf("unable to receive message: %v", err))
			return
		}
		if len(data) == 0 {
			continue
		}
		reqRaw, _, err := s.decoder.Decode(data, &defaultGVK, &bulkapi.BulkRequest{})
		if err != nil {
			s.abort(fmt.Errorf("unable to decode bulk request: %v", err))
			return
		}
		req, ok := reqRaw.(*bulkapi.BulkRequest)
		if !ok {
			s.abort(fmt.Errorf("unable to decode bulk request: cast error"))
			return
		}
		if errs := bulkvalidation.ValidateBulkRequest(req); len(errs) > 0 {
			s.sendResponseError(req, apierrors.NewInvalid(groupKind, "", errs))
			continue
		}
		if err = s.handleRequest(req); err != nil {
			s.sendResponseError(req, err)
			continue
		}
	}
}

func (s *bulkWatchHandler) handleRequest(r *bulkapi.BulkRequest) error {
	switch {
	case r.Watch != nil:
		return s.handleNewWatch(r)
	case r.StopWatch != nil:
		return s.handleStopWatch(r)
	default:
		return fmt.Errorf("unknown operation")
	}
}

func (s *bulkWatchHandler) handleStopWatch(r *bulkapi.BulkRequest) error {
	wid := r.StopWatch.WatchID
	w, ok := s.watches[wid]
	if !ok {
		return fmt.Errorf("watch not found")
	}
	if w.stopWatchRequest != nil {
		return fmt.Errorf("watch stop was alredy requested")
	}

	w.stopWatchRequest = r
	w.stopper.Do(w.watcher.Stop)
	return nil
}

func (s *bulkWatchHandler) normalizeSelector(rs *bulkapi.ResourceSelector) {
	if rs.Version == "" {
		rs.Version, _ = s.PreferredVersion[rs.Group]
	}
	if rs.Name != "" {
		nameSelector := fields.OneTermEqualSelector("metadata.name", rs.Name)
		if rs.Options == nil {
			rs.Options = &internalversion.ListOptions{}
		}
		rs.Options.FieldSelector = nameSelector
	}
}

func (s *bulkWatchHandler) handleNewWatch(r *bulkapi.BulkRequest) error {
	s.Lock()
	defer s.Unlock()

	op := r.Watch
	rs := op.Selector
	s.normalizeSelector(&rs)

	if _, ok := s.watches[op.WatchID]; ok {
		return fmt.Errorf("watch %v already exists", op.WatchID)
	}

	gv := schema.GroupVersion{rs.Group, rs.Version}
	groupInfo, ok := s.APIGroups[gv]
	if !ok {
		return fmt.Errorf("unsupported group '%s/%s", rs.Group, rs.Version)
	}
	storage, ok := groupInfo.Storage[rs.Resource]
	if !ok {
		return fmt.Errorf("unsupported resource %v", rs.Resource)
	}
	watcher, ok := storage.(rest.Watcher)
	if !ok {
		return fmt.Errorf("storage doesn't support watching")
	}

	// Watch list or single object, respect namespace.
	ctx := request.WithNamespace(s.Context, rs.Namespace)

	permChecker := s.newAuthorizationCheckerForWatch(ctx, rs)
	if err := permChecker(); err != nil {
		return err
	}
	wifc, err := watcher.Watch(ctx, rs.Options)
	if err != nil {
		return fmt.Errorf("unable to watch: %v", err)
	}

	// FIXME: What serializer should we use here?
	embeddedEncoder := groupInfo.Serializer.EncoderForVersion(s.serializerInfo.Serializer, gv)

	w := &singleWatchState{
		id: op.WatchID, encoder: embeddedEncoder,
		watcher:     wifc,
		selector:    rs,
		permChecker: permChecker,
	}
	s.watches[w.id] = w
	go func() {
		defer utilruntime.HandleCrash()
		defer s.sendWatchStoppedResponse(w)
		defer s.stopAndRemoveWatch(w)
		s.sendWatchStartedResponse(r, w)
		s.runWatchLoop(w)
	}()
	return nil
}

func (s *bulkWatchHandler) stopAndRemoveWatch(w *singleWatchState) {
	s.Lock()
	defer s.Unlock()
	w.watcher.Stop()
	delete(s.watches, w.id)
}

// Consumes objects from 'responses' channel and write them into the websocket connection.
func (s *bulkWatchHandler) runResponsesLoop(ws *websocket.Conn) {
	defer utilruntime.HandleCrash()
	streamBuf := &bytes.Buffer{}
	for {
		select {
		case response, ok := <-s.responses:
			if !ok {
				return
			}
			s.resetTimeout(ws)
			if err := s.encoder.Encode(response, streamBuf); err != nil {
				s.abort(fmt.Errorf("unable to encode event: %v", err))
				return
			}
			var data interface{}
			if s.serializerInfo.EncodesAsText {
				data = streamBuf.String()
			} else {
				data = streamBuf.Bytes()
			}
			if err := websocket.Message.Send(ws, data); err != nil {
				s.abort(err)
				return
			}
			streamBuf.Reset()
		case <-s.quit:
			return
		}
	}
}

func serializeEmbeddedObject(obj runtime.Object, e runtime.Encoder) (runtime.Object, error) {
	// TODO: Fixup(obj) (selfLink etc)
	buf := &bytes.Buffer{}
	if err := e.Encode(obj, buf); err != nil {
		return nil, fmt.Errorf("unable to encode watch object: %v", err)
	}
	// ContentType is not required here because Raw already contains correctly encoded data
	return &runtime.Unknown{Raw: buf.Bytes()}, nil
}

func (s *bulkWatchHandler) sendWatchStoppedResponse(w *singleWatchState) {
	s.sendResponse(w.stopWatchRequest, &bulkapi.BulkResponse{WatchStopped: &bulkapi.WatchStopped{WatchID: w.id}})
}

func (s *bulkWatchHandler) sendWatchStartedResponse(r *bulkapi.BulkRequest, w *singleWatchState) {
	s.sendResponse(r, &bulkapi.BulkResponse{WatchStarted: &bulkapi.WatchStarted{WatchID: w.id}})
}

func (s *bulkWatchHandler) runWatchLoop(w *singleWatchState) {
	ch := w.watcher.ResultChan()
	for {
		select {
		case <-s.quit:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			err := w.permChecker()
			if err == nil {
				event.Object, err = serializeEmbeddedObject(event.Object, w.encoder)
			}
			if err != nil {
				s.sendResponse(nil, &bulkapi.BulkResponse{
					WatchEvent: &bulkapi.BulkWatchEvent{
						WatchID: w.id,
						Event:   watch.Event{Type: watch.Error, Object: responsewriters.ErrorToAPIStatus(err)},
					}})
				return
			}
			s.sendResponse(nil, &bulkapi.BulkResponse{
				WatchEvent: &bulkapi.BulkWatchEvent{
					WatchID: w.id,
					Event:   event,
				}})
		}
	}
}

func (s *bulkWatchHandler) HandleWS(ws *websocket.Conn) {
	defer ws.Close()
	go s.readRequestsLoop(ws)
	go s.runResponsesLoop(ws)

	select {
	case <-s.quit:
		glog.V(10).Infof("bulkWatchHandler{} was quit")
		return
	case <-s.Context.Done():
		err := s.Context.Err()
		glog.V(10).Infof("Context was done due to %v", err)
		s.abort(err)
		return
	}
}
