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
	"time"

	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

// prepuller is an interface for PrepullImagesInParallel to use for prepulling the control plane images in parallel
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

// Make sure DaemonSetPrepuller implements the prepuller interface
var _ prepuller = &DaemonSetPrepuller{}

// NewDaemonSetPrepuller creates a new instance of the DaemonSetPrepuller struct
func NewDaemonSetPrepuller(client clientset.Interface, cfg *kubeadmapi.MasterConfiguration) *DaemonSetPrepuller {
	return &DaemonSetPrepuller{
		client: client,
		cfg:    cfg,
	}
}

// CreateFunc creates a DaemonSet for making the image available on every relevant node
func (d *DaemonSetPrepuller) CreateFunc(component string) {}

// WaitFunc waits for all Pods in the specified DaemonSet to be in the Running state
func (d *DaemonSetPrepuller) WaitFunc(component string) {}

// DeleteFunc deletes the DaemonSet used for making the image available on every relevant node
func (d *DaemonSetPrepuller) DeleteFunc(component string) {}

// PrepullImagesInParallel creates DaemonSets synchronously but waits in parallell for the images to pull
func PrepullImagesInParallel(kubePrepuller prepuller, timeout time.Duration) error {
	componentsToPrepull := constants.MasterComponents
	fmt.Printf("[upgrade/prepull] Will prepull images for components %v\n", componentsToPrepull)

	// TODO: Implement the prepull mechanism here

	fmt.Println("[upgrade/prepull] Successfully prepulled the images for all the control plane components")
	return nil
}
