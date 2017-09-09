// +build !linux,!windows

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

package host_path

import (
	"fmt"
	"os"

	"k8s.io/api/core/v1"
)

func (dftc *defaultFileTypeChecker) getFileType(_ string, _ os.FileInfo) (v1.HostPathType, error) {
	return "", fmt.Errorf("unsupported to get file type on current OS")
}
