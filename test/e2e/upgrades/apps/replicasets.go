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

package upgrades

import (
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	. "github.com/onsi/ginkgo"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

const (
	interval = 10 * time.Second
	timeout  = 5 * time.Minute
	rsName   = "rs"
)

// TODO: Test that the replicaset stays available during master (and maybe
// node and cluster upgrades).

// ReplicaSetUpgradeTest tests that a replicaset survives upgrade.
type ReplicaSetUpgradeTest struct {
	UID types.UID
}

func (ReplicaSetUpgradeTest) Name() string { return "[sig-apps] replicaset-upgrade" }

func (r *ReplicaSetUpgradeTest) Setup(f *framework.Framework) {
	c := f.ClientSet
	ns := f.Namespace.Name
	nginxImage := imageutils.GetE2EImage(imageutils.NginxSlim)

	By(fmt.Sprintf("Creating a replicaset %s in namespace %s", rsName, ns))
	replicaSet := framework.NewReplicaSet(rsName, ns, int32(1), map[string]string{"test": "upgrade"}, "nginx", nginxImage)
	rs, err := c.Extensions().ReplicaSets(ns).Create(replicaSet)
	framework.ExpectNoError(err)

	By(fmt.Sprintf("Waiting for replicaset %s to have all of its replicas ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	r.UID = rs.UID
}

// Test checks whether the replicasets are the same after an upgrade.
func (r *ReplicaSetUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	c := f.ClientSet
	ns := f.Namespace.Name

	// Block until upgrade is done
	By(fmt.Sprintf("Waiting for upgrade to finish before checking replicaset %s", rsName))
	<-done

	// Verify the RS is the same (survives) after the upgrade
	By(fmt.Sprintf("Getting new UID for replicaset %s after upgrade is done", rsName))
	newRS, err := c.Extensions().ReplicaSets(ns).Get(rsName, metav1.GetOptions{})
	framework.ExpectNoError(err)

	By(fmt.Sprintf("Waiting for replicaset %s to have all of its replicas ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	By(fmt.Sprintf("Checking UID to verify replicaset %s is the same after the upgrade", rsName))
	if !reflect.DeepEqual(newRS.UID, r.UID) {
		framework.ExpectNoError(fmt.Errorf("expected new replicaset UID: %v got: %v", r.UID, newRS.UID))
	}

	// Verify the RS is active by scaling up the RS by 1 and ensuring all pods are Ready
	By(fmt.Sprintf("Scaling up the replicaset %s by 1", rsName))
	*newRS.Spec.Replicas = 2
	scaledRS, err := c.Extensions().ReplicaSets(ns).Update(newRS)
	framework.ExpectNoError(err)

	By(fmt.Sprintf("Waiting for replicaset %s to have all of its replicas are ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	By(fmt.Sprintf("Verifying replicaset %s has 2 pods and both are ready.", rsName))
	if scaledRS.Status.Replicas != 2 && scaledRS.Status.ReadyReplicas != 2 {
		framework.ExpectNoError(fmt.Errorf("expected 2 ready pods for rs %s, got: %d", rsName, scaledRS.Status.ReadyReplicas))
	}
}

// Teardown cleans up any remaining resources.
func (r *ReplicaSetUpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
