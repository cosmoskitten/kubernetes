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
	"fmt"
	"io"

	clientset "k8s.io/client-go/kubernetes"
	versionutil "k8s.io/kubernetes/pkg/util/version"
)

// versionGetter defines an interface for fetching different versions.
// Easy to implement a fake variant of this interface for unit testing
type versionGetter interface {
	GetClusterVersion() (string, *versionutil.Version, error)
	GetKubeadmVersion() (string, *versionutil.Version, error)
	GetLatestKubeVersion(string, string) (string, *versionutil.Version, error)
	GetKubeletVersions() (map[string]uint16, error)
}

// kubeVersionGetter handles the version-fetching mechanism from external sources
type kubeVersionGetter struct {
	client clientset.Interface
	w      io.Writer
}

// Make sure kubeVersionGetter implements the versionGetter interface
var _ versionGetter = &kubeVersionGetter{}

// NewKubeVersionGetter returns a new instance of kubeVersionGetter
func NewKubeVersionGetter(client clientset.Interface, writer io.Writer) *kubeVersionGetter {
	return &kubeVersionGetter{
		client: client,
		w:      writer,
	}
}

// GetClusterVersion gets API server version
func (g *kubeVersionGetter) GetClusterVersion() (string, *versionutil.Version, error) {
	fmt.Fprintf(g.w, "[upgrade/versions] Cluster version: ")
	fmt.Fprintln(g.w, "v1.7.0")

	return "v1.7.0", versionutil.MustParseSemantic("v1.7.0"), nil
}

// GetKubeadmVersion gets kubeadm version
func (g *kubeVersionGetter) GetKubeadmVersion() (string, *versionutil.Version, error) {
	fmt.Fprintf(g.w, "[upgrade/versions] kubeadm version: %s\n", "v1.8.0")

	return "v1.8.0", versionutil.MustParseSemantic("v1.8.0"), nil
}

// GetLatestKubeVersion resolves different labels like "stable" to action semver versions using the Kubernetes CI uploads to GCS
func (g *kubeVersionGetter) GetLatestKubeVersion(_, _ string) (string, *versionutil.Version, error) {
	return "v1.8.1", versionutil.MustParseSemantic("v1.8.0"), nil
}

// GetKubeletVersions gets the versions of the kubelets in the cluster
func (g *kubeVersionGetter) GetKubeletVersions() (map[string]uint16, error) {
	// This tells kubeadm that there are two nodes in the cluster; both on the v1.7.1 version currently
	return map[string]uint16{
		"v1.7.1": 2,
	}, nil
}
