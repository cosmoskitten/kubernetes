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

package upgrade

import (
	"bytes"
	"reflect"
	"testing"

	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
)

func TestSortedSliceFromStringIntMap(t *testing.T) {
	var tests = []struct {
		strMap        map[string]uint16
		expectedSlice []string
	}{ // The returned slice should be alphabetically sorted based on the string keys in the map
		{
			strMap:        map[string]uint16{"foo": 1, "bar": 2},
			expectedSlice: []string{"bar", "foo"},
		},
		{ // The int value should not affect this func
			strMap:        map[string]uint16{"foo": 2, "bar": 1},
			expectedSlice: []string{"bar", "foo"},
		},
		{
			strMap:        map[string]uint16{"b": 2, "a": 1, "cb": 0, "ca": 1000},
			expectedSlice: []string{"a", "b", "ca", "cb"},
		},
		{ // This should work for version numbers as well; and the lowest version should come first
			strMap:        map[string]uint16{"v1.7.0": 1, "v1.6.1": 1, "v1.6.2": 1, "v1.8.0": 1, "v1.8.0-alpha.1": 1},
			expectedSlice: []string{"v1.6.1", "v1.6.2", "v1.7.0", "v1.8.0", "v1.8.0-alpha.1"},
		},
	}
	for _, rt := range tests {
		actualSlice := sortedSliceFromStringIntMap(rt.strMap)
		if !reflect.DeepEqual(actualSlice, rt.expectedSlice) {
			t.Errorf(
				"failed SortedSliceFromStringIntMap:\n\texpected: %v\n\t  actual: %v",
				rt.expectedSlice,
				actualSlice,
			)
		}
	}
}

func TestPrintConfiguration(t *testing.T) {
	var tests = []struct {
		cfg           *kubeadmapiext.MasterConfiguration
		buf           *bytes.Buffer
		expectedBytes []byte
	}{
		{
			cfg:           nil,
			expectedBytes: []byte(""),
		},
		{
			cfg: &kubeadmapiext.MasterConfiguration{
				KubernetesVersion: "v1.7.1",
			},
			expectedBytes: []byte(`[upgrade/config] Configuration used:
	api:
	  advertiseAddress: ""
	  bindPort: 0
	apiServerCertSANs: null
	apiServerExtraArgs: null
	authorizationModes: null
	certificatesDir: ""
	cloudProvider: ""
	controllerManagerExtraArgs: null
	etcd:
	  caFile: ""
	  certFile: ""
	  dataDir: ""
	  endpoints: null
	  extraArgs: null
	  image: ""
	  keyFile: ""
	featureFlags: null
	imageRepository: ""
	kubernetesVersion: v1.7.1
	networking:
	  dnsDomain: ""
	  podSubnet: ""
	  serviceSubnet: ""
	nodeName: ""
	schedulerExtraArgs: null
	token: ""
	tokenTTL: 0
	unifiedControlPlaneImage: ""
`),
		},
		{
			cfg: &kubeadmapiext.MasterConfiguration{
				KubernetesVersion: "v1.7.1",
				Networking: kubeadmapiext.Networking{
					ServiceSubnet: "10.96.0.1/12",
				},
			},
			expectedBytes: []byte(`[upgrade/config] Configuration used:
	api:
	  advertiseAddress: ""
	  bindPort: 0
	apiServerCertSANs: null
	apiServerExtraArgs: null
	authorizationModes: null
	certificatesDir: ""
	cloudProvider: ""
	controllerManagerExtraArgs: null
	etcd:
	  caFile: ""
	  certFile: ""
	  dataDir: ""
	  endpoints: null
	  extraArgs: null
	  image: ""
	  keyFile: ""
	featureFlags: null
	imageRepository: ""
	kubernetesVersion: v1.7.1
	networking:
	  dnsDomain: ""
	  podSubnet: ""
	  serviceSubnet: 10.96.0.1/12
	nodeName: ""
	schedulerExtraArgs: null
	token: ""
	tokenTTL: 0
	unifiedControlPlaneImage: ""
`),
		},
	}
	for _, rt := range tests {
		rt.buf = bytes.NewBufferString("")
		printConfiguration(rt.cfg, rt.buf)
		actualBytes := rt.buf.Bytes()
		if !bytes.Equal(actualBytes, rt.expectedBytes) {
			t.Errorf(
				"failed PrintConfiguration:\n\texpected: %q\n\t  actual: %q",
				string(rt.expectedBytes),
				string(actualBytes),
			)
		}
	}
}

func TestPrintAvailableUpgrades(t *testing.T) {
	var tests = []struct {
		upgrades      []upgrade.Upgrade
		buf           *bytes.Buffer
		expectedBytes []byte
	}{
		{
			upgrades: []upgrade.Upgrade{},
			expectedBytes: []byte(`Awesome, you're up-to-date! Enjoy!
`),
		},
		{
			upgrades: []upgrade.Upgrade{
				{
					Description: "version in the v1.7 series",
					Before: upgrade.StateBeforeUpgrade{
						KubeVersion: "v1.7.1",
						KubeletVersions: map[string]uint16{
							"v1.7.1": 1,
						},
						KubeadmVersion: "v1.7.2",
						DNSVersion:     "1.14.4",
					},
					After: upgrade.StateAfterUpgrade{
						KubeVersion:    "v1.7.3",
						KubeletVersion: "v1.7.3",
						KubeadmVersion: "v1.7.3",
						DNSVersion:     "1.14.4",
					},
				},
			},
			expectedBytes: []byte(`Components that must be upgraded manually after you've upgraded the control plane with 'kubeadm upgrade apply':
COMPONENT   CURRENT      AVAILABLE
Kubelet     1 x v1.7.1   v1.7.3

Upgrade to the latest version in the v1.7 series:

COMPONENT            CURRENT   AVAILABLE
API Server           v1.7.1    v1.7.3
Controller Manager   v1.7.1    v1.7.3
Scheduler            v1.7.1    v1.7.3
Kube Proxy           v1.7.1    v1.7.3
Kube DNS             1.14.4    1.14.4

You can now apply the upgrade by executing the following command:

	kubeadm upgrade apply --version v1.7.3

Note: Before you do can perform this upgrade, you have to update kubeadm to v1.7.3

_____________________________________________________________________

`),
		},
		{
			upgrades: []upgrade.Upgrade{
				{
					Description: "stable version",
					Before: upgrade.StateBeforeUpgrade{
						KubeVersion: "v1.7.3",
						KubeletVersions: map[string]uint16{
							"v1.7.3": 1,
						},
						KubeadmVersion: "v1.8.0",
						DNSVersion:     "1.14.4",
					},
					After: upgrade.StateAfterUpgrade{
						KubeVersion:    "v1.8.0",
						KubeletVersion: "v1.8.0",
						KubeadmVersion: "v1.8.0",
						DNSVersion:     "1.14.4",
					},
				},
			},
			expectedBytes: []byte(`Components that must be upgraded manually after you've upgraded the control plane with 'kubeadm upgrade apply':
COMPONENT   CURRENT      AVAILABLE
Kubelet     1 x v1.7.3   v1.8.0

Upgrade to the latest stable version:

COMPONENT            CURRENT   AVAILABLE
API Server           v1.7.3    v1.8.0
Controller Manager   v1.7.3    v1.8.0
Scheduler            v1.7.3    v1.8.0
Kube Proxy           v1.7.3    v1.8.0
Kube DNS             1.14.4    1.14.4

You can now apply the upgrade by executing the following command:

	kubeadm upgrade apply --version v1.8.0

_____________________________________________________________________

`),
		},
		{
			upgrades: []upgrade.Upgrade{
				{
					Description: "version in the v1.7 series",
					Before: upgrade.StateBeforeUpgrade{
						KubeVersion: "v1.7.3",
						KubeletVersions: map[string]uint16{
							"v1.7.3": 1,
						},
						KubeadmVersion: "v1.8.1",
						DNSVersion:     "1.14.4",
					},
					After: upgrade.StateAfterUpgrade{
						KubeVersion:    "v1.7.5",
						KubeletVersion: "v1.7.5",
						KubeadmVersion: "v1.8.1",
						DNSVersion:     "1.14.4",
					},
				},
				{
					Description: "stable version",
					Before: upgrade.StateBeforeUpgrade{
						KubeVersion: "v1.7.3",
						KubeletVersions: map[string]uint16{
							"v1.7.3": 1,
						},
						KubeadmVersion: "v1.8.1",
						DNSVersion:     "1.14.4",
					},
					After: upgrade.StateAfterUpgrade{
						KubeVersion:    "v1.8.2",
						KubeletVersion: "v1.8.2",
						KubeadmVersion: "v1.8.2",
						DNSVersion:     "1.14.4",
					},
				},
			},
			expectedBytes: []byte(`Components that must be upgraded manually after you've upgraded the control plane with 'kubeadm upgrade apply':
COMPONENT   CURRENT      AVAILABLE
Kubelet     1 x v1.7.3   v1.7.5

Upgrade to the latest version in the v1.7 series:

COMPONENT            CURRENT   AVAILABLE
API Server           v1.7.3    v1.7.5
Controller Manager   v1.7.3    v1.7.5
Scheduler            v1.7.3    v1.7.5
Kube Proxy           v1.7.3    v1.7.5
Kube DNS             1.14.4    1.14.4

You can now apply the upgrade by executing the following command:

	kubeadm upgrade apply --version v1.7.5

_____________________________________________________________________

Components that must be upgraded manually after you've upgraded the control plane with 'kubeadm upgrade apply':
COMPONENT   CURRENT      AVAILABLE
Kubelet     1 x v1.7.3   v1.8.2

Upgrade to the latest stable version:

COMPONENT            CURRENT   AVAILABLE
API Server           v1.7.3    v1.8.2
Controller Manager   v1.7.3    v1.8.2
Scheduler            v1.7.3    v1.8.2
Kube Proxy           v1.7.3    v1.8.2
Kube DNS             1.14.4    1.14.4

You can now apply the upgrade by executing the following command:

	kubeadm upgrade apply --version v1.8.2

Note: Before you do can perform this upgrade, you have to update kubeadm to v1.8.2

_____________________________________________________________________

`),
		},
		{
			upgrades: []upgrade.Upgrade{
				{
					Description: "experimental version",
					Before: upgrade.StateBeforeUpgrade{
						KubeVersion: "v1.7.5",
						KubeletVersions: map[string]uint16{
							"v1.7.5": 1,
						},
						KubeadmVersion: "v1.7.5",
						DNSVersion:     "1.14.4",
					},
					After: upgrade.StateAfterUpgrade{
						KubeVersion:    "v1.8.0-beta.1",
						KubeletVersion: "v1.8.0-beta.1",
						KubeadmVersion: "v1.8.0-beta.1",
						DNSVersion:     "1.14.4",
					},
				},
			},
			expectedBytes: []byte(`Components that must be upgraded manually after you've upgraded the control plane with 'kubeadm upgrade apply':
COMPONENT   CURRENT      AVAILABLE
Kubelet     1 x v1.7.5   v1.8.0-beta.1

Upgrade to the latest experimental version:

COMPONENT            CURRENT   AVAILABLE
API Server           v1.7.5    v1.8.0-beta.1
Controller Manager   v1.7.5    v1.8.0-beta.1
Scheduler            v1.7.5    v1.8.0-beta.1
Kube Proxy           v1.7.5    v1.8.0-beta.1
Kube DNS             1.14.4    1.14.4

You can now apply the upgrade by executing the following command:

	kubeadm upgrade apply --version v1.8.0-beta.1

Note: Before you do can perform this upgrade, you have to update kubeadm to v1.8.0-beta.1

_____________________________________________________________________

`),
		},
		{
			upgrades: []upgrade.Upgrade{
				{
					Description: "release candidate version",
					Before: upgrade.StateBeforeUpgrade{
						KubeVersion: "v1.7.5",
						KubeletVersions: map[string]uint16{
							"v1.7.5": 1,
						},
						KubeadmVersion: "v1.7.5",
						DNSVersion:     "1.14.4",
					},
					After: upgrade.StateAfterUpgrade{
						KubeVersion:    "v1.8.0-rc.1",
						KubeletVersion: "v1.8.0-rc.1",
						KubeadmVersion: "v1.8.0-rc.1",
						DNSVersion:     "1.14.4",
					},
				},
			},
			expectedBytes: []byte(`Components that must be upgraded manually after you've upgraded the control plane with 'kubeadm upgrade apply':
COMPONENT   CURRENT      AVAILABLE
Kubelet     1 x v1.7.5   v1.8.0-rc.1

Upgrade to the latest release candidate version:

COMPONENT            CURRENT   AVAILABLE
API Server           v1.7.5    v1.8.0-rc.1
Controller Manager   v1.7.5    v1.8.0-rc.1
Scheduler            v1.7.5    v1.8.0-rc.1
Kube Proxy           v1.7.5    v1.8.0-rc.1
Kube DNS             1.14.4    1.14.4

You can now apply the upgrade by executing the following command:

	kubeadm upgrade apply --version v1.8.0-rc.1

Note: Before you do can perform this upgrade, you have to update kubeadm to v1.8.0-rc.1

_____________________________________________________________________

`),
		},
	}
	for _, rt := range tests {
		rt.buf = bytes.NewBufferString("")
		printAvailableUpgrades(rt.upgrades, rt.buf)
		actualBytes := rt.buf.Bytes()
		if !bytes.Equal(actualBytes, rt.expectedBytes) {
			t.Errorf(
				"failed PrintAvailableUpgrades:\n\texpected: %q\n\t  actual: %q",
				string(rt.expectedBytes),
				string(actualBytes),
			)
		}
	}
}
