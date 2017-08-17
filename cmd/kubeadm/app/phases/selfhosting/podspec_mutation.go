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

package selfhosting

import (
	"path/filepath"
	"strings"

	"k8s.io/api/core/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
)

const (
	selfHostedKubeConfigDir = "/etc/kubernetes/kubeconfig"
)

// getDefaultMutators gets the mutator functions that alwasy should be used
func getDefaultMutators() map[string][]func(*v1.PodSpec) {
	return map[string][]func(*v1.PodSpec){
		kubeadmconstants.KubeAPIServer: {
			addNodeSelectorToPodSpec,
			setMasterTolerationOnPodSpec,
			setRightDNSPolicyOnPodSpec,
		},
		kubeadmconstants.KubeControllerManager: {
			addNodeSelectorToPodSpec,
			setMasterTolerationOnPodSpec,
			setRightDNSPolicyOnPodSpec,
		},
		kubeadmconstants.KubeScheduler: {
			addNodeSelectorToPodSpec,
			setMasterTolerationOnPodSpec,
			setRightDNSPolicyOnPodSpec,
		},
	}
}

// mutatePodSpec makes a Static Pod-hosted PodSpec suitable for self-hosting
func mutatePodSpec(mutators map[string][]func(*v1.PodSpec), name string, podSpec *v1.PodSpec) {
	// Get the mutator functions for the component in question, then loop through and execute them
	mutatorsForComponent := mutators[name]
	for _, mutateFunc := range mutatorsForComponent {
		mutateFunc(podSpec)
	}
}

// addNodeSelectorToPodSpec makes Pod require to be scheduled on a node marked with the master label
func addNodeSelectorToPodSpec(podSpec *v1.PodSpec) {
	if podSpec.NodeSelector == nil {
		podSpec.NodeSelector = map[string]string{kubeadmconstants.LabelNodeRoleMaster: ""}
		return
	}

	podSpec.NodeSelector[kubeadmconstants.LabelNodeRoleMaster] = ""
}

// setMasterTolerationOnPodSpec makes the Pod tolerate the master taint
func setMasterTolerationOnPodSpec(podSpec *v1.PodSpec) {
	if podSpec.Tolerations == nil {
		podSpec.Tolerations = []v1.Toleration{kubeadmconstants.MasterToleration}
		return
	}

	podSpec.Tolerations = append(podSpec.Tolerations, kubeadmconstants.MasterToleration)
}

// setRightDNSPolicyOnPodSpec makes sure the self-hosted components can look up things via kube-dns if necessary
func setRightDNSPolicyOnPodSpec(podSpec *v1.PodSpec) {
	podSpec.DNSPolicy = v1.DNSClusterFirstWithHostNet
}

// useSelfHostedVolumesForAPIServer makes sure the self-hosted api server has the right volume source coming from a self-hosted cluster
func useSelfHostedVolumesForAPIServer(podSpec *v1.PodSpec) {
	for i, v := range podSpec.Volumes {
		// If the volume name matches the expected one; switch the volume source from hostPath to cluster-hosted
		if v.Name == kubeadmconstants.KubeCertificatesVolumeName {
			podSpec.Volumes[i].VolumeSource = apiServerCertificatesVolumeSource()
		}
	}
}

// useSelfHostedVolumesForControllerManager makes sure the self-hosted controller manager has the right volume source coming from a self-hosted cluster
func useSelfHostedVolumesForControllerManager(podSpec *v1.PodSpec) {
	for i, v := range podSpec.Volumes {
		// If the volume name matches the expected one; switch the volume source from hostPath to cluster-hosted
		if v.Name == kubeadmconstants.KubeCertificatesVolumeName {
			podSpec.Volumes[i].VolumeSource = controllerManagerCertificatesVolumeSource()
		} else if v.Name == kubeadmconstants.KubeConfigVolumeName {
			podSpec.Volumes[i].VolumeSource = kubeConfigVolumeSource(kubeadmconstants.ControllerManagerKubeConfigFileName)
		}
	}

	// Change directory for the kubeconfig directory to selfHostedKubeConfigDir
	for i, vm := range podSpec.Containers[0].VolumeMounts {
		if vm.Name == kubeadmconstants.KubeConfigVolumeName {
			podSpec.Containers[0].VolumeMounts[i].MountPath = selfHostedKubeConfigDir
		}
	}

	podSpec.Containers[0].Command = replaceArgument(podSpec.Containers[0].Command, func(argMap map[string]string) map[string]string {
		argMap["kubeconfig"] = filepath.Join(selfHostedKubeConfigDir, kubeadmconstants.ControllerManagerKubeConfigFileName)
		return argMap
	})
}

// useSelfHostedVolumesForScheduler makes sure the self-hosted scheduler has the right volume source coming from a self-hosted cluster
func useSelfHostedVolumesForScheduler(podSpec *v1.PodSpec) {
	for i, v := range podSpec.Volumes {
		// If the volume name matches the expected one; switch the volume source from hostPath to cluster-hosted
		if v.Name == kubeadmconstants.KubeConfigVolumeName {
			podSpec.Volumes[i].VolumeSource = kubeConfigVolumeSource(kubeadmconstants.SchedulerKubeConfigFileName)
		}
	}

	// Change directory for the kubeconfig directory to selfHostedKubeConfigDir
	for i, vm := range podSpec.Containers[0].VolumeMounts {
		if vm.Name == kubeadmconstants.KubeConfigVolumeName {
			podSpec.Containers[0].VolumeMounts[i].MountPath = selfHostedKubeConfigDir
		}
	}

	podSpec.Containers[0].Command = replaceArgument(podSpec.Containers[0].Command, func(argMap map[string]string) map[string]string {
		argMap["kubeconfig"] = filepath.Join(selfHostedKubeConfigDir, kubeadmconstants.SchedulerKubeConfigFileName)
		return argMap
	})
}

func replaceArgument(command []string, argMutateFunc func(map[string]string) map[string]string) []string {
	argMap := kubeadmutil.ParseArgumentListToMap(command)

	// Save the first command (the executable) if we're sure it's not an argument (i.e. no --)
	var newCommand []string
	if len(command) > 0 && !strings.HasPrefix(command[0], "--") {
		newCommand = append(newCommand, command[0])
	}
	newArgMap := argMutateFunc(argMap)
	newCommand = append(newCommand, kubeadmutil.BuildArgumentListFromMap(newArgMap, map[string]string{})...)
	return newCommand
}
