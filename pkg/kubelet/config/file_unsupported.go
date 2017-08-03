// +build !linux

/*
Copyright 2016 The Kubernetes Authors.

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

// Reads the pod configuration from file or a directory of files.
package config

import (
	"errors"

	"github.com/golang/glog"
)

func (s *sourceFileListerWatcher) startWatch() {
	glog.Errorf("watching source file is unsupported in this build")
}

func (s *sourceFileListerWatcher) consumeWatchEvent(e *watchEvent) error {
	glog.Errorf("consuming watch event is unsupported in this build")
}
