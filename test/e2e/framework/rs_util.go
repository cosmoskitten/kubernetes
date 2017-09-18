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

package framework

import (
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	. "github.com/onsi/ginkgo"

	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	testutils "k8s.io/kubernetes/test/utils"
)

type updateRsFunc func(d *extensions.ReplicaSet)

func UpdateReplicaSetWithRetries(c clientset.Interface, namespace, name string, applyUpdate updateRsFunc) (*extensions.ReplicaSet, error) {
	var rs *extensions.ReplicaSet
	var updateErr error
	pollErr := wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		var err error
		if rs, err = c.Extensions().ReplicaSets(namespace).Get(name, metav1.GetOptions{}); err != nil {
			return false, err
		}
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(rs)
		if rs, err = c.Extensions().ReplicaSets(namespace).Update(rs); err == nil {
			Logf("Updating replicaset %q", name)
			return true, nil
		}
		updateErr = err
		return false, nil
	})
	if pollErr == wait.ErrWaitTimeout {
		pollErr = fmt.Errorf("couldn't apply the provided updated to replicaset %q: %v", name, updateErr)
	}
	return rs, pollErr
}

// CheckNewRSAnnotations check if the new RS's annotation is as expected
func CheckNewRSAnnotations(c clientset.Interface, ns, deploymentName string, expectedAnnotations map[string]string) error {
	deployment, err := c.Extensions().Deployments(ns).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	newRS, err := deploymentutil.GetNewReplicaSet(deployment, c.ExtensionsV1beta1())
	if err != nil {
		return err
	}
	for k, v := range expectedAnnotations {
		// Skip checking revision annotations
		if k != deploymentutil.RevisionAnnotation && v != newRS.Annotations[k] {
			return fmt.Errorf("Expected new RS annotations = %+v, got %+v", expectedAnnotations, newRS.Annotations)
		}
	}
	return nil
}

// Delete a ReplicaSet and all pods it spawned
func DeleteReplicaSet(clientset clientset.Interface, internalClientset internalclientset.Interface, ns, name string) error {
	By(fmt.Sprintf("deleting ReplicaSet %s in namespace %s", name, ns))
	rs, err := clientset.Extensions().ReplicaSets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			Logf("ReplicaSet %s was already deleted: %v", name, err)
			return nil
		}
		return err
	}
	startTime := time.Now()
	err = clientset.ExtensionsV1beta1().ReplicaSets(ns).Delete(name, &metav1.DeleteOptions{})
	if apierrs.IsNotFound(err) {
		Logf("ReplicaSet %s was already deleted: %v", name, err)
		return nil
	}
	deleteRSTime := time.Now().Sub(startTime)
	Logf("Deleting RS %s took: %v", name, deleteRSTime)
	if err == nil {
		err = waitForReplicaSetPodsGone(clientset, rs)
	}
	terminatePodTime := time.Now().Sub(startTime) - deleteRSTime
	Logf("Terminating ReplicaSet %s pods took: %v", name, terminatePodTime)
	return err
}

// waitForReplicaSetPodsGone waits until there are no pods reported under a
// ReplicaSet selector (because the pods have completed termination).
func waitForReplicaSetPodsGone(c clientset.Interface, rs *extensions.ReplicaSet) error {
	return wait.PollImmediate(Poll, 2*time.Minute, func() (bool, error) {
		selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		ExpectNoError(err)
		options := metav1.ListOptions{LabelSelector: selector.String()}
		if pods, err := c.Core().Pods(rs.Namespace).List(options); err == nil && len(pods.Items) == 0 {
			return true, nil
		}
		return false, nil
	})
}

// WaitForReadyReplicaSet waits until the replicaset has all of its replicas ready.
func WaitForReadyReplicaSet(c clientset.Interface, ns, name string) error {
	err := wait.Poll(Poll, pollShortTimeout, func() (bool, error) {
		rs, err := c.Extensions().ReplicaSets(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return *(rs.Spec.Replicas) == rs.Status.Replicas && *(rs.Spec.Replicas) == rs.Status.ReadyReplicas, nil
	})
	if err == wait.ErrWaitTimeout {
		err = fmt.Errorf("replicaset %q never became ready", name)
	}
	return err
}

func RunReplicaSet(config testutils.ReplicaSetConfig) error {
	By(fmt.Sprintf("creating replicaset %s in namespace %s", config.Name, config.Namespace))
	config.NodeDumpFunc = DumpNodeDebugInfo
	config.ContainerDumpFunc = LogFailedContainers
	return testutils.RunReplicaSet(config)
}

func NewReplicaSet(name, namespace string, replicas int32, podLabels map[string]string, imageName, image string) *extensions.ReplicaSet {
	return &extensions.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: extensions.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  imageName,
							Image: image,
						},
					},
				},
			},
		},
	}
}

func LogReplicaSet(rs *extensions.ReplicaSet) {
	if rs != nil {
		Logf(spew.Sprintf("Replicaset %q:\n%+v", rs.Name, *rs))
	} else {
		Logf("Replicaset is nil.")
	}
}

// WaitForReplicaSetImage waits for the RS's container image to match the given image.
func WaitForReplicaSetImage(c clientset.Interface, ns, replicaSetName string, image string, pollInterval, pollTimeout time.Duration) error {
	var rs *extensions.ReplicaSet
	var reason string
	err := wait.Poll(pollInterval, pollTimeout, func() (bool, error) {
		var err error
		rs, err = c.Extensions().ReplicaSets(ns).Get(replicaSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if rs == nil {
			reason = fmt.Sprintf("Replicaset %q is yet to be created", rs.Name)
			Logf(reason)
			return false, nil
		}
		if !containsImage(rs.Spec.Template.Spec.Containers, image) {
			reason = fmt.Sprintf("Replicaset %q doesn't have the required image %s.", rs.Name, image)
			Logf(reason)
			return false, nil
		}
		return true, nil
	})
	if err == wait.ErrWaitTimeout {
		LogReplicaSet(rs)
		err = fmt.Errorf(reason)
	}
	if rs == nil {
		return fmt.Errorf("failed to create new replicaset")
	}
	if err != nil {
		return fmt.Errorf("error waiting for replicaset %q (got %s) image to match expectation (expected %s): %v", rs.Name, rs.Spec.Template.Spec.Containers[0].Image, image, err)
	}
	return nil
}

// WaitForObservedReplicaSet polls for replicaset to be updated so that replicaset.Status.ObservedGeneration >= desiredGeneration.
// Return error if polling times out.
func WaitForObservedReplicaSet(c clientset.Interface, ns, replicaSetName string, desiredGeneration int64, interval, timeout time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		rs, err := c.Extensions().ReplicaSets(ns).Get(replicaSetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return rs.Status.ObservedGeneration >= desiredGeneration, nil
	})
}

func containsImage(containers []v1.Container, imageName string) bool {
	for _, container := range containers {
		if container.Image == imageName {
			return true
		}
	}
	return false
}
