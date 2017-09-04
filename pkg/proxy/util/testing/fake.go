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

package testing

import (
	"syscall"

	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/util/sets"
)

//FakeNetlinkHandle mock implementation of proxy NetlinkHandle
type FakeNetlinkHandle struct {
	BoundIP map[string]sets.String
}

//NewFakeNetlinkHandle will create a new FakeNetlinkHandle
func NewFakeNetlinkHandle() *FakeNetlinkHandle {
	return &FakeNetlinkHandle{
		BoundIP: make(map[string]sets.String),
	}
}

//LinkByName is a mock implementation
func (fake *FakeNetlinkHandle) LinkByName(dev string) (netlink.Link, error) {
	device := &netlink.Device{}
	device.Name = dev
	return device, nil
}

//AddrAdd is a mock implementation
func (fake *FakeNetlinkHandle) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	if address, ok := fake.BoundIP[link.Attrs().Name]; ok {
		if address.Has(addr.String()) {
			return syscall.Errno(syscall.EEXIST)
		}
		fake.BoundIP[link.Attrs().Name].Insert(addr.String())
	} else {
		fake.BoundIP[link.Attrs().Name] = sets.NewString(addr.String())
	}
	return nil
}

//AddrDel is a mock implementation
func (fake *FakeNetlinkHandle) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	if address, ok := fake.BoundIP[link.Attrs().Name]; ok {
		if address.Has(addr.String()) {
			fake.BoundIP[link.Attrs().Name].Delete(addr.String())
			return nil
		}
	}
	return syscall.Errno(syscall.EADDRNOTAVAIL)
}
