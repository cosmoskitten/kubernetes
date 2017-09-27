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
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/audit"
)

const (
	// ModeDirect indicates that the backend should process events directly,
	// Serialize event, then write it to logs. This mode spend more time.
	ModeDirect = "direct"
	// ModeBuffer indicates that the audit backend should buffer audit events
	// internally, several consumers get event one by one and do write the
	// real log. If the buffer is full, audit event will be dropped, so no
	// block event producer.
	ModeBuffer = "buffer"
)

// AllowedModes is the modes known by this webhook.
var AllowedModes = []string{
	ModeDirect,
	ModeBuffer,
}

const (
	// FormatLegacy saves event in 1-line text format.
	FormatLegacy = "legacy"
	// FormatJson saves event in structured json format.
	FormatJson = "json"

	// TODO make this configurable
	// worker numbers of routines that concurrently write events.
	workers = 5
	// Buffer up to 1000 events before blocking.
	defaultBufferSize = 1000
)

// The plugin name reported in error metrics.
const pluginName = "log"

// AllowedFormats are the formats known by log backend.
var AllowedFormats = []string{
	FormatLegacy,
	FormatJson,
}

type bufferBackend struct {
	out          io.Writer
	format       string
	groupVersion schema.GroupVersion
	// Channel to buffer events in memory before write them to file.
	buffer chan *auditinternal.Event
}

var _ audit.Backend = &bufferBackend{}

func NewBackend(out io.Writer, format, mode string, groupVersion schema.GroupVersion) (audit.Backend, error) {
	switch mode {
	case ModeBuffer:
		return newBufferBackend(out, format, groupVersion), nil
	case ModeDirect:
		return newDirectBackend(out, format, groupVersion), nil
	default:
		return nil, fmt.Errorf("log mode %q is not in list of known modes (%s)",
			mode, strings.Join(AllowedModes, ","))
	}

}

func newBufferBackend(out io.Writer, format string, groupVersion schema.GroupVersion) audit.Backend {
	return &bufferBackend{
		out:          out,
		format:       format,
		groupVersion: groupVersion,
		buffer:       make(chan *auditinternal.Event, defaultBufferSize),
	}
}

func (b *bufferBackend) ProcessEvents(events ...*auditinternal.Event) {
	for i, event := range events {
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

func (b *bufferBackend) processEvent(ev *auditinternal.Event) {
	logEvent(b.out, b.format, b.groupVersion, ev)
}

func (b *bufferBackend) worker() {
	for event := range b.buffer {
		b.processEvent(event)
	}
}

func (b *bufferBackend) Run(stopCh <-chan struct{}) error {
	glog.Infof("Starting audit log backend")

	for i := 0; i < workers; i++ {
		go wait.Until(b.worker, time.Second, stopCh)
	}

	return nil
}

func (b *bufferBackend) Shutdown() {
	close(b.buffer)
}

type directBackend struct {
	out          io.Writer
	format       string
	groupVersion schema.GroupVersion
}

var _ audit.Backend = &directBackend{}

func newDirectBackend(out io.Writer, format string, groupVersion schema.GroupVersion) audit.Backend {
	return &directBackend{
		out:          out,
		format:       format,
		groupVersion: groupVersion,
	}
}

func (b *directBackend) ProcessEvents(events ...*auditinternal.Event) {
	for _, ev := range events {
		b.processEvent(ev)
	}
}

func (b *directBackend) processEvent(ev *auditinternal.Event) {
	logEvent(b.out, b.format, b.groupVersion, ev)
}

func (b *directBackend) Run(stopCh <-chan struct{}) error {
	return nil
}

func (b *directBackend) Shutdown() {
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
