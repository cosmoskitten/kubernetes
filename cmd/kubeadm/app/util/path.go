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

package util

import (
	"path/filepath"
	"runtime"
)

// NormalizePath takes a slash-separated path (which would be a valid unix path) and ensures it is in the correct
// format for the os in which it is currently running (currently used to convert unix path to windows path)
// ex given "/etc/kubernetes/pki/ca.crt" on windows, it will return "C:\etc\kubernetes\pki\ca.crt"
// on linux, the call is a no-op
func NormalizePath(path string) string {
	if runtime.GOOS == "windows" {
		if filepath.VolumeName(path) == "" {
			return filepath.FromSlash(filepath.Join("C:", path))
		}
		return filepath.FromSlash(path)
	}
	return path
}
