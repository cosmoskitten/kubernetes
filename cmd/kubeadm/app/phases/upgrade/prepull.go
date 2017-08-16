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
	"time"

	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/images"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
)

const (
	prepullPrefix = "upgrade-prepull-"
)

type prepuller interface {
	CreateFunc(string)
	WaitFunc(string)
	DeleteFunc(string)
}

// DaemonSetPrepuller makes sure the control plane images are availble on all masters
type DaemonSetPrepuller struct {
	client clientset.Interface
	cfg    *kubeadmapi.MasterConfiguration
}

// NewDaemonSetPrepuller creates a new instance of the DaemonSetPrepuller struct
func NewDaemonSetPrepuller(client clientset.Interface, cfg *kubeadmapi.MasterConfiguration) *DaemonSetPrepuller {
	return &DaemonSetPrepuller{
		client: client,
		cfg:    cfg,
	}
}

// CreateFunc creates a DaemonSet for making the image available on every relevant node
func (d *DaemonSetPrepuller) CreateFunc(component string) {
	image := images.GetCoreImage(component, d.cfg.ImageRepository, d.cfg.KubernetesVersion, d.cfg.UnifiedControlPlaneImage)
	ds := buildPrePullDaemonSet(component, image)

	// Create the DaemonSet in the API Server
	if _, err := d.client.ExtensionsV1beta1().DaemonSets(metav1.NamespaceSystem).Create(ds); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			fmt.Printf("[upgrade/prepull] WARNING: Failed to create daemonset for prepulling component %q: %v", component, err)
			return
		}

		if _, err := d.client.ExtensionsV1beta1().DaemonSets(metav1.NamespaceSystem).Update(ds); err != nil {
			// TODO: We should retry on 409 responses
			fmt.Printf("[upgrade/prepull] WARNING: Failed to update daemonset for prepulling component %q: %v", component, err)
		}
	}
}

// WaitFunc waits for all Pods in the specified DaemonSet to be in the Running state
func (d *DaemonSetPrepuller) WaitFunc(component string) {
	fmt.Printf("[upgrade/prepull] Prepulling image for component %s.\n", component)
	apiclient.WaitForPodsWithLabel(d.client, 0, ioutil.Discard, "k8s-app=upgrade-prepull-"+component)
}

// DeleteFunc deletes the DaemonSet used for making the image available on every relevant node
func (d *DaemonSetPrepuller) DeleteFunc(component string) {
	dsName := addPrepullPrefix(component)
	foregroundDelete := metav1.DeletePropagationForeground
	deleteOptions := &metav1.DeleteOptions{
		PropagationPolicy: &foregroundDelete,
	}
	if err := d.client.ExtensionsV1beta1().DaemonSets(metav1.NamespaceSystem).Delete(dsName, deleteOptions); err != nil {
		fmt.Printf("[upgrade/prepull] WARNING: Unable to cleanup prepull DaemonSet %s\n", component)
	}
	fmt.Printf("[upgrade/prepull] Prepulled image for component %s.\n", component)
}

// PrepullImagesInParallel creates DaemonSets synchronously but waits in parallell for the images to pull
func PrepullImagesInParallel(kubePrepuller prepuller, timeout time.Duration) error {
	componentsToPrepull := constants.MasterComponents
	fmt.Printf("[upgrade/prepull] Will prepull images for components %v\n", componentsToPrepull)

	timeoutChan := time.After(timeout)

	// Synchronously create the DaemonSets
	for _, component := range componentsToPrepull {
		kubePrepuller.CreateFunc(component)
	}

	// Create a channel for streaming data from goroutines that run in parallell to a blocking for loop that cleans up
	prePulledChan := make(chan string, len(componentsToPrepull))
	for _, component := range componentsToPrepull {
		go func(c string) {
			// Wait as long as needed. This WaitFunc call should be blocking until completetion
			kubePrepuller.WaitFunc(c)
			// When the task is done, go ahead and cleanup by sending the name to the channel
			prePulledChan <- c
		}(component)
	}

	// This call blocks until all expected messages are received from the channel or errors out if timeoutChan fires.
	// For every successful wait, kubePrepuller.DeleteFunc is executed
	if err := waitForItemsFromChan(timeoutChan, prePulledChan, len(componentsToPrepull), kubePrepuller.DeleteFunc); err != nil {
		return err
	}

	fmt.Println("[upgrade/prepull] Successfully prepulled the images for all the control plane components")
	return nil
}

// waitForItemsFromChan waits for n elements from stringChan with a timeout. For every item received from stringChan, cleanupFunc is executed
func waitForItemsFromChan(timeoutChan <-chan time.Time, stringChan chan string, n int, cleanupFunc func(string)) error {
	i := 0
	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("The prepull operation timed out")
		case result := <-stringChan:
			i++
			cleanupFunc(result)
			if i == n {
				return nil
			}
		}
	}
	return nil
}

// addPrepullPrefix adds the prepull prefix for this functionality; can be used in names, labels, etc.
func addPrepullPrefix(component string) string {
	return fmt.Sprintf("%s%s", prepullPrefix, component)
}

// buildPrePullDaemonSet builds the DaemonSet that ensures the control plane image is available
func buildPrePullDaemonSet(component, image string) *extensions.DaemonSet {
	var gracePeriodSecs int64 = 0
	return &extensions.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: addPrepullPrefix(component)},
		Spec: extensions.DaemonSetSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": addPrepullPrefix(component),
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    component,
							Image:   image,
							Command: []string{"sleep", "3600"},
						},
					},
					NodeSelector: map[string]string{
						constants.LabelNodeRoleMaster: "",
					},
					Tolerations:                   []v1.Toleration{constants.MasterToleration},
					TerminationGracePeriodSeconds: &gracePeriodSecs,
				},
			},
		},
	}
}
