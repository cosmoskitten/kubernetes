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
)

// TODO: Test that the replicaset stays available during master (and maybe
// node and cluster upgrades).

// ReplicaSetUpgradeTest tests that a replicaset survives upgrade.
type ReplicaSetUpgradeTest struct {
	oldRS *extensions.ReplicaSet
	newRS *extensions.ReplicaSet
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
	rsName := "replicaset-hash-test"
	c := f.ClientSet
	ns := f.Namespace.Name
	nginxImage := imageutils.GetE2EImage(imageutils.NginxSlim)

	By(fmt.Sprintf("Creating a replicaset %q in namespace %q", rsName, ns))
	replicaSet := framework.NewReplicaSet(rsName, ns, int32(1), map[string]string{"test": "upgrade"}, "nginx", nginxImage)
	rs, err := c.Extensions().ReplicaSets(ns).Create(replicaSet)
	framework.ExpectNoError(err)

	// Wait for it to be updated to revision 1
	By(fmt.Sprintf("Waiting replicaset %q to be updated to revision 1", rsName))
	err = framework.WaitForReplicaSetRevisionAndImage(c, ns, rsName, "1", nginxImage, interval, timeout)
	framework.ExpectNoError(err)

	By(fmt.Sprintf("Waiting for replicaset %q to have all of its replicas ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	// Store the old replicaset - should be the same after the upgrade.
	r.oldRS = rs

	// Trigger a new rollout so that we have some history.
	By(fmt.Sprintf("Triggering a new rollout for replicaset %q", rsName))
	rs, err = framework.UpdateReplicaSetWithRetries(c, ns, rsName, func(update *extensions.ReplicaSet) {
		update.Spec.Template.Spec.Containers[0].Name = "updated-name"
	})
	framework.ExpectNoError(err)

	// Use observedGeneration to determine if the controller noticed the pod template update.
	framework.Logf("Wait replicaset %q to be observed by the replicaset controller", rsName)
	framework.ExpectNoError(framework.WaitForObservedReplicaSet(c, ns, rsName, rs.Generation, interval, timeout))

	// Wait for it to be updated to revision 2
	By(fmt.Sprintf("Waiting replicaset %q to be updated to revision 2", rsName))
	framework.ExpectNoError(framework.WaitForReplicaSetRevisionAndImage(c, ns, rsName, "2", nginxImage, interval, timeout))

	By(fmt.Sprintf("Waiting for replicaset %q to have all of its replicas ready.", rsName))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, ns, rsName))

	rs, err = c.Extensions().ReplicaSets(ns).Get(rs.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)
	if rs == nil {
		framework.ExpectNoError(fmt.Errorf("expected a new replicaset"))
	}

	if rs.UID == r.oldRS.UID {
		framework.ExpectNoError(fmt.Errorf("expected a new replicaset different from the previous one"))
	}

	// Store new replicaset - should be the same after the upgrade.
	r.newRS = rs
}

// Test checks whether the replicasets are the same after an upgrade.
func (r *ReplicaSetUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	// Immediately fetch old RS before upgrade is done
	By(fmt.Sprintf("Getting old replicaset %q before upgrade is done", r.oldRS.Name))
	c := f.ClientSet
	oldRS, err := c.Extensions().ReplicaSets(r.oldRS.Namespace).Get(r.oldRS.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)
	if oldRS == nil {
		framework.ExpectNoError(fmt.Errorf("expected an old replicaset %q, but got nil", r.oldRS.Name))
	}

	// Block until upgrade is done
	By(fmt.Sprintf("Waiting for upgrade to finish before checking replicaset %q", oldRS.Name))
	<-done

	By(fmt.Sprintf("Getting new replicaset %q after upgrade is done", oldRS.Name))
	newRS, err := c.Extensions().ReplicaSets(r.oldRS.Namespace).Get(oldRS.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)
	if newRS == nil {
		framework.ExpectNoError(fmt.Errorf("expected a new replicaset %q, but got nil", oldRS.Name))
	}

	By(fmt.Sprintf("Waiting for replicaset %q to have all of its replicas ready.", newRS.Name))
	framework.ExpectNoError(framework.WaitForReadyReplicaSet(c, newRS.Namespace, newRS.Name))

	By(fmt.Sprintf("Checking that replicaset %q is the same as after the upgrade", newRS.Name))
	if newRS.UID != r.newRS.UID {
		By(r.spewReplicaSets(oldRS, newRS))
		framework.ExpectNoError(fmt.Errorf("expected new replicaset:\n%#v\ngot new replicaset:\n%#v\n", r.newRS, newRS))
	}

	By(fmt.Sprintf("Checking that replicaset %q is the same as prior to the upgrade", oldRS.Name))
	if oldRS.UID != r.oldRS.UID {
		By(r.spewReplicaSets(oldRS, newRS))
		framework.ExpectNoError(fmt.Errorf("expected old replicaset:\n%#v\ngot old replicaset:\n%#v\n", r.oldRS, oldRS))
	}
}

// Teardown cleans up any remaining resources.
func (r *ReplicaSetUpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

func (r *ReplicaSetUpgradeTest) spewReplicaSets(oldRS *extensions.ReplicaSet, newRS *extensions.ReplicaSet) string {
	msg := fmt.Sprintf("old replicaset prior to the upgrade:\n%#v\n", r.oldRS)
	msg += fmt.Sprintf("new replicaset prior to the upgrade:\n%#v\n", r.newRS)
	msg += fmt.Sprintf("old replicaset after the upgrade:\n%#v\n", oldRS)
	msg += fmt.Sprintf("new replicaset after the upgrade:\n%#v\n", newRS)
	return msg
}
