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
	"path"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util/exec"
	utilstrings "k8s.io/kubernetes/pkg/util/strings"
	"k8s.io/kubernetes/pkg/volume"
)

// This is the primary entrypoint for volume plugins.
func ProbeVolumePlugins(pluginDir string) []volume.VolumePlugin {
	plugins := []volume.VolumePlugin{}

	files, _ := ioutil.ReadDir(pluginDir)
	for _, f := range files {
		// only directories are counted as plugins
		// and pluginDir/dirname/dirname should be an executable
		// unless dirname contains '~' for escaping namespace
		// e.g. dirname = vendor~cifs
		// then, executable will be pluginDir/dirname/cifs
		if f.IsDir() {
			execPath := path.Join(pluginDir, f.Name())

			attachablePlugin := &flexVolumeAttachablePlugin{
				flexVolumePlugin: flexVolumePlugin{
					driverName:          utilstrings.UnescapePluginName(f.Name()),
					execPath:            execPath,
					runner:              exec.New(),
					unsupportedCommands: []string{},
				},
			}

			// Check whether the plugin is attachable.
			ok, err := attachablePlugin.isAttachable()
			if err != nil {
				glog.Errorf("Unable to probe plugin: %s", attachablePlugin.driverName)
				continue
			}

			if ok {
				plugins = append(plugins, attachablePlugin)
			} else {
				// Plugin does not support attach/detach, so return flexVolumePlugin which supports
				// SetupAt & TearDown functionality
				plugin := &flexVolumePlugin{
					driverName:          utilstrings.UnescapePluginName(f.Name()),
					execPath:            execPath,
					runner:              exec.New(),
					unsupportedCommands: []string{},
				}
				plugins = append(plugins, plugin)
			}
		}
	}
	return plugins
}
