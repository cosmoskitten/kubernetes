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

package system

// GetSysSpec returns os specific SysSpec
var DefaultSysSpec = SysSpec{
	OS: "Microsoft Windows Server 2016",
	KernelSpec: KernelSpec{
		Versions:  []string{`10\.[0-9]\.1439[3-9]*`, `10\.[0-9]\.144[0-9]*`, `10\.[0-9]\.15[0-9]*`, `10\.[0-9]\.2[0-9]*`}, //requires >= '10.0.14393'
		Required:  []KernelConfig{},
		Optional:  []KernelConfig{},
		Forbidden: []KernelConfig{},
	},
	Cgroups: []string{},
	RuntimeSpec: RuntimeSpec{
		DockerSpec: &DockerSpec{
			Version:     []string{`17\.03\..*`}, //Requires [17.03] or later
			GraphDriver: []string{"windowsfilter"},
		},
	},
}
