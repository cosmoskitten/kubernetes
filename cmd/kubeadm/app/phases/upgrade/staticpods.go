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
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/controlplane"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	"k8s.io/kubernetes/pkg/util/version"
)

const (
	waitForComponentReadyTimeout = 1 * time.Minute
)

// UpgradeStaticPodControlPlane upgrades a static pod-hosted control plane
func UpgradeStaticPodControlPlane(client clientset.Interface, cfg *kubeadmapi.MasterConfiguration, newK8sVersion *version.Version) error {

	// This string-string map stores the component name and backup filepath (if a rollback is needed).
	// If a rollback is needed,
	recoverManifests := map[string]string{}

	tmpdir, err := ioutil.TempDir("", "kubeadm-upgrade")
	defer os.RemoveAll(tmpdir)
	if err != nil {
		return fmt.Errorf("couldn't create a temporary directory for the upgrade: %v")
	}

	backupManifestPath := filepath.Join(tmpdir, "old-manifests")
	if err := os.MkdirAll(backupManifestPath, 0700); err != nil {
		return fmt.Errorf("couldn't create a temporary directory for the upgrade: %v")
	}

	beforePodHashMap, err := apiclient.WaitForStaticPodControlPlaneHashes(client, waitForComponentReadyTimeout, cfg.NodeName)
	if err != nil {
		return err
	}

	// Write the updated static Pod manifests into the temporary directory
	err = controlplane.CreateInitStaticPodManifestFiles(tmpdir, cfg)

	fmt.Printf("[upgrade/staticpods] Wrote upgraded Static Pod manifests to %q\n", tmpdir)

	for _, component := range constants.MasterComponents {
		// The old manifest is here; in the /etc/kubernetes/
		currentManifestPath := constants.GetStaticPodFilepath(component, constants.GetStaticPodDirectory())
		// The old manifest will be moved here; into a subfolder of the temporary directory
		// If a rollback is needed, these manifests will be put back to where they where initially
		backupManifestPath := constants.GetStaticPodFilepath(component, backupManifestPath)
		// The new, upgraded manifest will be written here
		newManifestPath := constants.GetStaticPodFilepath(component, tmpdir)

		// Store the backup path in the recover list. If something goes wrong now, this component will be rolled back.
		recoverManifests[component] = backupManifestPath

		// Move the old manifest into the old-manifests directory
		if err := os.Rename(currentManifestPath, backupManifestPath); err != nil {
			// We need to rollback the old version
			return rollbackOldManifests(recoverManifests, err)
		}

		// Move the new manifest into the manifests directory
		if err := os.Rename(newManifestPath, currentManifestPath); err != nil {
			// We need to rollback the old version
			return rollbackOldManifests(recoverManifests, err)
		}

		fmt.Printf("[upgrade/staticpods] Moved upgraded manifest to %q and backed up old manifest to %q\n", currentManifestPath, backupManifestPath)
		fmt.Println("[upgrade/staticpods] Waiting for the kubelet to restart the component")

		// Wait for the mirror Pod hash to change; otherwise we'll run into race conditions here when the kubelet hasn't had time to
		// notice the removal of the Static Pod, leading to a false positive below where we check that the API endpoint is healthy
		// If we don't do this, there is a case where we remove the Static Pod manifest, kubelet is slow to react, kubeadm checks the
		// API endpoint below of the OLD Static Pod component and proceeds quickly enough, which might lead to unexpected results.
		if err := apiclient.WaitForStaticPodControlPlaneHashChange(client, waitForComponentReadyTimeout, cfg.NodeName, component, beforePodHashMap[component]); err != nil {
			// We need to rollback the old version
			return rollbackOldManifests(recoverManifests, err)
		}

		// Wait for the static pod component to come up and register itself as a mirror pod
		if err := apiclient.WaitForPodsWithLabel(client, waitForComponentReadyTimeout, os.Stdout, "component="+component); err != nil {
			// We need to rollback the old version
			return rollbackOldManifests(recoverManifests, err)
		}

		fmt.Printf("[upgrade/staticpods] Component %q upgraded successfully!\n", component)
	}
	return nil
}

func rollbackOldManifests(oldManifests map[string]string, origErr error) error {
	errs := []error{origErr}
	for component, backupPath := range oldManifests {
		// Where we should put back the backed up manifest
		realManifestPath := constants.GetStaticPodFilepath(component, constants.GetStaticPodDirectory())

		// Move the backup manifest back into the manifests directory
		err := os.Rename(backupPath, realManifestPath)
		if err != nil {
			errs = append(errs, err)
		}
	}
	// Let the user know there we're problems, but we tried to reçover
	return fmt.Errorf("couldn't upgrade control plane. kubeadm has tried to recover everything into the earlier state. Errors faced: %v", errs)
}
