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

package log

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/audit"
)

const (
	// ModeBlocking indicates that the backend should process events directly,
	// Serialize event, then write it to logs. This mode spend more time.
	ModeBlocking = "blocking"
	// ModeBuffered indicates that the audit backend should buffer audit events
	// internally. If the buffer is full, audit event will be dropped, so no
	// block event producer.
	ModeBuffered = "buffered"
)

// AllowedModes is the modes known by this webhook.
var AllowedModes = []string{
	ModeBlocking,
	ModeBuffered,
}

const (
	// FormatLegacy saves event in 1-line text format.
	FormatLegacy = "legacy"
	// FormatJson saves event in structured json format.
	FormatJson = "json"

	// TODO: make this configurable
	// Number of workers that concurrently write events.
	workers = 5
	// Buffer up to 1000 events.
	defaultBufferSize = 1000
)

// The plugin name reported in error metrics.
const pluginName = "log"

// AllowedFormats are the formats known by log backend.
var AllowedFormats = []string{
	FormatLegacy,
	FormatJson,
}

func NewBackend(out io.Writer, format, mode string, groupVersion schema.GroupVersion) (audit.Backend, error) {
	switch mode {
	case ModeBuffered:
		return newBufferedBackend(out, format, groupVersion), nil
	case ModeBlocking:
		return newBlockingBackend(out, format, groupVersion), nil
	default:
		return nil, fmt.Errorf("log mode %q is not in list of known modes (%s)",
			mode, strings.Join(AllowedModes, ","))
	}

}

type bufferedBackend struct {
	out          io.Writer
	format       string
	groupVersion schema.GroupVersion
	buffer       chan *auditinternal.Event
	wg           sync.WaitGroup
}

var _ audit.Backend = &bufferedBackend{}

func newBufferedBackend(out io.Writer, format string, groupVersion schema.GroupVersion) audit.Backend {
	return &bufferedBackend{
		out:          out,
		format:       format,
		groupVersion: groupVersion,
		buffer:       make(chan *auditinternal.Event, defaultBufferSize),
		wg:           sync.WaitGroup{},
	}
}

func (b *bufferedBackend) ProcessEvents(events ...*auditinternal.Event) {
	for i, e := range events {
		// Per the audit.Backend interface these events are reused after being
		// sent to the Sink. Deep copy and send the copy to the queue.
		event := e.DeepCopy()

		// The following mechanism is in place to support the situation when audit
		// events are still coming after the backend was shut down.
		var sendErr error
		func() {
			// If the backend was shut down and the buffer channel was closed, an
			// attempt to add an event to it will result in panic that we should
			// recover from.
			defer func() {
				if err := recover(); err != nil {
					sendErr = errors.New("audit log shut down")
				}
			}()

			select {
			case b.buffer <- event:
			default:
				sendErr = errors.New("audit log queue blocked")
			}
		}()
		if sendErr != nil {
			audit.HandlePluginError(pluginName, sendErr, events[i:]...)
			return
		}
	}
}

func (b *bufferedBackend) processEvent(ev *auditinternal.Event) {
	logEvent(b.out, b.format, b.groupVersion, ev)
}

func (b *bufferedBackend) worker() {
	for event := range b.buffer {
		b.processEvent(event)
	}
	b.wg.Done()
}

func (b *bufferedBackend) Run(stopCh <-chan struct{}) error {
	glog.Infof("Starting audit log backend")

	for i := 0; i < workers; i++ {
		b.wg.Add(1)
		go wait.Until(b.worker, time.Second, stopCh)
	}

	return nil
}

func (b *bufferedBackend) Shutdown() {
	close(b.buffer)
	b.wg.Wait()
}

type blockingBackend struct {
	out          io.Writer
	format       string
	groupVersion schema.GroupVersion
}

var _ audit.Backend = &blockingBackend{}

func newBlockingBackend(out io.Writer, format string, groupVersion schema.GroupVersion) audit.Backend {
	return &blockingBackend{
		out:          out,
		format:       format,
		groupVersion: groupVersion,
	}
}

func (b *blockingBackend) ProcessEvents(events ...*auditinternal.Event) {
	for _, ev := range events {
		b.processEvent(ev)
	}
}

func (b *blockingBackend) processEvent(ev *auditinternal.Event) {
	logEvent(b.out, b.format, b.groupVersion, ev)
}

func (b *blockingBackend) Run(stopCh <-chan struct{}) error {
	return nil
}

func (b *blockingBackend) Shutdown() {
	// Nothing to do here.
}

func logEvent(out io.Writer, format string, groupVersion schema.GroupVersion, ev *auditinternal.Event) {
	line := ""
	switch format {
	case FormatLegacy:
		line = audit.EventString(ev) + "\n"
	case FormatJson:
		bs, err := runtime.Encode(audit.Codecs.LegacyCodec(groupVersion), ev)
		if err != nil {
			audit.HandlePluginError(pluginName, err, ev)
			return
		}
		line = string(bs[:])
	default:
		audit.HandlePluginError(pluginName, fmt.Errorf("log format %q is not in list of known formats (%s)",
			format, strings.Join(AllowedFormats, ",")), ev)
		return
	}
	if _, err := fmt.Fprint(out, line); err != nil {
		audit.HandlePluginError(pluginName, err, ev)
	}
}
