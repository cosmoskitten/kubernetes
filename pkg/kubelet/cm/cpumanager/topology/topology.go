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

package topology

import (
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

// CPUDetails is a map from CPU ID to Core ID and Socket ID.
type CPUDetails map[int]CPUInfo

// CPUTopology contains details of node cpu, where :
// CPU  - logical CPU, cadvisor - thread
// Core - physical CPU, cadvisor - Core
// Socket - socket, cadvisor - Node
type CPUTopology struct {
	NumCPUs    int
	NumCores   int
	NumSockets int
	CPUDetails CPUDetails
}

// CPUsPerCore returns the number of logical CPUs are associated with
// each core.
func (topo *CPUTopology) CPUsPerCore() int {
	if topo.NumCores == 0 {
		return 0
	}
	return topo.NumCPUs / topo.NumCores
}

// CPUsPerSocket returns the number of logical CPUs are associated with
// each socket.
func (topo *CPUTopology) CPUsPerSocket() int {
	if topo.NumSockets == 0 {
		return 0
	}
	return topo.NumCPUs / topo.NumSockets
}

// CPUInfo contains the socket and core IDs associated with a CPU.
type CPUInfo struct {
	SocketID int
	CoreID   int
}

// KeepOnly returns a new CPUDetails object with only the supplied cpus.
func (d CPUDetails) KeepOnly(cpus cpuset.CPUSet) CPUDetails {
	result := CPUDetails{}
	for cpu, info := range d {
		if cpus.Contains(cpu) {
			result[cpu] = info
		}
	}
	return result
}

// Sockets returns all of the socket IDs associated with the CPUs in this
// CPUDetails.
func (d CPUDetails) Sockets() cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for _, info := range d {
		b.Add(info.SocketID)
	}
	return b.Result()
}

// CPUsInSocket returns all of the logical CPU IDs associated with the
// given socket ID in this CPUDetails.
func (d CPUDetails) CPUsInSocket(id int) cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for cpu, info := range d {
		if info.SocketID == id {
			b.Add(cpu)
		}
	}
	return b.Result()
}

// Cores returns all of the core IDs associated with the CPUs in this
// CPUDetails.
func (d CPUDetails) Cores() cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for _, info := range d {
		b.Add(info.CoreID)
	}
	return b.Result()
}

// CoresInSocket returns all of the core IDs associated with the given
// socket ID in this CPUDetails.
func (d CPUDetails) CoresInSocket(id int) cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for _, info := range d {
		if info.SocketID == id {
			b.Add(info.CoreID)
		}
	}
	return b.Result()
}

// CPUs returns all of the logical CPU IDs in this CPUDetails.
func (d CPUDetails) CPUs() cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for cpuID := range d {
		b.Add(cpuID)
	}
	return b.Result()
}

// CPUsInCore returns all of the logical CPU IDs associated with the
// given core ID in this CPUDetails.
func (d CPUDetails) CPUsInCore(id int) cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for cpu, info := range d {
		if info.CoreID == id {
			b.Add(cpu)
		}
	}
	return b.Result()
}

// Discover returns CPUTopology based on lscpu output
func Discover() (*CPUTopology, error) {
	out, err := exec.Command("lscpu", "-p").Output()
	if err != nil {
		return nil, fmt.Errorf("could not execute %q", "lscpu -p")
	}

	topo, err := parseTopology(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, fmt.Errorf("could not parse topology")
	}
	return topo, nil
}

func parseTopology(topology string) (*CPUTopology, error) {
	outLines := strings.Split(topology, "\n")
	// lscpu -p output looks like:
	// # comments
	// # comments
	// cpu,core,socket,node,,l1d,l1i,l2,l3
	// cpu,core,socket,node,,l1d,l1i,l2,l3
	CPUDetails := CPUDetails{}
	sockets := make(map[int]struct{})
	cores := make(map[int]struct{})

	for _, line := range outLines {
		// Skip informational header lines
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Skip empty line
		if len(line) == 0 {
			continue
		}

		var cpu, core, socket int
		n, err := fmt.Sscanf(line, "%d,%d,%d", &cpu, &core, &socket)
		if n != 3 {
			return nil, fmt.Errorf("expected to read 3 values but got %q", n)
		}
		if err != nil {
			return nil, fmt.Errorf("Sscanf failed")
		}

		sockets[socket] = struct{}{}
		cores[core] = struct{}{}

		CPUDetails[cpu] = CPUInfo{
			CoreID:   core,
			SocketID: socket,
		}
	}

	if len(CPUDetails) == 0 {
		return nil, fmt.Errorf("could not detect number of cpus")
	}

	return &CPUTopology{
		NumCPUs:    len(CPUDetails),
		NumSockets: len(sockets),
		NumCores:   len(cores),
		CPUDetails: CPUDetails,
	}, nil
}
