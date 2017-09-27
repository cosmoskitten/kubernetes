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

type backend struct {
	out          io.Writer
	format       string
	groupVersion schema.GroupVersion
	// Channel to buffer events in memory before write them to file.
	buffer chan *auditinternal.Event
}

var _ audit.Backend = &backend{}

func NewBackend(out io.Writer, format string, groupVersion schema.GroupVersion) audit.Backend {
	return &backend{
		out:          out,
		format:       format,
		groupVersion: groupVersion,
		buffer:       make(chan *auditinternal.Event, defaultBufferSize),
	}
}

func (b *backend) ProcessEvents(events ...*auditinternal.Event) {
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

func (b *backend) logEvent(ev *auditinternal.Event) {
	line := ""
	switch b.format {
	case FormatLegacy:
		line = audit.EventString(ev) + "\n"
	case FormatJson:
		bs, err := runtime.Encode(audit.Codecs.LegacyCodec(b.groupVersion), ev)
		if err != nil {
			audit.HandlePluginError(pluginName, err, ev)
			return
		}
		line = string(bs[:])
	default:
		audit.HandlePluginError(pluginName, fmt.Errorf("log format %q is not in list of known formats (%s)",
			b.format, strings.Join(AllowedFormats, ",")), ev)
		return
	}
	if _, err := fmt.Fprint(b.out, line); err != nil {
		audit.HandlePluginError(pluginName, err, ev)
	}
}

func (b *backend) worker() {
	for {
		event, ok := <-b.buffer
		if !ok {
			return
		}
		b.logEvent(event)
	}
}

func (b *backend) Run(stopCh <-chan struct{}) error {
	glog.Infof("Starting audit log backend")

	for i := 0; i < workers; i++ {
		go wait.Until(b.worker, time.Second, stopCh)
	}

	return nil
}

func (b *backend) Shutdown() {
	close(b.buffer)
}

