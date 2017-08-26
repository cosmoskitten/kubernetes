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

package flexvolume

import (
	"github.com/fsnotify/fsnotify"
)

// A callback-based filesystem watcher abstraction for fsnotify.
type FSWatcher interface {
	// Initializes the watcher with the given watch handlers.
	// When an event or error occurs, the corresponding handler is called.
	Init(FSEventHandler, FSErrorHandler)

	// Add a filesystem path to watch
	AddWatch(path string) error
}
type FSEventHandler func(event fsnotify.Event)
type FSErrorHandler func(err error)

type fsnotifyWatcher struct {
	*fsnotify.Watcher
}

var _ FSWatcher = &fsnotifyWatcher{}

func NewFsnotifyWatcher() (FSWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &fsnotifyWatcher{Watcher: watcher}, nil
}

func (w *fsnotifyWatcher) AddWatch(path string) error {
	return w.Add(path)
}

func (w *fsnotifyWatcher) Init(eventHandler FSEventHandler, errorHandler FSErrorHandler) {
	go func() {
		defer w.Close()
		for {
			select {
			case event := <-w.Events:
				if eventHandler != nil {
					eventHandler(event)
				}
			case err := <-w.Errors:
				if errorHandler != nil {
					errorHandler(err)
				}
			}
		}
	}()
}
