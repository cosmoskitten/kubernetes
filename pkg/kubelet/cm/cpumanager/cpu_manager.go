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

package cpumanager

import (
	cadvisorapi "github.com/google/cadvisor/info/v1"

	"k8s.io/api/core/v1"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/state"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
)

type kletGetter interface {
	GetPods() []*v1.Pod
	GetCachedMachineInfo() (*cadvisorapi.MachineInfo, error)
}

type runtimeService interface {
	UpdateContainerResources(id string, resources *runtimeapi.LinuxContainerResources) error
}

type policyName string

// Manager interface provides methods for Kubelet to manage pod cpus.
type Manager interface {
	// Start is called during Kubelet initialization.
	//
	// @pre:
	// - cAdvisor is initialized
	Start()

	// RegisterContainer is called between container creation and container start
	// so that initial CPU affinity settings can be written through to the
	// container runtime before the first process begins to execute.
	RegisterContainer(p *v1.Pod, c *v1.Container, containerID string) error

	// UnregisterContainer is called near UnregisterContainer() so that the
	// manager stops trying to update that container in the reconcilation loop
	// and any CPUs dedicated to the container are freed to the shared pool.
	UnregisterContainer(containerID string) error

	// Returns a read-only interface to the CPU manager internal state.
	State() state.Reader
}
