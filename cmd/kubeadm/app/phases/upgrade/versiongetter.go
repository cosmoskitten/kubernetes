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

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	versionutil "k8s.io/kubernetes/pkg/util/version"
	"k8s.io/kubernetes/pkg/version"
)

// versionGetter defines an interface for fetching different versions.
// Easy to implement a fake variant of this interface for unit testing
type versionGetter interface {
	GetClusterVersion() (string, *versionutil.Version, error)
	GetKubeadmVersion() (string, *versionutil.Version, error)
	GetLatestKubeVersion(string, string) (string, *versionutil.Version, error)
	GetKubeletVersions() (map[string]int32, error)
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
	clusterVersionInfo, err := g.client.Discovery().ServerVersion()
	if err != nil {
		fmt.Fprintln(g.w, "<notfound>")
		fmt.Fprintf(g.w, "[upgrade/versions] FATAL: Couldn't fetch cluster version from the API Server: %v\n", err)
		return "", nil, err
	}
	fmt.Fprintln(g.w, clusterVersionInfo.String())

	clusterVersion, err := versionutil.ParseSemantic(clusterVersionInfo.String())
	if err != nil {
		fmt.Fprintf(g.w, "[upgrade/versions] FATAL: Couldn't parse cluster version: %v\n", err)
		return "", nil, err
	}
	return clusterVersionInfo.String(), clusterVersion, nil
}

// GetKubeadmVersion gets kubeadm version
func (g *kubeVersionGetter) GetKubeadmVersion() (string, *versionutil.Version, error) {
	kubeadmVersionInfo := version.Get()
	fmt.Fprintf(g.w, "[upgrade/versions] kubeadm version: %s\n", kubeadmVersionInfo.String())

	kubeadmVersion, err := versionutil.ParseSemantic(kubeadmVersionInfo.String())
	if err != nil {
		fmt.Fprintf(g.w, "[upgrade/versions] FATAL: Couldn't parse kubeadm version: %v\n", err)
		return "", nil, err
	}
	return kubeadmVersionInfo.String(), kubeadmVersion, nil
}

// GetKubeadmVersion gets the latest versions from CI
func (g *kubeVersionGetter) GetLatestKubeVersion(ciVersionLabel, description string) (string, *versionutil.Version, error) {
	// Do not print anything if description is ""
	if description != "" {
		fmt.Fprintf(g.w, "[upgrade/versions] Latest %s: ", description)
	}

	versionStr, err := kubeadmutil.KubernetesReleaseVersion(ciVersionLabel)
	if err != nil {
		if description != "" {
			fmt.Fprintln(g.w, "<notfound>")
			fmt.Fprintf(g.w, "[upgrade/versions] FATAL: Couldn't fetch latest %s version from the internet: %v\n", description, err)
		}
		return "", nil, err
	}
	if description != "" {
		fmt.Fprintln(g.w, versionStr)
	}

	ver, err := versionutil.ParseSemantic(versionStr)
	if err != nil {
		if description != "" {
			fmt.Fprintf(g.w, "[upgrade/versions] FATAL: Couldn't parse latest %s version: %v\n", description, err)
		}
		return "", nil, err
	}
	return versionStr, ver, nil
}

// GetKubeletVersions gets the versions of the kubelets in the cluster
func (g *kubeVersionGetter) GetKubeletVersions() (map[string]int32, error) {
	nodes, err := g.client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("couldn't list all nodes in cluster")
	}
	return computeKubeletVersions(nodes.Items), nil
}

// computeKubeletVersions returns a string-int map that describes how many nodes are of a specific version
func computeKubeletVersions(nodes []v1.Node) map[string]int32 {
	kubeletVersions := map[string]int32{}
	for _, node := range nodes {
		kver := node.Status.NodeInfo.KubeletVersion
		if _, found := kubeletVersions[kver]; !found {
			kubeletVersions[kver] = 1
			continue
		}
		kubeletVersions[kver] += 1
	}
	return kubeletVersions
}
