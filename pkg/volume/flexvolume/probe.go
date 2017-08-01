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

package flexvolume

import (
	"io/ioutil"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/volume"
)

type flexVolumeProber struct {
	pluginDir string // Flexvolume driver directory
}

func NewFlexVolumeProber(pluginDir string) volume.DynamicPluginProber {
	return &flexVolumeProber{pluginDir: pluginDir}
}

func (prober *flexVolumeProber) Probe() []volume.VolumePlugin {
	plugins := []volume.VolumePlugin{}

	files, _ := ioutil.ReadDir(prober.pluginDir)
	for _, f := range files {
		// only directories are counted as plugins
		// and pluginDir/dirname/dirname should be an executable
		// unless dirname contains '~' for escaping namespace
		// e.g. dirname = vendor~cifs
		// then, executable will be pluginDir/dirname/cifs
		if f.IsDir() {
			plugin, err := NewFlexVolumePlugin(prober.pluginDir, f.Name())
			if err != nil {
				glog.Errorf(
					"Error creating Flexvolume plugin from directory %s, skipping. Error: %s",
					f.Name(), err)
				continue
			}

			plugins = append(plugins, plugin)
		}
	}
	return plugins
}
