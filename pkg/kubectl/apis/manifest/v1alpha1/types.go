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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PackageMeta contains all the metadata driving package searching and browsing, and fork/rebase upgrade workflows.
type PackageMeta struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Description of the application package.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// An pointer to the icon.
	Icon string `json:"icon,omitempty" yaml:"icon,omitempty"`

	// Search keywords for this application package.
	Keywords []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`

	// Homepage of the application package.
	Home string `json:"home,omitempty" yaml:"home,omitempty"`

	// Source specifies the upstream URL, e.g. https://github.com/foo/bar.git, file://host/path, etc.
	// hosting the resource files specified in Base and Overlays.
	// This is useful in the fork/rebase workflow.
	Source string `json:"source,omitempty" yaml:"sources,omitempty"`

	// Version of the application package.
	AppVersion string `json:"appVersion,omitempty" yaml:"appVersion,omitempty"`
}

// Package could be either a base or a overlay manifest.
type Package struct {
	// Metadata for package searching and browsing, and fork/rebase workflow.
	Meta PackageMeta `json:"meta,omitempty" yaml:"meta,omitempty"`

	// Partial name that will prefix the name of the base resources.
	NamePrefix string `json:"namePrefix,omitempty" yaml:"namePrefix,omitempty"`

	// Labels to add to all objects and selectors.
	// These labels would also be used to form the selector for apply --prune
	// Named differently than “labels” to avoid confusion with metadata for this object
	ObjectLabels map[string]string `json:"objectLabels,omitempty" yaml:"objectLabels,omitempty"`

	// Annotations to add to all objects.
	ObjectAnnotations map[string]string `json:"objectAnnotations,omitempty" yaml:"objectAnnotations,omitempty"`

	// A Base entry specifies the file path, relative to Source, of an API resource definition.
	Bases []string `json:"bases,omitempty" yaml:"bases,omitempty"`

	// An Overlay entry specifies the file path, relative to Source, of a file
	// containing a Strategic Merge Patch overlay in API resource form.
	Overlays []string `json:"overlays,omitempty" yaml:"overlays,omitempty"`

	// List of configmaps to generate from configuration sources.
	// Base and overlay configmap should go to Bases and Overlays respectively.
	Configmaps []ConfigMap `json:"configmaps,omitempty" yaml:"configmaps,omitempty"`

	// List of secrets to generate from secret sources.
	// Base and overlay secret should go to Bases and Overlays respectively.
	Secrets []Secret `json:"secrets,omitempty" yaml:"secrets,omitempty"`

	// Whether PersistentVolumeClaims should be deleted with the other resources.
	OwnPersistentVolumeClaims bool `json:"ownPersistentVolumeClaims,omitempty" yaml:"ownPersistentVolumeClaims,omitempty"`

	// Whether recursive look for Kube-manifest.yaml, similar to `kubectl --recursive` behavior.
	Recursive bool `json:"recursive,omitempty" yaml:"recursive,omitempty"`

	// Whether prune resources not defined in Kube-manifest.yaml, similar to `kubectl apply --prune` behavior.
	Prune bool `json:"prune,omitempty" yaml:"prune,omitempty"`
}

// ConfigMap contains the metadata of how to generate a configmap.
type ConfigMap struct {
	// The type of the configmap. e.g. `env` and `file`.
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	// Name prefix of the configmap, the full name will be NamePrefix + hash(content of configmap).
	NamePrefix string `json:"namePrefix,omitempty" yaml:"namePrefix,omitempty"`

	// Configuration source file name.
	File string `json:"file,omitempty" yaml:"file,omitempty"`
}

// Secret contains the metadata of how to generate a secret.
type Secret struct {
	// The type of the secret. e.g. `tls`.
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	// Name prefix of the secret, the full name will be NamePrefix + hash(content of secret).
	NamePrefix string `json:"namePrefix,omitempty" yaml:"namePrefix,omitempty"`

	// Cert file for the secret.
	CertFile string `json:"certFile,omitempty" yaml:"certFile,omitempty"`

	// Key file for the secret.
	KeyFile string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
}
