/*
Copyright 2015 The Kubernetes Authors.

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

package fc

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume"
)

// Abstract interface to disk operations.
type diskManager interface {
	MakeGlobalPDName(disk fcDisk) string
	// Attaches the disk to the kubelet's host machine.
	AttachDisk(b fcDiskMounter) (string, error)
	// Detaches the disk from the kubelet's host machine.
	DetachDisk(disk fcDiskUnmounter, devName string) error
}

// utility to mount a disk based filesystem
func diskSetUp(manager diskManager, b fcDiskMounter, volPath string, mounter mount.Interface, fsGroup *int64) error {
	globalPDPath := manager.MakeGlobalPDName(*b.fcDisk)
	noMnt, err := mounter.IsLikelyNotMountPoint(volPath)

	if err != nil && !os.IsNotExist(err) {
		glog.Errorf("cannot validate mountpoint: %s", volPath)
		return err
	}
	if !noMnt {
		return nil
	}
	if err := os.MkdirAll(volPath, 0750); err != nil {
		glog.Errorf("failed to mkdir:%s", volPath)
		return err
	}
	// Perform a bind mount to the full path to allow duplicate mounts of the same disk.
	options := []string{"bind"}
	if b.readOnly {
		options = append(options, "ro")
	}
	err = mounter.Mount(globalPDPath, volPath, "", options)
	if err != nil {
		glog.Errorf("Failed to bind mount: source:%s, target:%s, err:%v", globalPDPath, volPath, err)
		noMnt, mntErr := b.mounter.IsLikelyNotMountPoint(volPath)
		if mntErr != nil {
			glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
			return err
		}
		if !noMnt {
			if mntErr = b.mounter.Unmount(volPath); mntErr != nil {
				glog.Errorf("Failed to unmount: %v", mntErr)
				return err
			}
			noMnt, mntErr = b.mounter.IsLikelyNotMountPoint(volPath)
			if mntErr != nil {
				glog.Errorf("IsLikelyNotMountPoint check failed: %v", mntErr)
				return err
			}
			if !noMnt {
				//  will most likely retry on next sync loop.
				glog.Errorf("%s is still mounted, despite call to unmount().  Will try again next sync loop.", volPath)
				return err
			}
		}
		os.Remove(volPath)

		return err
	}

	if !b.readOnly {
		volume.SetVolumeOwnership(&b, fsGroup)
	}

	return nil
}

// utility to tear down a disk based filesystem
func diskTearDown(manager diskManager, c fcDiskUnmounter, volPath string, mounter mount.Interface) error {
	noMnt, err := mounter.IsLikelyNotMountPoint(volPath)
	if err != nil {
		glog.Errorf("cannot validate mountpoint %s", volPath)
		return err
	}
	if noMnt {
		return os.Remove(volPath)
	}

	if err := mounter.Unmount(volPath); err != nil {
		glog.Errorf("failed to unmount %s", volPath)
		return err
	}

	noMnt, mntErr := mounter.IsLikelyNotMountPoint(volPath)
	if mntErr != nil {
		glog.Errorf("isMountpoint check failed: %v", mntErr)
		return err
	}
	if noMnt {
		if err := os.Remove(volPath); err != nil {
			return err
		}
	}
	return nil
}

// utility to map a device to symbolic link
func deviceSetUp(manager diskManager, b fcDiskMapper, volPath string) error {
	globalPDPath := manager.MakeGlobalPDName(*b.fcDisk)
	if !filepath.IsAbs(globalPDPath) || !filepath.IsAbs(volPath) {
		return fmt.Errorf("These paths should be absolute: globalPDPath: %s, volPath: %s", globalPDPath, volPath)
	}
	devicePath, err := os.Readlink(globalPDPath + "/" + "symlink")
	if err != nil {
		glog.Errorf("failed to readlink: %s", globalPDPath+"/"+"symlink")
		return err
	}
	noMnt, err := b.mounter.IsLikelyNotMountPoint(volPath)
	if !noMnt {
		return fmt.Errorf("%s already mounted. This volume cant' be used as raw block volume.", volPath)
	}
	if err != nil && !os.IsNotExist(err) {
		glog.Errorf("cannot validate global mount path: %s", volPath)
		return err
	}
	if err = os.MkdirAll(volPath, 0750); err != nil {
		return fmt.Errorf("Failed to mkdir %s, error", volPath)
	}
	if err := os.Symlink(devicePath, volPath+"/"+"symlink"); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

// utility to tear down a device
func deviceTearDown(manager diskManager, c fcDiskUnmapper, volPath string) error {
	fi, err := os.Lstat(volPath + "/" + "symlink")
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		err := os.Remove(volPath + "/" + "symlink")
		if err != nil {
			return err
		}
	}
	return nil

}
