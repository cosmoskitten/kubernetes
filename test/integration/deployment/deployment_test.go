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

package deployment

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	"k8s.io/kubernetes/test/integration/framework"
)

func TestNewDeployment(t *testing.T) {
	s, closeFn, rm, dc, informers, c := dcSetup(t)
	defer closeFn()
	name := "test-new-deployment"
	ns := framework.CreateTestingNamespace(name, s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)

	replicas := int32(20)
	tester := &deploymentTester{t: t, c: c, deployment: newDeployment(name, ns.Name, replicas, "extensions/v1beta1")}
	tester.deployment.Spec.MinReadySeconds = 4

	tester.deployment.Annotations = map[string]string{"test": "should-copy-to-replica-set", v1.LastAppliedConfigAnnotation: "should-not-copy-to-replica-set"}
	deploy, err := c.Extensions().Deployments(ns.Name).Create(tester.deployment)
	if err != nil {
		t.Fatalf("failed to create deployment %s: %v", deploy.Name, err)
	}

	// Start informer and controllers
	stopCh := make(chan struct{})
	defer close(stopCh)
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	go dc.Run(5, stopCh)

	// Wait for the Deployment to be updated to revision 1
	err = tester.waitForDeploymentRevisionAndImage("1", fakeImage)
	if err != nil {
		t.Fatalf("failed to wait for Deployment revision %s: %v", deploy.Name, err)
	}

	// Make sure the Deployment status becomes valid while manually marking Deployment pods as ready at the same time
	tester.waitForDeploymentStatusValidAndMarkPodsReady()

	// Check new RS annotations
	newRS, err := deploymentutil.GetNewReplicaSet(deploy, c.ExtensionsV1beta1())
	if err != nil {
		t.Fatalf("failed to get new ReplicaSet of Deployment %s: %v", deploy.Name, err)
	}
	if newRS.Annotations["test"] != "should-copy-to-replica-set" {
		t.Errorf("expected new ReplicaSet annotations copied from Deployment %s, got: %v", deploy.Name, newRS.Annotations)
	}
	if newRS.Annotations[v1.LastAppliedConfigAnnotation] != "" {
		t.Errorf("expected new ReplicaSet last-applied annotation not copied from Deployment %s", deploy.Name)
	}
}

// TODO(juntee): add tests for selector immutability when apps/v1beta2 infomers and listers are available
func TestDeploymentSelectorMutability(t *testing.T) {
	s, closeFn, rm, dc, informers, c := dcSetup(t)
	defer closeFn()
	name := "test-deployment-selector-mutability"
	ns := framework.CreateTestingNamespace(name, s, t)
	defer framework.DeleteTestingNamespace(ns, s, t)

	replicas := int32(20)
	extensionV1beta1Tester := &deploymentTester{t: t, c: c, deployment: newDeployment(name, ns.Name, replicas, "extensions/v1beta1")}
	extensionV1beta1Tester.deployment.Spec.MinReadySeconds = 4

	deploy, err := c.Extensions().Deployments(ns.Name).Create(extensionV1beta1Tester.deployment)
	if err != nil {
		t.Fatalf("failed to create deployment %s: %v", deploy.Name, err)
	}

	// Start informer and controllers
	stopCh := make(chan struct{})
	defer close(stopCh)
	informers.Start(stopCh)
	go rm.Run(5, stopCh)
	go dc.Run(5, stopCh)

	// Wait for the Deployment to be updated to revision 1
	err = extensionV1beta1Tester.waitForDeploymentRevisionAndImage("1", fakeImage)
	if err != nil {
		t.Fatalf("failed to wait for Deployment revision %s: %v", deploy.Name, err)
	}

	// Make sure the Deployment status becomes valid while manually marking Deployment pods as ready at the same time
	extensionV1beta1Tester.waitForDeploymentStatusValidAndMarkPodsReady()

	// Change selector - selectors are MUTABLE for extensions/v1beta1 only
	newDeployment := extensionV1beta1Tester.deployment
	newSelectorLabels := map[string]string{"changed_name": "changed_test"}
	newDeployment.Spec.Selector.MatchLabels = newSelectorLabels
	newDeployment.Spec.Template.Labels = newSelectorLabels
	deploy, err = c.Extensions().Deployments(ns.Name).Update(newDeployment)
	if err != nil {
		t.Fatalf("failed to update deployment %s: %v", deploy.Name, err)
	}
	if !reflect.DeepEqual(deploy.Spec.Selector.MatchLabels, newSelectorLabels) {
		t.Errorf("selector should be changed for extensions/v1beta1, expected: %v, got: %v", newSelectorLabels, deploy.Spec.Selector.MatchLabels)
	}
}
