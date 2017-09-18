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

	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/util/version"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	. "github.com/onsi/ginkgo"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

const (
	interval = 2 * time.Second
	timeout  = 1 * time.Minute
	rsName   = "replicaset-hash-test"
)

// TODO: Test that the replicaset stays available during master (and maybe
// node and cluster upgrades).

// ReplicaSetUpgradeTest tests that a replicaset survives upgrade.
type ReplicaSetUpgradeTest struct {
	oldRSStatus extensions.ReplicaSetStatus
	newRSStatus extensions.ReplicaSetStatus
}

func (ReplicaSetUpgradeTest) Name() string { return "[sig-apps] replicaset-upgrade" }

func (ReplicaSetUpgradeTest) Skip(upgCtx upgrades.UpgradeContext) bool {
	// As of 1.8, the client code we call into no longer supports talking to a server <1.7. (see #47685)
	minVersion := version.MustParseSemantic("v1.7.0")

	for _, vCtx := range upgCtx.Versions {
		if vCtx.Version.LessThan(minVersion) {
			return true
		}
	}
	return false
}

var _ upgrades.Skippable = ReplicaSetUpgradeTest{}

// Setup creates a replicaset and makes sure it has a new and an old replicaset running.
// This calls in to client code and should not be expected to work against a cluster more than one minor version away from the current version.
func (r *ReplicaSetUpgradeTest) Setup(f *framework.Framework) {
	c := f.ClientSet
	ns := f.Namespace.Name
	nginxImage := imageutils.GetE2EImage(imageutils.NginxSlim)

	By(fmt.Sprintf("Creating a replicaset %q in namespace %q", rsName, ns))
	replicaSet := framework.NewReplicaSet(rsName, ns, int32(1), map[string]string{"test": "upgrade"}, "nginx", nginxImage)
	rs, err := c.Extensions().ReplicaSets(ns).Create(replicaSet)
	framework.ExpectNoError(err)

	By(fmt.Sprintf("Waiting replicaset %q's container image to be updated", rsName))
	err = framework.WaitForReplicaSetImage(c, ns, rsName, nginxImage, interval, timeout)
	framework.ExpectNoError(err)

	By(fmt.Sprintf("Waiting for replicaset %q to have all of its replicas ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	// Store the old RS status.
	r.oldRSStatus = rs.Status
	oldUID := rs.UID

	// Trigger a new rollout so that we have some history.
	By(fmt.Sprintf("Triggering a new rollout for replicaset %q", rsName))
	rs, err = framework.UpdateReplicaSetWithRetries(c, ns, rsName, func(update *extensions.ReplicaSet) {
		update.Spec.Template.Spec.Containers[0].Name = "updated-nginx"
	})
	framework.ExpectNoError(err)

	// Use observedGeneration to determine if the controller noticed the pod template update.
	framework.Logf("Wait replicaset %q to be observed by the replicaset controller", rsName)
	framework.ExpectNoError(framework.WaitForObservedReplicaSet(c, ns, rsName, rs.Generation, interval, timeout))

	By(fmt.Sprintf("Waiting for replicaset %q to have all of its replicas ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	rs, err = c.Extensions().ReplicaSets(ns).Get(rs.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)
	if rs == nil {
		framework.ExpectNoError(fmt.Errorf("expected a replicaset"))
	}

	// Store new RS status.
	r.newRSStatus = rs.Status

	if rs.UID != oldUID {
		framework.ExpectNoError(fmt.Errorf("expected getting the same replicaset after the new rollout"))
	}
}

// Test checks whether the replicasets are the same after an upgrade.
func (r *ReplicaSetUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	c := f.ClientSet
	ns := f.Namespace.Name

	// Block until upgrade is done
	By(fmt.Sprintf("Waiting for upgrade to finish before checking replicaset %q", rsName))
	<-done

	By(fmt.Sprintf("Getting new status for replicaset %q after upgrade is done", rsName))
	newRS, err := c.Extensions().ReplicaSets(ns).Get(rsName, metav1.GetOptions{})
	framework.ExpectNoError(err)
	if newRS == nil {
		framework.ExpectNoError(fmt.Errorf("expected a replicaset %q, but got nil", rsName))
	}
	newRSStatus := newRS.Status

	By(fmt.Sprintf("Waiting for replicaset %q to have all of its replicas ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	By(fmt.Sprintf("Checking that new status of replicaset %q is the same as after the upgrade", rsName))
	if !reflect.DeepEqual(newRSStatus, r.newRSStatus) {
		framework.ExpectNoError(fmt.Errorf("expected new replicaset status:\n%#v\ngot:\n%#v\n", r.newRSStatus, newRSStatus))
	}
}

// Teardown cleans up any remaining resources.
func (r *ReplicaSetUpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
