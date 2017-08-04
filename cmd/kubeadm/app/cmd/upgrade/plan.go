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
	"os"
	"sort"
	"text/tabwriter"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	clientset "k8s.io/client-go/kubernetes"
	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeconfigutil "k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig"
)

// NewCmdPlan returns the cobra command for `kubeadm upgrade plan`
func NewCmdPlan(parentFlags *cmdUpgradeFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Check which versions are available to upgrade to",
		Run: func(_ *cobra.Command, _ []string) {
			err := RunPlan(parentFlags)
			kubeadmutil.CheckErr(err)
		},
	}

	return cmd
}

// RunPlan takes care of outputting available versions to upgrade to for the user
func RunPlan(parentFlags *cmdUpgradeFlags) error {

	// Start with the basics, verify that the cluster is healthy and build a client
	client, _, err := EnforceRequirements(parentFlags.kubeConfigPath, parentFlags.cfgPath, parentFlags.printConfig)
	if err != nil {
		return err
	}

	versionGetterImpl := upgrade.NewKubeVersionGetter(client, os.Stdout)
	availUpgrades, err := upgrade.GetAvailableUpgrades(versionGetterImpl, parentFlags.allowExperimentalUpgrades, parentFlags.allowRCUpgrades)
	if err != nil {
		return err
	}

	// Tell the user which upgrades are available
	printAvailableUpgrades(availUpgrades)
	return nil
}

// EnforceRequirements verifies that it's okay to upgrade and returns the clientset and configuration needed
func EnforceRequirements(kubeConfigPath, cfgPath string, printConfig bool) (clientset.Interface, *kubeadmapiext.MasterConfiguration, error) {
	client, err := kubeconfigutil.ClientSetFromFile(kubeConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create a Kubernetes client from file %q: %v", kubeConfigPath, err)
	}

	// Run healthchecks against the cluster
	if !upgrade.VerifyClusterHealth(client) {
		return nil, nil, fmt.Errorf("the cluster is not in an upgradeable state")
	}
	// Fetch the configuration from a file or ConfigMap and validate it
	cfg, err := upgrade.FetchConfiguration(client, os.Stdout, cfgPath)
	if err != nil {
		return nil, nil, err
	}

	// If the user told us to print this information out; do it!
	if printConfig {
		printConfiguration(cfg)
	}

	return client, cfg, nil
}

// printAvailableUpgrades prints a UX-friendly overview of what versions are available to upgrade to
func printAvailableUpgrades(upgrades []upgrade.Upgrade) {

	// Return earlier if no upgrades can be made
	if len(upgrades) == 0 {
		fmt.Println("Awesome, you're up-to-date! Enjoy!")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 10, 4, 3, ' ', 0)

	// Loop through the upgrade possibilities and output text to the command line
	for _, upgrade := range upgrades {

		if upgrade.CanUpgradeKubelets() {
			fmt.Println("Components that must be upgraded manually after you've upgraded the control plane with `kubeadm upgrade apply`:")
			fmt.Fprintln(w, "COMPONENT\tCURRENT\tAVAILABLE")
			firstPrinted := false

			// The map is of the form <old-version>:<node-count>. Here all the keys are put into a slice and sorted
			// in order to always get the right order. Then the map value is extracted separately
			for _, oldVersion := range sortedSliceFromMap(upgrade.Before.KubeletVersions) {
				nodeCount := upgrade.Before.KubeletVersions[oldVersion]
				if !firstPrinted {
					// Output the Kubelet header only on the first version pair
					fmt.Fprintf(w, "Kubelet\t%d x %s\t%s\n", nodeCount, oldVersion, upgrade.After.KubeletVersion)
					firstPrinted = true
					continue
				}
				fmt.Fprintf(w, "\t\t%d x %s\t%s\n", nodeCount, oldVersion, upgrade.After.KubeletVersion)
			}
			w.Flush()
			fmt.Println("")
		}

		fmt.Printf("Upgrade to the latest %s:\n", upgrade.Description)
		fmt.Println("")
		fmt.Fprintln(w, "COMPONENT\tCURRENT\tAVAILABLE")
		fmt.Fprintf(w, "API Server\t%s\t%s\n", upgrade.Before.KubeVersion, upgrade.After.KubeVersion)
		fmt.Fprintf(w, "Controller Manager\t%s\t%s\n", upgrade.Before.KubeVersion, upgrade.After.KubeVersion)
		fmt.Fprintf(w, "Scheduler\t%s\t%s\n", upgrade.Before.KubeVersion, upgrade.After.KubeVersion)
		fmt.Fprintf(w, "Kube Proxy\t%s\t%s\n", upgrade.Before.KubeVersion, upgrade.After.KubeVersion)
		fmt.Fprintf(w, "Kube DNS\t%s\t%s\n", upgrade.Before.DNSVersion, upgrade.After.DNSVersion)
		w.Flush()
		fmt.Println("")
		fmt.Println("You can now apply the upgrade by executing the following command:")
		fmt.Println("")
		fmt.Printf("\tkubeadm upgrade apply --version %s\n", upgrade.After.KubeVersion)
		fmt.Println("")

		if upgrade.Before.KubeadmVersion != upgrade.After.KubeadmVersion {
			fmt.Printf("Note: Before you do can perform this upgrade, you have to update kubeadm to %s\n", upgrade.After.KubeadmVersion)
			fmt.Println("")
		}

		fmt.Println("_____________________________________________________________________")
		fmt.Println("")
	}
}

func printConfiguration(cfg *kubeadmapiext.MasterConfiguration) {
	cfgYaml, err := yaml.Marshal(*cfg)
	if err == nil {
		fmt.Println("[upgrade/config] Configuration used:")

		scanner := bufio.NewScanner(bytes.NewReader(cfgYaml))
		for scanner.Scan() {
			fmt.Printf("\t%s\n", scanner.Text())
		}
	}
}

func sortedSliceFromMap(strMap map[string]int32) []string {
	strSlice := []string{}
	for k := range strMap {
		strSlice = append(strSlice, k)
	}
	sort.Strings(strSlice)
	return strSlice
}
