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

package upgrade

import (
	"fmt"
	"io"
	"io/ioutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	"k8s.io/kubernetes/pkg/api"
)

// FetchConfiguration fetches configuration required for upgrading your cluster from a file (which has precedence) or a ConfigMap in the cluster
func FetchConfiguration(client clientset.Interface, w io.Writer, cfgPath string) (*kubeadmapiext.MasterConfiguration, error) {
	fmt.Println("[upgrade] Making sure the configuration is correct:")

	versionedcfg := &kubeadmapiext.MasterConfiguration{}
	configBytes := []byte{}
	var err error

	if cfgPath != "" {
		fmt.Printf("[upgrade/config] Reading configuration options from a file: %s\n", cfgPath)

		configBytes, err = ioutil.ReadFile(cfgPath)
		if err != nil {
			fmt.Printf("[upgrade/config] FATAL: Could not load configuration from the file: %v\n", err)
			return nil, err
		}
	} else {
		configMap, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(constants.MasterConfigurationConfigMap, metav1.GetOptions{})
		if err == nil {
			fmt.Printf("[upgrade/config] Reading configuration from the cluster (you can get this with 'kubectl -n %s get cm %s -oyaml')\n", metav1.NamespaceSystem, constants.MasterConfigurationConfigMap)
			configBytes = []byte(configMap.Data[constants.MasterConfigurationConfigMapKey])
		} else {
			fmt.Println("[upgrade/config] WARNING: All configuration is defaulted. If you are sure you want to customize something, pass a configuration file to --config")
			// Sleep here or ask to proceed?
		}
	}

	// Take the versioned configuration populated from the configmap, default it and validate
	// Return the internal version of the API object
	if versionedcfg, err = bytesToValidatedMasterConfig(configBytes); err != nil {
		fmt.Printf("[upgrade/config] FATAL: Could not load configuration from the file: %v\n", err)
		return nil, err
	}

	return versionedcfg, nil
}

func bytesToValidatedMasterConfig(b []byte) (*kubeadmapiext.MasterConfiguration, error) {
	cfg := &kubeadmapiext.MasterConfiguration{}
	finalCfg := &kubeadmapiext.MasterConfiguration{}
	internalcfg := &kubeadmapi.MasterConfiguration{}

	if err := runtime.DecodeInto(api.Codecs.UniversalDecoder(), b, cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config from bytes: %v", err)
	}
	// Default and convert to the internal version
	api.Scheme.Default(cfg)
	api.Scheme.Convert(cfg, internalcfg, nil)

	// Applies dynamic defaults to settings not provided with flags
	if err := configutil.SetInitDynamicDefaults(internalcfg); err != nil {
		return nil, err
	}
	// Validates cfg (flags/configs + defaults + dynamic defaults)
	if err := validation.ValidateMasterConfiguration(internalcfg).ToAggregate(); err != nil {
		return nil, err
	}
	// Finally converts back to the external version
	api.Scheme.Convert(internalcfg, finalCfg, nil)
	return finalCfg, nil
}
