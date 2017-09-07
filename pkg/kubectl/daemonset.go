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

package kubectl

import (
	"fmt"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// BaseGenerator: implement the common functionality.
type BaseGenerator struct {
	Name   string
	Images []string
}

// validate: check if the caller has forgotten to set one of our fields.
func (b BaseGenerator) validate() error {
	if len(b.Name) == 0 {
		return fmt.Errorf("name must be specified")
	}
	if len(b.Images) == 0 {
		return fmt.Errorf("at least one image must be specified")
	}
	return nil
}

// structuredGenerate: determine the fields of a controller. The struct that
// embeds Generator should assemble these pieces into a
// runtime.Object.
func (b BaseGenerator) structuredGenerate() (
	podSpec v1.PodSpec,
	labels map[string]string,
	selector metav1.LabelSelector,
	err error,
) {
	err = b.validate()
	if err != nil {
		return
	}
	podSpec = buildPodSpec(b.Images)
	labels = map[string]string{}
	labels["app"] = b.Name
	selector = metav1.LabelSelector{MatchLabels: labels}
	return
}

// DaemonSetGeneratorV1Beta1 supports stable generation of a daemonset
type DaemonSetGeneratorV1Beta1 struct {
	BaseGenerator
}

// Ensure it supports the generator pattern that uses parameters specified during construction
var _ StructuredGenerator = &DaemonSetGeneratorV1Beta1{}

// StructuredGenerate outputs a daemonset object using the configured fields
func (s *DaemonSetGeneratorV1Beta1) StructuredGenerate() (runtime.Object, error) {
	podSpec, labels, selector, err := s.structuredGenerate()
	return &extensionsv1beta1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   s.Name,
			Labels: labels,
		},
		Spec: extensionsv1beta1.DaemonSetSpec{
			Selector: &selector,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}, err
}

// DaemonSetGeneratorV1Beta2 supports stable generation of a daemonset
type DaemonSetGeneratorV1Beta2 struct {
	BaseGenerator
}

// Ensure it supports the generator pattern that uses parameters specified during construction
var _ StructuredGenerator = &DaemonSetGeneratorV1Beta2{}

// StructuredGenerate outputs a daemonset object using the configured fields
func (s *DaemonSetGeneratorV1Beta2) StructuredGenerate() (runtime.Object, error) {
	podSpec, labels, selector, err := s.structuredGenerate()
	return &appsv1beta2.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   s.Name,
			Labels: labels,
		},
		Spec: appsv1beta2.DaemonSetSpec{
			Selector: &selector,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}, err
}
