/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package util

import (
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
)

// ExampleFetcher fetches examples and templates for Kubernetes
// API object kinds.
type ExampleFetcher interface {
	// NewExample fetches a new copy of an example object of
	// the given version and kind, prepopulated with common values.
	// If no example is found, it should return false for the boolean,
	// but still return an emtpy object of the correct version and kind.
	NewExample(version, kind string) (runtime.Object, bool, error)
}

// staticExampleFetcher returns examples based on a static list of examples
type staticExampleFetcher struct {
	scheme   *runtime.Scheme
	examples map[string]map[string]runtime.Object
}

var _ ExampleFetcher = &staticExampleFetcher{}

func (f *staticExampleFetcher) NewExample(version, kind string) (runtime.Object, bool, error) {
	if versionedExamples, ok := f.examples[version]; ok {
		if example, ok := versionedExamples[kind]; ok {
			example, err := f.scheme.DeepCopy(example)
			if err != nil {
				return nil, true, err
			}

			// This cast is ok since we originally copied a runtime.Object
			return example.(runtime.Object), true, nil
		}
	}

	example, err := f.scheme.New(version, kind)
	return example, false, err
}

func NewStaticExampleFetcher(scheme *runtime.Scheme) ExampleFetcher {
	return &staticExampleFetcher{
		scheme: scheme,
		examples: map[string]map[string]runtime.Object{
			"v1": {
				"Pod": &v1.Pod{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "webserver",
								Image: "nginx",
								Ports: []v1.ContainerPort{
									{Name: "http", ContainerPort: 80, Protocol: "TCP"},
								},
								VolumeMounts: []v1.VolumeMount{
									{Name: "html", ReadOnly: true, MountPath: "/usr/share/nginx/html"},
								},
							},
						},
						Volumes: []v1.Volume{
							{Name: "html", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
						},
					},
				},
			},
		},
	}
}
