// +build windows

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

package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/utils/exec"

	"github.com/golang/glog"
)

type Mounter struct {
	mounterPath string
}

func (mounter *Mounter) Mount(source string, target string, fstype string, options []string) error {
	target = normalizeWindowsPath(target)

	if source == "tmpfs" {
		glog.Infof("windowsMount: mounting source (%q), target (%q), with options (%q)", source, target, options)
		return os.MkdirAll(target, 0755)
	}

	parentDir := filepath.Dir(target)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return err
	}

	glog.V(4).Infof("windowsMount: mount options(%q) source:%q, target:%q, fstype:%q, begin to mount",
		options, source, target, fstype)
	bindSource := ""
	ex := exec.New()

	if bind, _ := isBind(options); bind {
		bindSource = normalizeWindowsPath(source)
	} else {
		// mount azure file
		if len(options) < 2 {
			glog.Warningf("windowsMount: mount options(%q) command number(%d) less than 2, source:%q, target:%q, skip mounting",
				options, len(options), source, target)
			return nil
		}
		cmd := fmt.Sprintf(`$User = "AZURE\%s";$PWord = ConvertTo-SecureString -String "%s" -AsPlainText -Force;`+
			`$Credential = New-Object -TypeName System.Management.Automation.PSCredential -ArgumentList $User, $PWord`,
			options[0], options[1])

		driverLetter, err := getAvailableDriveLetter()
		if err != nil {
			return err
		}
		bindSource = driverLetter + ":"
		cmd += fmt.Sprintf(";New-SmbGlobalMapping -LocalPath %s -RemotePath %s -Credential $Credential", bindSource, source)

		if output, err := ex.Command("powershell", "/c", cmd).CombinedOutput(); err != nil {
			// we don't return error here, even though New-SmbGlobalMapping failed, we still make it successful,
			// will return error when Windows 2016 RS3 is ready on azure
			glog.Errorf("windowsMount: SmbGlobalMapping failed: %v, output: %q", err, string(output))
			return os.MkdirAll(target, 0755)
		}
	}

	if output, err := ex.Command("cmd", "/c", "mklink", "/D", target, bindSource).CombinedOutput(); err != nil {
		glog.Errorf("mklink failed: %v, source(%q) target(%q) output: %q", err, bindSource, target, string(output))
		return fmt.Errorf("mklink failed: %v, output: %q", err, string(output))
	}

	return nil
}

func (mounter *Mounter) Unmount(target string) error {
	glog.V(4).Infof("windowsMount: Unmount target (%q)", target)
	target = normalizeWindowsPath(target)
	ex := exec.New()
	if output, err := ex.Command("cmd", "/c", "rmdir", target).CombinedOutput(); err != nil {
		return fmt.Errorf("rmdir failed: %v, output: %q", err, string(output))
	}
	return nil
}

func (mounter *Mounter) List() ([]MountPoint, error) {
	return []MountPoint{}, nil
}

func (mounter *Mounter) IsMountPointMatch(mp MountPoint, dir string) bool {
	return mp.Path == dir
}

func (mounter *Mounter) IsNotMountPoint(dir string) (bool, error) {
	return IsNotMountPoint(mounter, dir)
}

func (mounter *Mounter) IsLikelyNotMountPoint(file string) (bool, error) {
	stat, err := os.Lstat(file)
	if err != nil {
		return true, err
	}
	// If current file is a symlink, then it is a mountpoint.
	if stat.Mode()&os.ModeSymlink != 0 {
		return false, nil
	}

	return true, nil
}

func (mounter *Mounter) GetDeviceNameFromMount(mountPath, pluginDir string) (string, error) {
	return getDeviceNameFromMount(mounter, mountPath, pluginDir)
}

func (mounter *Mounter) DeviceOpened(pathname string) (bool, error) {
	return false, nil
}

func (mounter *Mounter) PathIsDevice(pathname string) (bool, error) {
	return false, nil
}

func (mounter *SafeFormatAndMount) formatAndMount(source string, target string, fstype string, options []string) error {
	// Try to mount the disk
	glog.V(4).Infof("Attempting to formatAndMount disk: %s %s %s", fstype, source, target)

	if err := validateDiskNumber(source); err != nil {
		glog.Errorf("azureDisk Mount: formatAndMount failed, err: %v\n", err)
		return err
	}

	driveLetter, err := getDriveLetterByDiskNumber(source)
	if err != nil {
		return err
	}
	driverPath := driveLetter + ":"
	target = normalizeWindowsPath(target)
	glog.V(4).Infof("Attempting to formatAndMount disk: %s %s %s", fstype, driverPath, target)
	ex := exec.New()
	if output, err := ex.Command("cmd", "/c", "mklink", "/D", target, driverPath).CombinedOutput(); err != nil {
		return fmt.Errorf("mklink failed: %v, output: %q", err, string(output))
	}
	return nil
}

func getAvailableDriveLetter() (string, error) {
	cmd := "$used = Get-PSDrive | Select-Object -Expand Name | Where-Object { $_.Length -eq 1 }"
	cmd += ";$drive = 67..90 | ForEach-Object { [string][char]$_ } | Where-Object { $used -notcontains $_ } | Select-Object -First 1;$drive"
	ex := exec.New()
	output, err := ex.Command("powershell", "/c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("getAvailableDriveLetter failed: %v, output: %q", err, string(output))
	}

	if len(output) == 0 {
		return "", fmt.Errorf("windowsMount: there is no available drive letter now")
	}
	return string(output)[:1], nil
}

func normalizeWindowsPath(path string) string {
	normalizedPath := strings.Replace(path, "/", "\\", -1)
	if strings.HasPrefix(normalizedPath, "\\") {
		normalizedPath = "c:" + normalizedPath
	}
	return normalizedPath
}

func normalizeWindowsPath(path string) string {
	normalizedPath := strings.Replace(path, "/", "\\", -1)

	if strings.HasPrefix(normalizedPath, "\\") {
		normalizedPath = "c:" + normalizedPath
	}
	return normalizedPath
}

func validateDiskNumber(disk string) error {
	if len(disk) < 1 || len(disk) > 2 {
		return fmt.Errorf("wrong disk number format: %q", disk)
	}

	_, err := strconv.Atoi(disk)
	return err
}

func getDriveLetterByDiskNumber(diskNum string) (string, error) {
	ex := exec.New()
	cmd := fmt.Sprintf("(Get-Partition -DiskNumber %s).DriveLetter", diskNum)
	output, err := ex.Command("powershell", "/c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("windowsMount: Get Drive Letter failed: %v, output: %q", err, string(output))
	}
	if len(string(output)) < 1 {
		return "", fmt.Errorf("windowsMount: Get Drive Letter failed, output is empty")
	}
	return string(output)[:1], nil
}
