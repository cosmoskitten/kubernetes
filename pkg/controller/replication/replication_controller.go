/*
Copyright 2014 The Kubernetes Authors.

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

// If you make changes to this file, you should also make the corresponding change in ReplicaSet.

package replication

import (
	"k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/controller/replicaset"
)

const (
	BurstReplicas = replicaset.BurstReplicas
)

// ReplicationManager is responsible for synchronizing ReplicationController objects stored
// in the system with actual running pods.
// It is actually just a wrapper around ReplicaSetController.
type ReplicationManager struct {
	replicaset.ReplicaSetController
}

// NewReplicationManager configures a replication manager with the specified event recorder
func NewReplicationManager(podInformer coreinformers.PodInformer, rcInformer coreinformers.ReplicationControllerInformer, kubeClient clientset.Interface, burstReplicas int) *ReplicationManager {
	return &ReplicationManager{
		*replicaset.NewBaseController(informerAdapter{rcInformer}, podInformer, clientsetAdapter{kubeClient}, burstReplicas,
			v1.SchemeGroupVersion.WithKind("ReplicationController"),
			"replication_controller",
			"replication-controller",
			"replicationmanager",
			"RC",
		),
	}
}
