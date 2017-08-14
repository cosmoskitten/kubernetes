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

package apiclient

import (
	"fmt"
	"net"
	"strings"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	core "k8s.io/client-go/testing"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/pkg/registry/core/service/ipallocator"
)

// InitDryRunGetter
// Need to handle these routes in a special manner:
// - GET /default/services/kubernetes -- must return a valid Service
// - GET /clusterrolebindings/system:nodes -- can safely return a NotFound error
// - GET /kube-system/secrets/bootstrap-token-* -- can safely return a NotFound error
// - GET /nodes/<node-name> -- must return a valid Node
// - ...all other, unknown GETs/LISTs will be logged
type InitDryRunGetter struct {
	masterName    string
	serviceSubnet string
}

func NewInitDryRunGetter(masterName string, serviceSubnet string) *InitDryRunGetter {
	return &InitDryRunGetter{
		masterName:    masterName,
		serviceSubnet: serviceSubnet,
	}
}

func (idr *InitDryRunGetter) HandleGetAction(action core.GetAction) (bool, runtime.Object, error) {
	funcs := []func(core.GetAction) (bool, runtime.Object, error){
		idr.handleKubernetesService,
		idr.handleGetNode,
		idr.handleSystemNodesClusterRoleBinding,
		idr.handleGetBootstrapToken,
	}
	for _, f := range funcs {
		handled, obj, err := f(action)
		if handled {
			return handled, obj, err
		}
	}

	return false, nil, nil
}

// HandleListAction handles known LIST actions for init. Currently, there aren't any
func (idr *InitDryRunGetter) HandleListAction(action core.ListAction) (bool, runtime.Object, error) {
	return false, nil, nil
}

// handleKubernetesService
// The kube-dns addon code GETs the kubernetes service in order to extract the service subnet
func (idr *InitDryRunGetter) handleKubernetesService(action core.GetAction) (bool, runtime.Object, error) {
	if action.GetName() != "kubernetes" || action.GetResource().Resource != "services" {
		// We can't handle this event
		return false, nil, nil
	}

	_, svcSubnet, err := net.ParseCIDR(idr.serviceSubnet)
	if err != nil {
		return true, nil, fmt.Errorf("error parsing CIDR %q: %v", idr.serviceSubnet, err)
	}

	internalAPIServerVirtualIP, err := ipallocator.GetIndexedIP(svcSubnet, 1)
	if err != nil {
		return true, nil, fmt.Errorf("unable to get first IP address from the given CIDR (%s): %v", svcSubnet.String(), err)
	}

	// We can safely return a NotFound error here as the code will just proceed normally and don't care about modifying this clusterrolebinding
	// This can only happen on an upgrade
	return true, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				"component": "apiserver",
				"provider":  "kubernetes",
			},
		},
		Spec: v1.ServiceSpec{
			ClusterIP: internalAPIServerVirtualIP.String(),
			Ports: []v1.ServicePort{
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstr.FromInt(6443),
				},
			},
		},
	}, nil
}

// handleGetNode
func (idr *InitDryRunGetter) handleGetNode(action core.GetAction) (bool, runtime.Object, error) {
	if action.GetResource().Resource != "nodes" {
		// We can't handle this event
		return false, nil, nil
	}
	// We can safely return a NotFound error here as the code will just proceed normally and don't care about modifying this clusterrolebinding
	// This can only happen on an upgrade; and in that case the ClientBackedDryRunGetter impl will be used
	return true, &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: idr.masterName,
			Labels: map[string]string{
				"kubernetes.io/hostname": idr.masterName,
			},
		},
		Spec: v1.NodeSpec{
			ExternalID: idr.masterName,
		},
	}, nil
}

// handleSystemNodesClusterRoleBinding
func (idr *InitDryRunGetter) handleSystemNodesClusterRoleBinding(action core.GetAction) (bool, runtime.Object, error) {
	if action.GetName() != constants.NodesClusterRoleBinding || action.GetResource().Resource != "clusterrolebindings" {
		// We can't handle this event
		return false, nil, nil
	}
	// We can safely return a NotFound error here as the code will just proceed normally and don't care about modifying this clusterrolebinding
	// This can only happen on an upgrade; and in that case the ClientBackedDryRunGetter impl will be used
	return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "clusterrolebinding not found")
}

// handleGetBootstrapToken
func (idr *InitDryRunGetter) handleGetBootstrapToken(action core.GetAction) (bool, runtime.Object, error) {
	if strings.HasPrefix(action.GetName(), "bootstrap-token-") || action.GetResource().Resource != "secrets" {
		// We can't handle this event
		return false, nil, nil
	}
	// We can safely return a NotFound error here as the code will just proceed normally and don't care about modifying this clusterrolebinding
	// This can only happen on an upgrade; and in that case the ClientBackedDryRunGetter impl will be used
	return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "secret not found")
}
