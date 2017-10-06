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
	args := []string {"(Get-CimInstance Win32_OperatingSystem).Version"}
	kernel, err := exec.Command("powershell", args...).Output()
	return kernel, err
}
