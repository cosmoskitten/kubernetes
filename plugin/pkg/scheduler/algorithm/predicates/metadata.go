/*
Copyright 2016 The Kubernetes Authors.

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

package predicates

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	schedutil "k8s.io/kubernetes/plugin/pkg/scheduler/util"
)

type PredicateMetadataFactory struct {
	podLister algorithm.PodLister
}

//  Note that predicateMetadata and matchingPodAntiAffinityTerm need to be declared in the same file
//  due to the way declarations are processed in predicate declaration unit tests.
type matchingPodAntiAffinityTerm struct {
	term *v1.PodAffinityTerm
	node *v1.Node
}

// NOTE: When new fields are added/removed or logic is changed, please make sure
// that RemovePod and AddPod functions are updated to work with the new changes.
type predicateMetadata struct {
	pod                                *v1.Pod
	podBestEffort                      bool
	podRequest                         *schedulercache.Resource
	podPorts                           map[int]bool
	matchingAntiAffinityTerms          map[types.UID][]matchingPodAntiAffinityTerm //key is a pod UID with the anti-affinity rule.
	serviceAffinityInUse               bool
	serviceAffinityMatchingPodList     []*v1.Pod
	serviceAffinityMatchingPodServices []*v1.Service
}

// PredicateMetadataProducer: Helper types/variables...
type PredicateMetadataProducer func(pm *predicateMetadata)

var predicateMetaProducerRegisterLock sync.Mutex
var predicateMetadataProducers map[string]PredicateMetadataProducer = make(map[string]PredicateMetadataProducer)

func RegisterPredicateMetadataProducer(predicateName string, precomp PredicateMetadataProducer) {
	predicateMetaProducerRegisterLock.Lock()
	defer predicateMetaProducerRegisterLock.Unlock()
	predicateMetadataProducers[predicateName] = precomp
}

func NewPredicateMetadataFactory(podLister algorithm.PodLister) algorithm.MetadataProducer {
	factory := &PredicateMetadataFactory{
		podLister,
	}
	return factory.GetMetadata
}

// GetMetadata returns the predicateMetadata used which will be used by various predicates.
func (pfactory *PredicateMetadataFactory) GetMetadata(pod *v1.Pod, nodeNameToInfoMap map[string]*schedulercache.NodeInfo) interface{} {
	// If we cannot compute metadata, just return nil
	if pod == nil {
		return nil
	}
	matchingTerms, err := getMatchingAntiAffinityTerms(pod, nodeNameToInfoMap)
	if err != nil {
		return nil
	}
	predicateMetadata := &predicateMetadata{
		pod:                       pod,
		podBestEffort:             isPodBestEffort(pod),
		podRequest:                GetResourceRequest(pod),
		podPorts:                  schedutil.GetUsedPorts(pod),
		matchingAntiAffinityTerms: matchingTerms,
	}
	for predicateName, precomputeFunc := range predicatePrecomputations {
		glog.V(10).Infof("Precompute: %v", predicateName)
		precomputeFunc(predicateMetadata)
	}
	return predicateMetadata
}

// RemovePod changes predicateMetadata assuming that the given `deletedPod` is
// deleted from the system.
func (meta *predicateMetadata) RemovePod(deletedPod *v1.Pod) error {
	if deletedPod.GetUID() == meta.pod.GetUID() {
		return fmt.Errorf("deletedPod and meta.pod must not be the same.")
	}
	// Delete any anti-affinity rule from the deletedPod.
	delete(meta.matchingAntiAffinityTerms, deletedPod.GetUID())
	// All pods in the serviceAffinityMatchingPodList are in the same namespace.
	// So, if the namespace of the first one is not the same as the namespace of the
	// deletedPod, we don't need to check the list, as deletedPod isn't in the list.
	if meta.serviceAffinityInUse &&
		len(meta.serviceAffinityMatchingPodList) > 0 &&
		deletedPod.Namespace == meta.serviceAffinityMatchingPodList[0].Namespace {
		deletedPodIndex := -1
		for i, pod := range meta.serviceAffinityMatchingPodList {
			if pod.GetUID() == deletedPod.GetUID() {
				deletedPodIndex = i
				break
			}
		}
		if deletedPodIndex >= 0 {
			meta.serviceAffinityMatchingPodList = append(meta.serviceAffinityMatchingPodList[:deletedPodIndex], meta.serviceAffinityMatchingPodList[deletedPodIndex+1:]...)
		}
	}
	return nil
}

// AddPod changes predicateMetadata assuming that `newPod` is added to the
// system.
func (meta *predicateMetadata) AddPod(addedPod *v1.Pod, nodeInfo *schedulercache.NodeInfo) error {
	addedPodUID := addedPod.GetUID()
	if addedPodUID == meta.pod.GetUID() {
		return fmt.Errorf("addedPod and meta.pod must not be the same.")
	}
	if nodeInfo.Node() == nil {
		return fmt.Errorf("Invalid node in nodeInfo.")
	}
	// Add matching anti-affinity terms of the addedPod to the map.
	if podMatchingTerms, err := getMatchingAntiAffinityTermsOfExistingPod(meta.pod, addedPod, nodeInfo.Node()); err == nil {
		if len(podMatchingTerms) > 0 {
			meta.matchingAntiAffinityTerms[addedPodUID] = append(meta.matchingAntiAffinityTerms[addedPodUID], podMatchingTerms...)
		}
	} else {
		return err
	}
	// If addedPod is in the same namespace as the meta.pod, update the the list
	// of matching pods if applicable.
	if meta.serviceAffinityInUse && addedPod.Namespace == meta.pod.Namespace {
		selector := CreateSelectorFromLabels(meta.pod.Labels)
		if selector.Matches(labels.Set(addedPod.Labels)) {
			meta.serviceAffinityMatchingPodList = append(meta.serviceAffinityMatchingPodList, addedPod)
		}
	}
	return nil
}
