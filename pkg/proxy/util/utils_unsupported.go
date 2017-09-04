// +build !linux

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
	"fmt"
)

type NetlinkHandle interface {
}

type EmptyHandle struct {
}

func NewNetLinkHandle() NetlinkHandle {
	return &EmptyHandle{}
}

// EnsureAddressBind checks if address is bound to the interface and, if not, binds it. If the address is already bound, return true.
func EnsureAddressBind(address, devName string, h NetlinkHandle) (exist bool, err error) {
	return false, fmt.Errorf("netlink not supported for this platform")
}

// UnbindAddress unbind address with the interface
func UnbindAddress(address, devName string, h NetlinkHandle) error {
	return fmt.Errorf("netlink not supported for this platform")
}
