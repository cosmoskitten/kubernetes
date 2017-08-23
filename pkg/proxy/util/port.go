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

	"github.com/golang/glog"
)

// LocalPort describes a port on specific IP address and protocol
type LocalPort struct {
	// Desc is the identity message of a given local port
	Desc string
	// IP is the IP address part of a given local port.
	// For node port service, the IP filed is empty to represent all IP addresses in the node.
	IP string
	// Port is the port part of a given local port
	Port int
	// Protocol is the protocol part of a given local port
	Protocol string
}

func (lp *LocalPort) String() string {
	return fmt.Sprintf("%q (%s:%d/%s)", lp.Desc, lp.IP, lp.Port, lp.Protocol)
}

// Closeable is an interface around closing an port.
type Closeable interface {
	Close() error
}

// PortOpener is an interface around port opening/closing.
// Abstracted out for testing.
type PortOpener interface {
	OpenLocalPort(lp *LocalPort) (Closeable, error)
}

// RevertPorts is closing ports in replacementPortsMap but not in originalPortsMap. In other words, it only
// closes the ports opened in this sync.
func RevertPorts(replacementPortsMap, originalPortsMap map[LocalPort]Closeable) {
	for k, v := range replacementPortsMap {
		// Only close newly opened local ports - leave ones that were open before this update
		if originalPortsMap[k] == nil {
			glog.V(2).Infof("Closing local port %s", k.String())
			v.Close()
		}
	}
}
