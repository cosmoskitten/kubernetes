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
	"net"
	"syscall"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/helper"

	"github.com/golang/glog"
	"github.com/vishvananda/netlink"
)

func IsLocalIP(ip string) (bool, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}
	for i := range addrs {
		intf, _, err := net.ParseCIDR(addrs[i].String())
		if err != nil {
			return false, err
		}
		if net.ParseIP(ip).Equal(intf) {
			return true, nil
		}
	}
	return false, nil
}

func ShouldSkipService(svcName types.NamespacedName, service *api.Service) bool {
	// if ClusterIP is "None" or empty, skip proxying
	if !helper.IsServiceIPSet(service) {
		glog.V(3).Infof("Skipping service %s due to clusterIP = %q", svcName, service.Spec.ClusterIP)
		return true
	}
	// Even if ClusterIP is set, ServiceTypeExternalName services don't get proxied
	if service.Spec.Type == api.ServiceTypeExternalName {
		glog.V(3).Infof("Skipping service %s due to Type=ExternalName", svcName)
		return true
	}
	return false
}

type NetlinkHandle interface {
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	AddrDel(link netlink.Link, addr *netlink.Addr) error
	LinkByName(dev string) (netlink.Link, error)
}

// EnsureAddressBind checks if address is bound to the interface and, if not, binds it. If the address is already bound, return true.
func EnsureAddressBind(address, devName string, h NetlinkHandle) (exist bool, err error) {
	dev, err := h.LinkByName(devName)
	if err != nil {
		return false, fmt.Errorf("error get interface: %s, err: %v", devName, err)
	}
	addr, err := netlink.ParseAddr(address + "/32")
	if err != nil {
		return false, fmt.Errorf("error parse ip address: %s/32, err: %v", address, err)
	}
	if err := h.AddrAdd(dev, addr); err != nil {
		// "EEXIST" will be returned if the address is already bound to device
		if err == syscall.Errno(syscall.EEXIST) {
			return true, nil
		}
		return false, fmt.Errorf("error bind address: %s to interface: %s, err: %v", address, devName, err)
	}
	return false, nil
}

// UnbindAddress unbind address with the interface
func UnbindAddress(address, devName string, h NetlinkHandle) error {
	dev, err := h.LinkByName(devName)
	if err != nil {
		return fmt.Errorf("error get interface: %s, err: %v", devName, err)
	}
	addr, err := netlink.ParseAddr(address + "/32")
	if err != nil {
		return fmt.Errorf("error parse ip address: %s/32, err: %v", address, err)
	}
	if err := h.AddrDel(dev, addr); err != nil {
		return fmt.Errorf("error unbind address: %s from interface: %s, err: %v", address, devName, err)
	}
	return nil
}
