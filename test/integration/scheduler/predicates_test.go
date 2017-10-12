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

package scheduler

import (
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/integration/framework"
	testutils "k8s.io/kubernetes/test/utils"
)

// This file tests the scheduler predicates functionality.

var podLabel = map[string]string{"service": "securityscan"}
var podLabel2 = map[string]string{"security": "S1"}

var predicatesTests = []struct {
	pod       *v1.Pod
	pods      []*v1.Pod
	fits      bool
	errorType string
	test      string
}{
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "security",
											Operator: metav1.LabelSelectorOpDoesNotExist,
											Values:   []string{"securityscan"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		fits:      false,
		errorType: "invalidPod",
		test:      "validates that a pod with an invalid podAffinity is rejected because of the LabelSelectorRequirement is invalid",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "security",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		fits: false,
		test: "validates that Inter-pod-Affinity is respected if not matching",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename2",
				Labels: podLabel,
			},
			Spec: v1.PodSpec{
				NodeName: "testnode-0",
			},
		},
		},
		fits: true,
		test: "validates that InterPodAffinity is respected if matching. requiredDuringSchedulingIgnoredDuringExecution in PodAffinity using In operator that matches the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpNotIn,
											Values:   []string{"securityscan3", "value3"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "testnode-0"},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename2",
				Labels: podLabel}}},
		fits: true,
		test: "validates that InterPodAffinity is respected if matching. requiredDuringSchedulingIgnoredDuringExecution in PodAffinity using not in operator in labelSelector that matches the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
								Namespaces:  []string{"diff-namespace"},
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "testnode-0"},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename2",
				Labels: podLabel, Namespace: "ns"}}},
		fits: false,
		test: "validates that inter-pod-affinity is respected when pods have different Namespaces",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"antivirusscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "testnode-0"}, ObjectMeta: metav1.ObjectMeta{
			Name:   "fakename2",
			Labels: podLabel}}},
		fits: false,
		test: "Doesn't satisfy the PodAffinity because of unmatching labelSelector with the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpExists,
										}, {
											Key:      "wrongkey",
											Operator: metav1.LabelSelectorOpDoesNotExist,
										},
									},
								},
								TopologyKey: "region",
							}, {
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan"},
										}, {
											Key:      "service",
											Operator: metav1.LabelSelectorOpNotIn,
											Values:   []string{"WrongValue"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "testnode-0"}, ObjectMeta: metav1.ObjectMeta{
			Name:   "fakename2",
			Labels: podLabel}}},
		fits: true,
		test: "validates that InterPodAffinity is respected if matching with multiple affinities in multiple RequiredDuringSchedulingIgnoredDuringExecution ",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabel2,
				Name:   "fakename",
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpExists,
										}, {
											Key:      "wrongkey",
											Operator: metav1.LabelSelectorOpDoesNotExist,
										},
									},
								},
								TopologyKey: "region",
							}, {
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan2"},
										}, {
											Key:      "service",
											Operator: metav1.LabelSelectorOpNotIn,
											Values:   []string{"WrongValue"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "testnode-0"}, ObjectMeta: metav1.ObjectMeta{
			Name:   "fakename2",
			Labels: podLabel}}},
		fits: false,
		test: "The labelSelector requirements(items of matchExpressions) are ANDed, the pod cannot schedule onto the node because one of the matchExpression items doesn't match.",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
					PodAntiAffinity: &v1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"antivirusscan", "value2"},
										},
									},
								},
								TopologyKey: "node",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "testnode-0"}, ObjectMeta: metav1.ObjectMeta{
			Name:   "fakename2",
			Labels: podLabel}}},
		fits: true,
		test: "validates that InterPod Affinity and AntiAffinity is respected if matching",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
					PodAntiAffinity: &v1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"antivirusscan", "value2"},
										},
									},
								},
								TopologyKey: "node",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{
			{
				Spec: v1.PodSpec{
					NodeName: "testnode-0",
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "service",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"antivirusscan", "value2"},
											},
										},
									},
									TopologyKey: "node",
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "fakename2",
					Labels: podLabel},
			},
		},
		fits: true,
		test: "satisfies the PodAffinity and PodAntiAffinity and PodAntiAffinity symmetry with the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel2,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
					PodAntiAffinity: &v1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "zone",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "testnode-0"}, ObjectMeta: metav1.ObjectMeta{
			Name:   "fakename2",
			Labels: podLabel}}},
		fits: false,
		test: "satisfies the PodAffinity but doesn't satisfies the PodAntiAffinity with the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
					PodAntiAffinity: &v1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"antivirusscan", "value2"},
										},
									},
								},
								TopologyKey: "node",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{
			{
				Spec: v1.PodSpec{
					NodeName: "testnode-0",
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "service",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"securityscan", "value3"},
											},
										},
									},
									TopologyKey: "zone",
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "fakename2",
					Labels: podLabel},
			},
		},
		fits: false,
		test: "satisfies the PodAffinity and PodAntiAffinity but doesn't satisfies PodAntiAffinity symmetry with the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel,
			},
			Spec: v1.PodSpec{
				Affinity: &v1.Affinity{
					PodAffinity: &v1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "service",
											Operator: metav1.LabelSelectorOpNotIn,
											Values:   []string{"securityscan", "value2"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{{Spec: v1.PodSpec{
			NodeName: "machine2"}, ObjectMeta: metav1.ObjectMeta{
			Name:   "fakename2",
			Labels: podLabel}}},
		fits: false,
		test: "pod matches its own Label in PodAffinity and that matches the existing pod Labels",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fakename",
				Labels: podLabel,
			},
			Spec: v1.PodSpec{},
		},
		pods: []*v1.Pod{
			{
				Spec: v1.PodSpec{NodeName: "testnode-0",
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "service",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"securityscan", "value2"},
											},
										},
									},
									TopologyKey: "zone",
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "fakename2",
					Labels: podLabel},
			},
		},
		fits: false,
		test: "Verify that PodAntiAffinity of an existing pod is respected when PodAntiAffinity symmetry is not satisfied with the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "fake-name",
				Labels: podLabel,
			},
			Spec: v1.PodSpec{},
		},
		pods: []*v1.Pod{
			{
				Spec: v1.PodSpec{
					NodeName: "testnode-0",
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "service",
												Operator: metav1.LabelSelectorOpNotIn,
												Values:   []string{"securityscan", "value2"},
											},
										},
									},
									TopologyKey: "zone",
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "fake-name2",
					Labels: podLabel},
			},
		},
		fits: true,
		test: "Verify that PodAntiAffinity from existing pod is respected when pod statisfies PodAntiAffinity symmetry with the existing pod",
	},
	{
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "fake-name2"},
			Spec: v1.PodSpec{
				NodeSelector: map[string]string{"region": "r1"},
				Affinity: &v1.Affinity{
					PodAntiAffinity: &v1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "foo",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"abc"},
										},
									},
								},
								TopologyKey: "region",
							},
						},
					},
				},
			},
		},
		pods: []*v1.Pod{
			{
				Spec: v1.PodSpec{
					NodeName: "testnode-0",
				},
				ObjectMeta: metav1.ObjectMeta{Name: "fakename", Labels: map[string]string{"foo": "abc"}},
			},
		},
		fits: false,
		test: "nodes[0] and nodes[1] have same topologyKey and label value. nodes[0] has an existing pod that matches the inter pod affinity rule. The new pod can not be scheduled onto either of the two nodes.",
	},
}

// TestInterPodAffinity verifies that scheduler's inter pod affinity and
// anti-affinity predicate functions works correctly.
func TestInterPodAffinity(t *testing.T) {
	context := initTest(t, "inter-pod-affinity")
	defer cleanupTest(t, context)

	// Add a few nodes.
	nodes, err := createNodes(context.clientSet, "testnode", nil, 2)
	if err != nil {
		t.Fatalf("Cannot create nodes: %v", err)
	}

	// Add labels to the nodes.
	labels := map[string]string{
		"region": "r1",
		"zone":   "z11",
	}

	cs := context.clientSet

	for _, node := range nodes {
		if err = testutils.AddLabelsToNode(context.clientSet, node.Name, labels); err != nil {
			t.Fatalf("Cannot add labels to node: %v", err)
		}
		if err = waitForNodeLabels(context.clientSet, node.Name, labels); err != nil {
			t.Fatalf("Adding labels to node didn't succeed: %v", err)
		}
	}

	for _, test := range predicatesTests {

		for _, pod := range test.pods {
			nsName := context.ns.Name
			if pod.Namespace != "" {
				nsName = pod.Namespace
			}

			pod.Spec.Containers = []v1.Container{{Name: "container", Image: framework.GetPauseImageName(cs)}}

			createdPod, err := cs.CoreV1().Pods(nsName).Create(pod)
			if err != nil {
				t.Fatalf("Test Failed: error, %v, while creating pod during test: %v", err, test.test)
			}

			err = wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, podScheduled(cs, createdPod.Namespace, createdPod.Name))
			if err != nil {
				t.Errorf("Test Failed: error, %v, while waiting for pod during test, %v", err, test)
			}
		}

		test.pod.Spec.Containers = []v1.Container{{Name: "container", Image: framework.GetPauseImageName(cs)}}

		testPod, err := cs.CoreV1().Pods(context.ns.Name).Create(test.pod)
		if err != nil {
			if !(test.errorType == "invalidPod" && errors.IsInvalid(err)) {
				t.Fatalf("Test Failed: error, %v, while creating pod during test: %v", err, test.test)
			}
			continue
		}

		waitTime := wait.ForeverTestTimeout
		if !test.fits {
			waitTime = time.Second
		}

		err = wait.Poll(100*time.Millisecond, waitTime, podScheduled(cs, testPod.Namespace, testPod.Name))
		if test.fits {
			if err != nil {
				t.Errorf("Test Failed: %v, err %v, test.fits %v", test.test, err, test.fits)
			}
		} else {
			if err != wait.ErrWaitTimeout {
				t.Errorf("Test Failed: error, %v, while waiting for pod to get scheduled, %v", err, test.test)
			}
		}

		for _, pod := range test.pods {
			nsName := context.ns.Name
			if pod.Namespace != "" {
				nsName = pod.Namespace
			}

			err = cs.CoreV1().Pods(nsName).Delete(pod.Name, metav1.NewDeleteOptions(0))
			if err != nil {
				t.Errorf("Test Failed: error, %v, while deleting pod during test: %v", err, test.test)
			}

			err = wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, podDeleted(cs, nsName, pod.Name))
			if err != nil {
				t.Errorf("Test Failed: error, %v, while waiting for pod to get deleted, %v", err, test.test)
			}
		}

		err = cs.CoreV1().Pods(context.ns.Name).Delete(test.pod.Name, metav1.NewDeleteOptions(0))
		if err != nil {
			t.Errorf("Test Failed: error, %v, while deleting pod during test: %v", err, test.test)
		}

		err = wait.Poll(100*time.Millisecond, wait.ForeverTestTimeout, podDeleted(cs, context.ns.Name, test.pod.Name))
		if err != nil {
			t.Errorf("Test Failed: error, %v, while waiting for pod to get deleted, %v", err, test.test)
		}
	}
}
