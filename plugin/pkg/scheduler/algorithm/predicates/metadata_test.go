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

package predicates

import (
	"fmt"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	schedulertesting "k8s.io/kubernetes/plugin/pkg/scheduler/testing"
	"reflect"
	"testing"
)

var label1 = map[string]string{
	"region": "r1",
	"zone":   "z11",
}
var label2 = map[string]string{
	"region": "r1",
	"zone":   "z12",
}
var label3 = map[string]string{
	"region": "r2",
	"zone":   "z21",
}
var label4 = map[string]string{
	"region": "r2",
	"zone":   "z22",
}

func equivalentLists(l1, l2 []interface{}) bool {
	if len(l1) != len(l2) {
		return false
	}
	for _, item1 := range l1 {
		found := false
		for _, item2 := range l2 {
			if item1 == item2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// predicateMetadataEquivalent returns true if the two metadata are equivalent.
// Note: this function does not compare podRequest.
func predicateMetadataEquivalent(meta1, meta2 *predicateMetadata) error {
	if meta1.pod != meta2.pod {
		return fmt.Errorf("pods are not the same.")
	}
	if meta1.podBestEffort != meta2.podBestEffort {
		return fmt.Errorf("podBestEfforts are not equal.")
	}
	if meta1.serviceAffinityInUse != meta1.serviceAffinityInUse {
		return fmt.Errorf("serviceAffinityInUses are not equal.")
	}
	if len(meta1.podPorts) != len(meta2.podPorts) {
		return fmt.Errorf("podPorts are not equal.")
	}
	for k1, v1 := range meta1.podPorts {
		if v2, found := meta2.podPorts[k1]; !found || v1 != v2 {
			return fmt.Errorf("podPorts are not equal.")
		}
	}
	for k1, v1 := range meta1.matchingAntiAffinityTerms {
		var v2 []matchingPodAntiAffinityTerm
		found := false
		if v2, found = meta2.matchingAntiAffinityTerms[k1]; !found {
			return fmt.Errorf("matchingAntiAffinityTerms have different length.")
		}
		for _, term1 := range v1 {
			found := false
			for _, term2 := range v2 {
				if reflect.DeepEqual(term1.term, term2.term) && reflect.DeepEqual(term1.node, term2.node) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("matchingAntiAffinityTerms are not euqal: missing %v", term1.term)
			}
		}
	}
	if meta1.serviceAffinityInUse {
		if len(meta1.serviceAffinityMatchingPodList) != len(meta2.serviceAffinityMatchingPodList) {
			return fmt.Errorf("serviceAffinityMatchingPodLists have different length.")
		}
		for _, pod1 := range meta1.serviceAffinityMatchingPodList {
			found := false
			for _, pod2 := range meta1.serviceAffinityMatchingPodList {
				if reflect.DeepEqual(pod1, pod2) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("serviceAffinityMatchingPodLists are not euqal.")
			}
		}
		for _, service1 := range meta1.serviceAffinityMatchingPodServices {
			found := false
			for _, service2 := range meta1.serviceAffinityMatchingPodServices {
				if service1 == service2 {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("serviceAffinityMatchingPodServices are not euqal.")
			}
		}
	}
	return nil
}

func TestPredicateMetadata_AddRemovePod(t *testing.T) {
	selector1 := map[string]string{"foo": "bar"}
	antiAffinityFooBar := &v1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"bar"},
						},
					},
				},
				TopologyKey: "region",
			},
		},
	}
	antiAffinityComplex := &v1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"bar", "buzz"},
						},
					},
				},
				TopologyKey: "region",
			},
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "service",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"bar", "security", "test"},
						},
					},
				},
				TopologyKey: "zone",
			},
		},
	}

	tests := []struct {
		description  string
		pendingPod   *v1.Pod
		addedPod     *v1.Pod
		existingPods []*v1.Pod
		nodes        []v1.Node
		services     []*v1.Service
	}{
		{
			description: "no anti-affinity or service affinity exist",
			pendingPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "pending", Labels: selector1},
			},
			existingPods: []*v1.Pod{
				{ObjectMeta: metav1.ObjectMeta{UID: "p1", Labels: selector1},
					Spec: v1.PodSpec{NodeName: "nodeA"},
				},
				{ObjectMeta: metav1.ObjectMeta{UID: "p2"},
					Spec: v1.PodSpec{NodeName: "nodeC"},
				},
			},
			addedPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "addedPod", Labels: selector1},
				Spec:       v1.PodSpec{NodeName: "nodeB"},
			},
			nodes: []v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeA", Labels: label1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeB", Labels: label2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeC", Labels: label3}},
			},
		},
		{
			description: "metadata anti-affinity terms are updated correctly after adding and removing a pod",
			pendingPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "pending", Labels: selector1},
			},
			existingPods: []*v1.Pod{
				{ObjectMeta: metav1.ObjectMeta{UID: "p1", Labels: selector1},
					Spec: v1.PodSpec{NodeName: "nodeA"},
				},
				{ObjectMeta: metav1.ObjectMeta{UID: "p2"},
					Spec: v1.PodSpec{
						NodeName: "nodeC",
						Affinity: &v1.Affinity{
							PodAntiAffinity: antiAffinityFooBar,
						},
					},
				},
			},
			addedPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "addedPod", Labels: selector1},
				Spec: v1.PodSpec{
					NodeName: "nodeB",
					Affinity: &v1.Affinity{
						PodAntiAffinity: antiAffinityFooBar,
					},
				},
			},
			nodes: []v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeA", Labels: label1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeB", Labels: label2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeC", Labels: label3}},
			},
		},
		{
			description: "metadata service-affinity data are updated correctly after adding and removing a pod",
			pendingPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "pending", Labels: selector1},
			},
			existingPods: []*v1.Pod{
				{ObjectMeta: metav1.ObjectMeta{UID: "p1", Labels: selector1},
					Spec: v1.PodSpec{NodeName: "nodeA"},
				},
				{ObjectMeta: metav1.ObjectMeta{UID: "p2"},
					Spec: v1.PodSpec{NodeName: "nodeC"},
				},
			},
			addedPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "addedPod", Labels: selector1},
				Spec:       v1.PodSpec{NodeName: "nodeB"},
			},
			services: []*v1.Service{{Spec: v1.ServiceSpec{Selector: selector1}}},
			nodes: []v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeA", Labels: label1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeB", Labels: label2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeC", Labels: label3}},
			},
		},
		{
			description: "metadata anti-affinity terms and service affinity data are updated correctly after adding and removing a pod",
			pendingPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "pending", Labels: selector1},
			},
			existingPods: []*v1.Pod{
				{ObjectMeta: metav1.ObjectMeta{UID: "p1", Labels: selector1},
					Spec: v1.PodSpec{NodeName: "nodeA"},
				},
				{ObjectMeta: metav1.ObjectMeta{UID: "p2"},
					Spec: v1.PodSpec{
						NodeName: "nodeC",
						Affinity: &v1.Affinity{
							PodAntiAffinity: antiAffinityFooBar,
						},
					},
				},
			},
			addedPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "addedPod", Labels: selector1},
				Spec: v1.PodSpec{
					NodeName: "nodeA",
					Affinity: &v1.Affinity{
						PodAntiAffinity: antiAffinityComplex,
					},
				},
			},
			services: []*v1.Service{{Spec: v1.ServiceSpec{Selector: selector1}}},
			nodes: []v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeA", Labels: label1}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeB", Labels: label2}},
				{ObjectMeta: metav1.ObjectMeta{Name: "nodeC", Labels: label3}},
			},
		},
	}

	for _, test := range tests {
		allPodLister := schedulertesting.FakePodLister(append(test.existingPods, test.addedPod))
		// getMeta creates predicate meta data given the list of pods.
		getMeta := func(lister schedulertesting.FakePodLister) (*predicateMetadata, map[string]*schedulercache.NodeInfo) {
			nodeInfoMap := map[string]*schedulercache.NodeInfo{}
			for _, node := range test.nodes {
				var podsOnNode []*v1.Pod
				for _, pod := range lister {
					if pod.Spec.NodeName == node.Name {
						podsOnNode = append(podsOnNode, pod)
					}
				}
				nodeInfo := schedulercache.NewNodeInfo(podsOnNode...)
				nodeInfo.SetNode(&node)
				nodeInfoMap[node.Name] = nodeInfo
			}
			_, precompute := NewServiceAffinityPredicate(lister, schedulertesting.FakeServiceLister(test.services), FakeNodeListInfo(test.nodes), nil)
			RegisterPredicateMetadataProducer("predicateMetadataProducerTesting", precompute)
			pmf := PredicateMetadataFactory{lister}
			meta := pmf.GetMetadata(test.pendingPod, nodeInfoMap)
			return meta.(*predicateMetadata), nodeInfoMap
		}

		// allPodsMeta is meta data produced when all pods, including test.addedPod
		// are give to the metadata producer.
		allPodsMeta, _ := getMeta(allPodLister)
		// existingPodsMeta1 is meta data produced for test.existingPods (without test.addedPod).
		existingPodsMeta1, nodeInfoMap := getMeta(schedulertesting.FakePodLister(test.existingPods))
		// Add test.addedPod to existingPodsMeta1 and make sure meta is equal to allPodsMeta
		nodeInfo := nodeInfoMap[test.addedPod.Spec.NodeName]
		if err := existingPodsMeta1.AddPod(test.addedPod, nodeInfo); err != nil {
			t.Errorf("test [%v]: error adding pod to meta: %v", test.description, err)
		}
		if err := predicateMetadataEquivalent(allPodsMeta, existingPodsMeta1); err != nil {
			t.Errorf("test [%v]: meta data are not equivalent: %v", test.description, err)
		}
		// Remove the added pod and make sure it is still equal to existingPodsMeta
		existingPodsMeta2, _ := getMeta(schedulertesting.FakePodLister(test.existingPods))
		if err := existingPodsMeta1.RemovePod(test.addedPod); err != nil {
			t.Errorf("test [%v]: error removing pod from meta: %v", test.description, err)
		}
		if err := predicateMetadataEquivalent(existingPodsMeta1, existingPodsMeta2); err != nil {
			t.Errorf("test [%v]: meta data are not equivalent: %v", test.description, err)
		}
	}
}
