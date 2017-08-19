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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/ghodss/yaml"

	clientset "k8s.io/client-go/kubernetes"
	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
)

// upgradeVariables holds variables needed for performing an upgrade or planning to do so
type upgradeVariables struct {
	client        clientset.Interface
	cfg           *kubeadmapiext.MasterConfiguration
	versionGetter upgrade.VersionGetter
}

// EnforceRequirements verifies that it's okay to upgrade and then returns the variables needed for the rest of the procedure
func EnforceRequirements(kubeConfigPath, cfgPath string, printConfig, nonInteractively bool) (*upgradeVariables, error) {
	client, err := kubeconfigutil.ClientSetFromFile(kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't create a Kubernetes client from file %q: %v", kubeConfigPath, err)
	}

	// Run healthchecks against the cluster
	if err := upgrade.VerifyClusterHealth(client); err != nil {
		return nil, err
	}

	// Fetch the configuration from a file or ConfigMap and validate it
	cfg, err := upgrade.FetchConfiguration(client, os.Stdout, cfgPath, nonInteractively)
	if err != nil {
		return nil, err
	}

	// If the user told us to print this information out; do it!
	if printConfig {
		printConfiguration(cfg, os.Stdout)
	}

	return &upgradeVariables{
		client: client,
		cfg:    cfg,
		// Use a real version getter interface that queries the API server, the kubeadm client and the Kubernetes CI system for latest versions
		versionGetter: upgrade.NewKubeVersionGetter(client, os.Stdout),
	}, nil
}

// printConfiguration prints the external version of the API to yaml
func printConfiguration(cfg *kubeadmapiext.MasterConfiguration, w io.Writer) {
	// Short-circuit if cfg is nil, so we can safely get the value of the pointer below
	if cfg == nil {
		return
	}

	cfgYaml, err := yaml.Marshal(*cfg)
	if err == nil {
		fmt.Fprintln(w, "[upgrade/config] Configuration used:")

		scanner := bufio.NewScanner(bytes.NewReader(cfgYaml))
		for scanner.Scan() {
			fmt.Fprintf(w, "\t%s\n", scanner.Text())
		}
	}
}
