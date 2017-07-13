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

package kubectl

import (
	"fmt"
	"strings"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// BaseDeploymentGenerator: implement the common functionality of
// DeploymentBasicGeneratorV1 and DeploymentBasicAppsGeneratorV1 (both of the
// 'kubectl create deployment' Generators). To reduce confusion, it's best to
// keep this struct in the same file as those generators.
type BaseDeploymentGenerator struct {
	Name   string
	Images []string

	// This is an optional parameter and can be left blank.
	// NOTE: even when this parameter is blank, the "app" label will be set
	// on the resulting deployment.
	Labels string

	// Replicas is not optional in this struct but typically it defaults to
	// 1 in the command system.
	// Determines the number of replicas on the deployment.
	Replicas int32

	// Env is the environment variable mapping to be set in the deployed
	// container. Use environment variables to configure your application.
	// Example: []string{"TARGET=alderaan", "DEATH_STAR_DEBUG_LEVEL=4"}
	// You can leave this nil if you like.
	Env []string

	// Limits and Requests are strings like "cpu=200m,memory=512Mi".
	Limits   string
	Requests string
}

// ParamNames: return the parameters expected by the BaseDeploymentGenerator.
// This method is here to aid in validation. When given a Generator, you can
// learn what it expects by calling this method.
func (BaseDeploymentGenerator) ParamNames() []GeneratorParam {
	return []GeneratorParam{
		{"name", true},
		{"image", true},
		{"labels", false},
		{"replicas", false},
		{"env", false},
		{"limits", false},
		{"requests", false},
	}
}

// validate: check if the caller has forgotten to set one of our fields.
// We don't bother to check if the optional fields have been set. Do not add
// validation to the optional fields. Those problems will be caught at a lower
// level and bubbled up. baseDeploymentGenerator is just a way to get parameters
// into a Generator.
func (b BaseDeploymentGenerator) validate() error {
	if len(b.Name) == 0 {
		return fmt.Errorf("name must be specified")
	}
	if len(b.Images) == 0 {
		return fmt.Errorf("at least one image must be specified")
	}
	return nil
}

// baseDeploymentGeneratorFromParams: return a new BaseDeploymentGenerator with
// the fields set from params. The returned BaseDeploymentGenerator should have
// all required fields set and will pass validate() with no errors.
func baseDeploymentGeneratorFromParams(params map[string]interface{}) (*BaseDeploymentGenerator, error) {
	paramNames := (BaseDeploymentGenerator{}).ParamNames()
	err := ValidateParams(paramNames, params)
	if err != nil {
		return nil, err
	}
	name, isString := params["name"].(string)
	if !isString {
		return nil, fmt.Errorf("expected string, saw %#v for 'name'", params["name"])
	}
	imageStrings, isArray := params["image"].([]string)
	if params["image"] != nil && !isArray {
		return nil, fmt.Errorf("expected []string, saw %#v for 'image'", params["image"])
	}
	labels, isString := params["labels"].(string)
	if params["labels"] != nil && !isString {
		return nil, fmt.Errorf("expected string, saw %#v for 'labels'", params["labels"])
	}
	replicas, isInt := params["replicas"].(int)
	if params["replicas"] == nil {
		// default to 1 replica
		replicas = 1
	} else if !isInt {
		return nil, fmt.Errorf("expected int, saw %#v for 'replicas'", params["replicas"])
	}
	replicasInt32 := int32(replicas)
	env, isArray := params["env"].([]string)
	if params["env"] != nil && !isArray {
		return nil, fmt.Errorf("expected []string, saw %#v for 'env'", params["env"])
	}
	limits, isString := params["limits"].(string)
	if params["limits"] != nil && !isString {
		return nil, fmt.Errorf("expected string, saw %#v for 'limits'", params["limits"])
	}
	requests, isString := params["requests"].(string)
	if params["requests"] != nil && !isString {
		return nil, fmt.Errorf("expected string, saw %#v for 'requests'", params["requests"])
	}
	return &BaseDeploymentGenerator{
		Name:     name,
		Images:   imageStrings,
		Labels:   labels,
		Replicas: replicasInt32,
		Env:      env,
		Limits:   limits,
		Requests: requests,
	}, nil
}

// structuredGenerate: determine the fields of a deployment. The struct that
// embeds BaseDeploymentGenerator should assemble these pieces into a
// runtime.Object.
func (b BaseDeploymentGenerator) structuredGenerate() (
	podSpec v1.PodSpec,
	labels map[string]string,
	selector metav1.LabelSelector,
	replicas int32,
	err error,
) {
	err = b.validate()
	if err != nil {
		return
	}
	envVars, err := parseEnvs(b.Env)
	if err != nil {
		return
	}
	limits, err := populateResourceListV1(b.Limits)
	if err != nil {
		return
	}
	requests, err := populateResourceListV1(b.Requests)
	if err != nil {
		return
	}
	resourceRequirements := v1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
	podSpec = buildPodSpec(b.Images, envVars, resourceRequirements)

	// Load labels from the parameters if any Labels were provided.
	// Labels are optional.
	labels = map[string]string{}
	if len(b.Labels) > 0 {
		labels, err = ParseLabels(b.Labels)
		if err != nil {
			return
		}
	}

	labels["app"] = b.Name
	selector = metav1.LabelSelector{MatchLabels: labels}
	replicas = b.Replicas
	return
}

// buildPodSpec: parse the image strings and assemble them into the Containers
// of a PodSpec. This is all you need to create the PodSpec for a deployment.
func buildPodSpec(images []string, env []v1.EnvVar, resourceRequirements v1.ResourceRequirements) v1.PodSpec {
	podSpec := v1.PodSpec{Containers: []v1.Container{}}
	for _, imageString := range images {
		// Retain just the image name
		imageSplit := strings.Split(imageString, "/")
		name := imageSplit[len(imageSplit)-1]
		// Remove any tag or hash
		if strings.Contains(name, ":") {
			name = strings.Split(name, ":")[0]
		} else if strings.Contains(name, "@") {
			name = strings.Split(name, "@")[0]
		}
		podSpec.Containers = append(podSpec.Containers, v1.Container{
			Name:      name,
			Image:     imageString,
			Env:       env,
			Resources: resourceRequirements,
		})
	}
	return podSpec
}

// DeploymentBasicGeneratorV1 supports stable generation of a deployment
type DeploymentBasicGeneratorV1 struct {
	BaseDeploymentGenerator
}

// Ensure it supports the generator pattern that uses parameters specified during construction
var _ StructuredGenerator = &DeploymentBasicGeneratorV1{}

func (s DeploymentBasicGeneratorV1) Generate(params map[string]interface{}) (runtime.Object, error) {
	base, err := baseDeploymentGeneratorFromParams(params)
	if err != nil {
		return nil, err
	}
	return (&DeploymentBasicGeneratorV1{*base}).StructuredGenerate()
}

// StructuredGenerate outputs a deployment object using the configured fields
func (s *DeploymentBasicGeneratorV1) StructuredGenerate() (runtime.Object, error) {
	podSpec, labels, selector, replicas, err := s.structuredGenerate()
	return &extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   s.Name,
			Labels: labels,
		},
		Spec: extensionsv1beta1.DeploymentSpec{
			Replicas: &replicas,
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

// DeploymentBasicAppsGeneratorV1 supports stable generation of a deployment under apps/v1beta1 endpoint
type DeploymentBasicAppsGeneratorV1 struct {
	BaseDeploymentGenerator
}

// Ensure it supports the generator pattern that uses parameters specified during construction
var _ StructuredGenerator = &DeploymentBasicAppsGeneratorV1{}

func (s DeploymentBasicAppsGeneratorV1) Generate(params map[string]interface{}) (runtime.Object, error) {
	base, err := baseDeploymentGeneratorFromParams(params)
	if err != nil {
		return nil, err
	}
	return (&DeploymentBasicAppsGeneratorV1{*base}).StructuredGenerate()
}

// StructuredGenerate outputs a deployment object using the configured fields
func (s *DeploymentBasicAppsGeneratorV1) StructuredGenerate() (runtime.Object, error) {
	podSpec, labels, selector, replicas, err := s.structuredGenerate()
	return &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   s.Name,
			Labels: labels,
		},
		Spec: appsv1beta1.DeploymentSpec{
			Replicas: &replicas,
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
