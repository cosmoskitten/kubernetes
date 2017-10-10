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

package system

import (
	"os/exec"
)

type KernelValidatorHelper interface {
	GetKernelRelease() ([]byte, error)
}

type DefaultKernelValidatorHelper struct {
}

func (o *DefaultKernelValidatorHelper) GetKernelRelease() ([]byte, error) {
	kernel, err := exec.Command("uname", "-r").CombinedOutput()
	return kernel, err
}

type WindowsKernelValidatorHelper struct {
}

func (o *WindowsKernelValidatorHelper) GetKernelRelease() ([]byte, error) {
	args := []string{"(Get-CimInstance Win32_OperatingSystem).Version"}
	kernel, err := exec.Command("powershell", args...).Output()
	return kernel, err
}
