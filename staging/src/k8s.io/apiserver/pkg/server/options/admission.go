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

package options

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
)

// AdmissionOptions holds the admission options
type AdmissionOptions struct {
	// RecommendedPluginOrder holds an ordered list of plugin names we recommend to use by default
	RecommendedPluginOrder []string
	// DefaultOffPlugins a list of plugin names that should be disabled by default
	DefaultOffPlugins []string
	PluginNames       []string
	ConfigFile        string
	Plugins           *admission.Plugins
}

// NewAdmissionOptions creates a new instance of AdmissionOptions
// Note:
// In addition it calls RegisterAllAdmissionPlugins to register
// all generic admission plugins.
func NewAdmissionOptions() *AdmissionOptions {
	options := &AdmissionOptions{
		Plugins:                &admission.Plugins{},
		PluginNames:            []string{},
		RecommendedPluginOrder: []string{lifecycle.PluginName},
	}
	server.RegisterAllAdmissionPlugins(options.Plugins)
	return options
}

// AddFlags adds flags related to admission for a specific APIServer to the specified FlagSet
func (a *AdmissionOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(&a.PluginNames, "admission-control", a.PluginNames, ""+
		"Ordered list of plug-ins to do admission control of resources into cluster. "+
		"Comma-delimited list of: "+strings.Join(a.Plugins.Registered(), ", ")+".")

	fs.StringVar(&a.ConfigFile, "admission-control-config-file", a.ConfigFile,
		"File with admission control configuration.")
}

// ApplyTo adds the admission chain to the server configuration.
// In case admission plugin names were not provided by a custer-admin they will be prepared from the recommended/default values.
// In addition the method lazily initializes a generic plugin that is appended to the list of pluginInitializers
// note this method uses:
//  genericconfig.LoopbackClientConfig
//  genericconfig.SharedInformerFactory
//  genericconfig.Authorizer
func (a *AdmissionOptions) ApplyTo(serverCfg *server.Config, pluginInitializers ...admission.PluginInitializer) error {
	a.preparePluginNamesIfNotProvided()

	pluginsConfigProvider, err := admission.ReadAdmissionConfiguration(a.PluginNames, a.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read plugin config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(serverCfg.LoopbackClientConfig)
	if err != nil {
		return err
	}
	genericInitializer, err := initializer.New(clientset, serverCfg.SharedInformerFactory, serverCfg.Authorizer)
	if err != nil {
		return err
	}
	initializersChain := admission.PluginInitializers{}
	pluginInitializers = append(pluginInitializers, genericInitializer)
	initializersChain = append(initializersChain, pluginInitializers...)

	admissionChain, err := a.Plugins.NewFromPlugins(a.PluginNames, pluginsConfigProvider, initializersChain)
	if err != nil {
		return err
	}

	serverCfg.AdmissionControl = admissionChain
	return nil
}

func (a *AdmissionOptions) Validate() []error {
	errs := []error{}
	return errs
}

// preparePluginNamesIfNotProvided sets admission plugin names if they
// were not provided by a cluster-admin. This method makes use of RecommendedPluginOrder
// as well as DefaultOffPlugins fields.
func (a *AdmissionOptions) preparePluginNamesIfNotProvided() {
	if len(a.PluginNames) > 0 {
		return
	}
	a.PluginNames = a.RecommendedPluginOrder

	onlyEnabledPluginNames := []string{}
	for _, pluginName := range a.PluginNames {
		disablePlugin := false
		for _, disabledPluginName := range a.DefaultOffPlugins {
			if pluginName == disabledPluginName {
				disablePlugin = true
				break
			}
		}
		if !disablePlugin {
			onlyEnabledPluginNames = append(onlyEnabledPluginNames, pluginName)
		}
	}

	a.PluginNames = onlyEnabledPluginNames
}
