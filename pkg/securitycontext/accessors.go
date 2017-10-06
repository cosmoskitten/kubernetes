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

package securitycontext

import (
	"reflect"

	"k8s.io/kubernetes/pkg/api"
)

type PodSecurityContextAccessor interface {
	HostNetwork() bool
	HostPID() bool
	HostIPC() bool
	SELinuxOptions() *api.SELinuxOptions
	RunAsUser() *int64
	RunAsNonRoot() *bool
	SupplementalGroups() []int64
	FSGroup() *int64
}

type PodSecurityContextMutator interface {
	PodSecurityContextAccessor

	SetHostNetwork(bool)
	SetHostPID(bool)
	SetHostIPC(bool)
	SetSELinuxOptions(*api.SELinuxOptions)
	SetRunAsUser(*int64)
	SetRunAsNonRoot(*bool)
	SetSupplementalGroups([]int64)
	SetFSGroup(*int64)

	PodSecurityContext() *api.PodSecurityContext
}

func NewPodSecurityContextMutator(podSC *api.PodSecurityContext) PodSecurityContextMutator {
	return &PodSecurityContextWrapper{podSC: podSC}
}

type PodSecurityContextWrapper struct {
	podSC *api.PodSecurityContext
}

func (w *PodSecurityContextWrapper) PodSecurityContext() *api.PodSecurityContext {
	return w.podSC
}

func (w *PodSecurityContextWrapper) ensurePodSC() {
	if w.podSC == nil {
		w.podSC = &api.PodSecurityContext{}
	}
}

func (w *PodSecurityContextWrapper) HostNetwork() bool {
	if w.podSC == nil {
		return false
	}
	return w.podSC.HostNetwork
}
func (w *PodSecurityContextWrapper) SetHostNetwork(v bool) {
	if w.podSC == nil && v == false {
		return
	}
	w.ensurePodSC()
	w.podSC.HostNetwork = v
}
func (w *PodSecurityContextWrapper) HostPID() bool {
	if w.podSC == nil {
		return false
	}
	return w.podSC.HostPID
}
func (w *PodSecurityContextWrapper) SetHostPID(v bool) {
	if w.podSC == nil && v == false {
		return
	}
	w.ensurePodSC()
	w.podSC.HostPID = v
}
func (w *PodSecurityContextWrapper) HostIPC() bool {
	if w.podSC == nil {
		return false
	}
	return w.podSC.HostIPC
}
func (w *PodSecurityContextWrapper) SetHostIPC(v bool) {
	if w.podSC == nil && v == false {
		return
	}
	w.ensurePodSC()
	w.podSC.HostIPC = v
}
func (w *PodSecurityContextWrapper) SELinuxOptions() *api.SELinuxOptions {
	if w.podSC == nil {
		return nil
	}
	return w.podSC.SELinuxOptions
}
func (w *PodSecurityContextWrapper) SetSELinuxOptions(v *api.SELinuxOptions) {
	if w.podSC == nil && v == nil {
		return
	}
	w.ensurePodSC()
	w.podSC.SELinuxOptions = v
}
func (w *PodSecurityContextWrapper) RunAsUser() *int64 {
	if w.podSC == nil {
		return nil
	}
	return w.podSC.RunAsUser
}
func (w *PodSecurityContextWrapper) SetRunAsUser(v *int64) {
	if w.podSC == nil && v == nil {
		return
	}
	w.ensurePodSC()
	w.podSC.RunAsUser = v
}
func (w *PodSecurityContextWrapper) RunAsNonRoot() *bool {
	if w.podSC == nil {
		return nil
	}
	return w.podSC.RunAsNonRoot
}
func (w *PodSecurityContextWrapper) SetRunAsNonRoot(v *bool) {
	if w.podSC == nil && v == nil {
		return
	}
	w.ensurePodSC()
	w.podSC.RunAsNonRoot = v
}
func (w *PodSecurityContextWrapper) SupplementalGroups() []int64 {
	if w.podSC == nil {
		return nil
	}
	return w.podSC.SupplementalGroups
}
func (w *PodSecurityContextWrapper) SetSupplementalGroups(v []int64) {
	if w.podSC == nil && len(v) == 0 {
		return
	}
	w.ensurePodSC()
	if len(v) == 0 && len(w.podSC.SupplementalGroups) == 0 {
		return
	}
	w.podSC.SupplementalGroups = v
}
func (w *PodSecurityContextWrapper) FSGroup() *int64 {
	if w.podSC == nil {
		return nil
	}
	return w.podSC.FSGroup
}
func (w *PodSecurityContextWrapper) SetFSGroup(v *int64) {
	if w.podSC == nil && v == nil {
		return
	}
	w.ensurePodSC()
	w.podSC.FSGroup = v
}

type ContainerSecurityContextAccessor interface {
	Capabilities() *api.Capabilities
	Privileged() *bool
	SELinuxOptions() *api.SELinuxOptions
	RunAsUser() *int64
	RunAsNonRoot() *bool
	ReadOnlyRootFilesystem() *bool
	AllowPrivilegeEscalation() *bool
}

type ContainerSecurityContextMutator interface {
	ContainerSecurityContextAccessor

	ContainerSecurityContext() *api.SecurityContext

	SetCapabilities(*api.Capabilities)
	SetPrivileged(*bool)
	SetSELinuxOptions(*api.SELinuxOptions)
	SetRunAsUser(*int64)
	SetRunAsNonRoot(*bool)
	SetReadOnlyRootFilesystem(*bool)
	SetAllowPrivilegeEscalation(*bool)
}

func NewContainerSecurityContextMutator(containerSC *api.SecurityContext) ContainerSecurityContextMutator {
	return &ContainerSecurityContextWrapper{containerSC: containerSC}
}

type ContainerSecurityContextWrapper struct {
	containerSC *api.SecurityContext
}

func (w *ContainerSecurityContextWrapper) ContainerSecurityContext() *api.SecurityContext {
	return w.containerSC
}

func (w *ContainerSecurityContextWrapper) ensureContainerSC() {
	if w.containerSC == nil {
		w.containerSC = &api.SecurityContext{}
	}
}

func (w *ContainerSecurityContextWrapper) Capabilities() *api.Capabilities {
	if w.containerSC == nil {
		return nil
	}
	return w.containerSC.Capabilities
}
func (w *ContainerSecurityContextWrapper) SetCapabilities(v *api.Capabilities) {
	if w.containerSC == nil && v == nil {
		return
	}
	w.ensureContainerSC()
	w.containerSC.Capabilities = v
}
func (w *ContainerSecurityContextWrapper) Privileged() *bool {
	if w.containerSC == nil {
		return nil
	}
	return w.containerSC.Privileged
}
func (w *ContainerSecurityContextWrapper) SetPrivileged(v *bool) {
	if w.containerSC == nil && v == nil {
		return
	}
	w.ensureContainerSC()
	w.containerSC.Privileged = v
}
func (w *ContainerSecurityContextWrapper) SELinuxOptions() *api.SELinuxOptions {
	if w.containerSC == nil {
		return nil
	}
	return w.containerSC.SELinuxOptions
}
func (w *ContainerSecurityContextWrapper) SetSELinuxOptions(v *api.SELinuxOptions) {
	if w.containerSC == nil && v == nil {
		return
	}
	w.ensureContainerSC()
	w.containerSC.SELinuxOptions = v
}
func (w *ContainerSecurityContextWrapper) RunAsUser() *int64 {
	if w.containerSC == nil {
		return nil
	}
	return w.containerSC.RunAsUser
}
func (w *ContainerSecurityContextWrapper) SetRunAsUser(v *int64) {
	if w.containerSC == nil && v == nil {
		return
	}
	w.ensureContainerSC()
	w.containerSC.RunAsUser = v
}
func (w *ContainerSecurityContextWrapper) RunAsNonRoot() *bool {
	if w.containerSC == nil {
		return nil
	}
	return w.containerSC.RunAsNonRoot
}
func (w *ContainerSecurityContextWrapper) SetRunAsNonRoot(v *bool) {
	if w.containerSC == nil && v == nil {
		return
	}
	w.ensureContainerSC()
	w.containerSC.RunAsNonRoot = v
}
func (w *ContainerSecurityContextWrapper) ReadOnlyRootFilesystem() *bool {
	if w.containerSC == nil {
		return nil
	}
	return w.containerSC.ReadOnlyRootFilesystem
}
func (w *ContainerSecurityContextWrapper) SetReadOnlyRootFilesystem(v *bool) {
	if w.containerSC == nil && v == nil {
		return
	}
	w.ensureContainerSC()
	w.containerSC.ReadOnlyRootFilesystem = v
}
func (w *ContainerSecurityContextWrapper) AllowPrivilegeEscalation() *bool {
	if w.containerSC == nil {
		return nil
	}
	return w.containerSC.AllowPrivilegeEscalation
}
func (w *ContainerSecurityContextWrapper) SetAllowPrivilegeEscalation(v *bool) {
	if w.containerSC == nil && v == nil {
		return
	}
	w.ensureContainerSC()
	w.containerSC.AllowPrivilegeEscalation = v
}

func NewEffectiveContainerSecurityContextMutator(podSC PodSecurityContextAccessor, containerSC ContainerSecurityContextMutator) ContainerSecurityContextMutator {
	return &EffectiveContainerSecurityContextWrapper{podSC: podSC, containerSC: containerSC}
}

type EffectiveContainerSecurityContextWrapper struct {
	podSC       PodSecurityContextAccessor
	containerSC ContainerSecurityContextMutator
}

func (w *EffectiveContainerSecurityContextWrapper) ContainerSecurityContext() *api.SecurityContext {
	return w.containerSC.ContainerSecurityContext()
}

func (w *EffectiveContainerSecurityContextWrapper) Capabilities() *api.Capabilities {
	return w.containerSC.Capabilities()
}
func (w *EffectiveContainerSecurityContextWrapper) SetCapabilities(v *api.Capabilities) {
	if !reflect.DeepEqual(w.Capabilities(), v) {
		w.containerSC.SetCapabilities(v)
	}
}
func (w *EffectiveContainerSecurityContextWrapper) Privileged() *bool {
	return w.containerSC.Privileged()
}
func (w *EffectiveContainerSecurityContextWrapper) SetPrivileged(v *bool) {
	if !reflect.DeepEqual(w.Privileged(), v) {
		w.containerSC.SetPrivileged(v)
	}
}
func (w *EffectiveContainerSecurityContextWrapper) SELinuxOptions() *api.SELinuxOptions {
	if v := w.containerSC.SELinuxOptions(); v != nil {
		return v
	}
	return w.podSC.SELinuxOptions()
}
func (w *EffectiveContainerSecurityContextWrapper) SetSELinuxOptions(v *api.SELinuxOptions) {
	if !reflect.DeepEqual(w.SELinuxOptions(), v) {
		w.containerSC.SetSELinuxOptions(v)
	}
}
func (w *EffectiveContainerSecurityContextWrapper) RunAsUser() *int64 {
	if v := w.containerSC.RunAsUser(); v != nil {
		return v
	}
	return w.podSC.RunAsUser()
}
func (w *EffectiveContainerSecurityContextWrapper) SetRunAsUser(v *int64) {
	if !reflect.DeepEqual(w.RunAsUser(), v) {
		w.containerSC.SetRunAsUser(v)
	}
}
func (w *EffectiveContainerSecurityContextWrapper) RunAsNonRoot() *bool {
	if v := w.containerSC.RunAsNonRoot(); v != nil {
		return v
	}
	return w.podSC.RunAsNonRoot()
}
func (w *EffectiveContainerSecurityContextWrapper) SetRunAsNonRoot(v *bool) {
	if !reflect.DeepEqual(w.RunAsNonRoot(), v) {
		w.containerSC.SetRunAsNonRoot(v)
	}
}
func (w *EffectiveContainerSecurityContextWrapper) ReadOnlyRootFilesystem() *bool {
	return w.containerSC.ReadOnlyRootFilesystem()
}
func (w *EffectiveContainerSecurityContextWrapper) SetReadOnlyRootFilesystem(v *bool) {
	if !reflect.DeepEqual(w.ReadOnlyRootFilesystem(), v) {
		w.containerSC.SetReadOnlyRootFilesystem(v)
	}
}
func (w *EffectiveContainerSecurityContextWrapper) AllowPrivilegeEscalation() *bool {
	return w.containerSC.AllowPrivilegeEscalation()
}
func (w *EffectiveContainerSecurityContextWrapper) SetAllowPrivilegeEscalation(v *bool) {
	if !reflect.DeepEqual(w.AllowPrivilegeEscalation(), v) {
		w.containerSC.SetAllowPrivilegeEscalation(v)
	}
}
