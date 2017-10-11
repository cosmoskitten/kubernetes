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

package system

import (
	"os/exec"
)

// KernelValidatorHelper is an interface intended to help with os specific kernel validation
type KernelValidatorHelper interface {
	// GetKernelRelease gets the current kernel release version of the system
	GetKernelRelease() ([]byte, error)
}

// DefaultKernelValidatorHelper is the 'linux' implementation of KernelValidatorHelper
type DefaultKernelValidatorHelper struct {
}

func (o *DefaultKernelValidatorHelper) GetKernelRelease() ([]byte, error) {
	kernel, err := exec.Command("uname", "-r").CombinedOutput()
	return kernel, err
}

// WindowsKernelValidatorHelper is the 'windows' implementation of KernelValidatorHelper
type WindowsKernelValidatorHelper struct {
}

func (o *WindowsKernelValidatorHelper) GetKernelRelease() ([]byte, error) {
	args := []string{"(Get-CimInstance Win32_OperatingSystem).Version"}
	kernel, err := exec.Command("powershell", args...).Output()
	return kernel, err
}
