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
	"text/template"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ghodss/yaml"
	"github.com/renstrom/dedent"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api"
	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/pkg/version"
	versionutil "k8s.io/kubernetes/pkg/util/version"
	clientset "k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
)

var (
	upgradeAvailableTmpl = template.Must(template.New("upgrade").Parse(dedent.Dedent(`

		Upgrade to the latest {{ .NewVersionDescription }}:

						Current version	Upgrade available
		API server:			{{ .CurrentKubeVersion }}			{{ .NewKubeVersion }}
		Controller-manager:	{{ .CurrentKubeVersion }}			{{ .NewKubeVersion }}
		Scheduler:			{{ .CurrentKubeVersion }}			{{ .NewKubeVersion }}
		Kube-proxy:			{{ .CurrentKubeVersion }}			{{ .NewKubeVersion }}
		Kube-dns:			{{ .CurrentDNSVersion }}			{{ .NewDNSVersion }}

		You can now apply the upgrade by executing the following command:

			kubeadm upgrade apply --version {{ .NewKubeVersion }}
		`)))
)

func NewCmdPlan() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Check which versions are available to upgrade to",
		RunE:  func(cmd *cobra.Command, args []string) error {
			kubeConfigPath := cmd.Flags().Lookup("kubeconfig").Value.String()
			cfgPath := cmd.Flags().Lookup("config").Value.String()
			client, err := kubeconfigutil.ClientSetFromFile(kubeConfigPath)
			if err != nil {
				return fmt.Errorf("couldn't create a Kubernetes client from file %q: %v", kubeConfigPath, err)
			}

			if !checkClusterReady(client) {
				return nil
			}
			if !validateConfiguration(client, cfgPath) {
				return nil
			}
			computeAvailableUpgrades(client)
			return nil
		},
	}

	return cmd
}

func validateConfiguration(client clientset.Interface, cfgPath string) bool {
	fmt.Println("--> Making sure the configuration is correct:")

	versionedcfg := &kubeadmapiext.MasterConfiguration{}

	if cfgPath != "" {
		fmt.Println("---> Taking configuration options from a file: %s")
		if err := configutil.TryLoadMasterConfiguration(cfgPath, versionedcfg); err != nil {
			fmt.Println("----> Could not load configuration from the file: %v", err)
			return false
		}
	} else {
		configMapName := "kubeadm-config"
		configMap, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(configMapName, metav1.GetOptions{})
		if err == nil {
			configBytes := []byte(configMap.Data["config"])

			fmt.Println("---> Loading configuration from the cluster (from ConfigMap %q in namespace %q)\n", configMapName, metav1.NamespaceSystem)
			if err := runtime.DecodeInto(api.Codecs.UniversalDecoder(), configBytes, versionedcfg); err != nil {
				fmt.Println("----> Could not load configuration from the cluster: %v")
				return false
			}
		} else {
			fmt.Println("---> Using the default configuration")
			// Sleep here or ask to proceed?
		}
	}
	

	// Take the versioned configuration populated from the configmap, default it and validate
	// Return the internal version of the API object
	cfg, err := configutil.MakeMasterConfigurationFromDefaults("", versionedcfg)
	if err != nil {
		fmt.Println("----> The defaulting or validation for the configuration failed: %v")
		return false
	}
	
	// TODO: Convert back to the versioned API when outputting this to the user
	cfgBytes, err := yaml.Marshal(*cfg)
	if err == nil {
		// TODO: Support different levels of verbosity
		fmt.Printf("---> Configuration used:\n---\n%s---\n", string(cfgBytes))
	}
	return true
}

type upgradeAvailable struct {
	NewVersionDescription, CurrentKubeVersion, CurrentDNSVersion, NewKubeVersion, NewDNSVersion string
}


func computeAvailableUpgrades(client clientset.Interface) bool {
	fmt.Println("--> Fetching available versions:")

	// Collect the upgrades kubeadm can do in this list
	upgrades := []upgradeAvailable{}

	fmt.Printf("---> Cluster version: ")
	clusterVersionInfo, err := client.Discovery().ServerVersion()
	if err != nil {
		fmt.Println("<notfound>")
		fmt.Printf("----> Couldn't fetch cluster version from the API Server: %v\n", err)
		return false
	}
	fmt.Println(clusterVersionInfo.String())

	clusterVersion, err := versionutil.ParseSemantic(clusterVersionInfo.String())
	if err != nil {
		fmt.Printf("----> Couldn't parse cluster version: %v\n", err)
		return false
	}

	kubeadmVersionInfo := version.Get()
	fmt.Printf("---> kubeadm version: %s\n", kubeadmVersionInfo.String())

	kubeadmVersion, err := versionutil.ParseSemantic(kubeadmVersionInfo.String())
	if err != nil {
		fmt.Printf("----> Couldn't parse kubeadm version: %v\n", err)
		return false
	}

	// Get and output the current latest stable version
	stableVersionStr, stableVersion, success := fetchKubeVersionFromCILabel("stable", "stable version")
	if !success {
		return false
	}

	// Do a "dumb guess" that a new minor upgrade is available just because the latest stable version is higher than the cluster version
	// This guess will be corrected once we know if there is a patch version available
	canDoMinorUpgrade := clusterVersion.LessThan(stableVersion)

	// A patch version doesn't exist if the cluster version is higher than the stable version
	// In the case an user is trying to upgrade from, let's say, v1.8.0 to v1.9.0-alpha.2 (given we support such upgrades experimentally)
	// a stable-1.9 branch doesn't exist yet. Hence this check.
	if patchVersionBranchExists(clusterVersion, stableVersion) {

		currentBranch := getBranchFromVersion(clusterVersionInfo.String())
		versionLabel := fmt.Sprintf("stable-%s", currentBranch)
		description := fmt.Sprintf("patch version on the v%s branch", currentBranch)

		// Get and output the latest patch version for the cluster branch
		patchVersionStr, patchVersion, success := fetchKubeVersionFromCILabel(versionLabel, description)
		if !success {
			return false
		}

		// Check if a minor version upgrade is possible when a patch release exists
		// It's only possible if the latest stable version is higher than the current patch version
		// If that's the case, they must be on different branches => a newer minor version can be upgraded to
		canDoMinorUpgrade = minorUpgradePossibleWithPatchRelease(stableVersion, patchVersion)

		// If the cluster version is lower than the newest patch version, we should inform about the possible upgrade
		if patchUpgradePossible(clusterVersion, patchVersion) {
			upgrades = append(upgrades, upgradeAvailable{
				NewVersionDescription: description,
				NewKubeVersion: patchVersionStr,
				NewDNSVersion: "vX.Y.Z",
				CurrentKubeVersion: clusterVersionInfo.String(),
				CurrentDNSVersion: "vX.Y.Z",
			})
		}
	}

	// If the kubeadm version is lower than the latest stable version, notify the user that kubeadm should be upgraded first
	if kubeadmVersion.LessThan(stableVersion) {
		fmt.Printf("----> Note! The kubeadm CLI should be upgraded to %s (using your package manager) first, if you want to upgrade your cluster to %s\n", stableVersionStr, stableVersionStr)
	}

	
	if canDoMinorUpgrade {
		upgrades = append(upgrades, upgradeAvailable{
			NewVersionDescription: "stable version",
			NewKubeVersion: stableVersionStr	,
			NewDNSVersion: "vX.Y.Z",
			CurrentKubeVersion: clusterVersionInfo.String(),
			CurrentDNSVersion: "vX.Y.Z",
		})
	}

	// Return earlier if no upgrades can be made
	if len(upgrades) == 0 {
		fmt.Println("----> Awesome, you're up-to-date! Enjoy!")
		return true
	}

	// Loop through the upgrade possibilities and output text to the command line based on a template
	for _, upgrade := range upgrades {
		upgradeAvailableTmpl.Execute(os.Stdout, upgrade)
	}
	fmt.Printf("\nYou should upgrade the kubelets in your cluster via your package manager.")

	return true
}

func getBranchFromVersion(version string) string {
	return strings.TrimPrefix(version, "v")[:3]
}

func patchVersionBranchExists(clusterVersion, stableVersion *versionutil.Version) bool {
	return stableVersion.AtLeast(clusterVersion)
}

func patchUpgradePossible(clusterVersion, patchVersion *versionutil.Version) bool {
	return clusterVersion.LessThan(patchVersion)
}

func minorUpgradePossibleWithPatchRelease(stableVersion, patchVersion *versionutil.Version) bool {
	return patchVersion.LessThan(stableVersion)
}

func fetchKubeVersionFromCILabel(ciVersionLabel, description string) (string, *versionutil.Version, bool) {
	fmt.Printf("---> Latest %s: ", description)

	versionStr, err := kubeadmutil.KubernetesReleaseVersion(ciVersionLabel)
	if err != nil {
		fmt.Println("<notfound>")
		fmt.Printf("----> Couldn't fetch %s version from the internet: %v\n", description, err)
		return "", nil, false
	}
	ver, err := versionutil.ParseSemantic(versionStr)
	if err != nil {
		fmt.Printf("----> Couldn't parse %s version: %v\n", description, err)
		return "", nil, false
	}
	fmt.Println(versionStr)
	return versionStr, ver, true
}
