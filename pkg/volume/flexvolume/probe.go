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

	"os"

	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/apimachinery/pkg/util/errors"
)

type flexVolumeProber struct {
	mutex       sync.Mutex
	pluginDir   string // Flexvolume driver directory
	watcher     *fsnotify.Watcher
	probeNeeded bool      // Must only read and write this through testAndSetProbeNeeded.
	lastUpdated time.Time // Last time probeNeeded was updated.
}

func NewFlexVolumeProber(pluginDir string) volume.DynamicPluginProber {
	return &flexVolumeProber{pluginDir: pluginDir}
}

func (prober *flexVolumeProber) Init() error {
	prober.testAndSetProbeNeeded(true)
	prober.lastUpdated = time.Now()

	if err := prober.createPluginDir(); err != nil {
		return err
	}
	if err := prober.initWatcher(); err != nil {
		return err
	}

	go func() {
		defer prober.watcher.Close()
		for {
			select {
			case event := <-prober.watcher.Events:
				if err := prober.handleWatchEvent(event); err != nil {
					glog.Errorf("Flexvolume prober watch: %s", err)
				}
			case err := <-prober.watcher.Errors:
				glog.Errorf("Received an error from watcher: %s", err)
			}
		}
	}()

	return nil
}

// Probes for Flexvolume drivers.
// If a filesystem update has occurred since the last probe, updated = true
// and the list of probed plugins is returned.
// Otherwise, update = false and probedPlugins = nil.
//
// If an error occurs, updated and plugins are set arbitrarily.
func (prober *flexVolumeProber) Probe() (updated bool, plugins []volume.VolumePlugin, err error) {
	probeNeeded := prober.testAndSetProbeNeeded(false)

	if !probeNeeded {
		return false, nil, nil
	}

	files, err := ioutil.ReadDir(prober.pluginDir)
	if err != nil {
		return false, nil, fmt.Errorf("Error reading the Flexvolume directory: %s", err)
	}

	plugins = []volume.VolumePlugin{}
	allErrs := []error{}
	for _, f := range files {
		// only directories are counted as plugins
		// and pluginDir/dirname/dirname should be an executable
		// unless dirname contains '~' for escaping namespace
		// e.g. dirname = vendor~cifs
		// then, executable will be pluginDir/dirname/cifs
		if f.IsDir() {
			plugin, pluginErr := NewFlexVolumePlugin(prober.pluginDir, f.Name())
			if pluginErr != nil {
				pluginErr = fmt.Errorf(
					"Error creating Flexvolume plugin from directory %s, skipping. Error: %s",
					f.Name(), pluginErr)
				allErrs = append(allErrs, pluginErr)
				continue
			}

			plugins = append(plugins, plugin)
		}
	}

	return true, plugins, errors.NewAggregate(allErrs)
}

func (prober *flexVolumeProber) handleWatchEvent(event fsnotify.Event) error {
	eventPathAbs, err := filepath.Abs(event.Name)
	if err != nil {
		return err
	}

	pluginDirAbs, err := filepath.Abs(prober.pluginDir)
	if err != nil {
		return err
	}

	if eventOpIs(event, fsnotify.Remove) && eventPathAbs == pluginDirAbs {
		// pluginDir needs to exist in order to be watched.
		if err := prober.createPluginDir(); err != nil {
			return err
		}
		if err := prober.addWatchRecursive(pluginDirAbs); err != nil {
			return err
		}
	} else if eventOpIs(event, fsnotify.Create) {
		if err := prober.addWatchRecursive(eventPathAbs); err != nil {
			return err
		}
	}

	if time.Since(prober.lastUpdated) > time.Second { // Reduce the frequency of probes.
		prober.testAndSetProbeNeeded(true)
		prober.lastUpdated = time.Now()
	}

	return nil
}

// Recursively adds to watch all directories inside and including the file specified by the given filename.
// If the file is a symlink to a directory, it will watch the symlink but not any of the
// subdirectories.
func (prober *flexVolumeProber) addWatchRecursive(filename string) error {
	addWatch := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if err := prober.watcher.Add(path); err != nil {
				glog.Errorf("Error recursively adding watch: %v", err)
			}
		}
		return nil
	}
	return filepath.Walk(filename, addWatch)
}

// Creates a new filesystem watcher and adds watches for the plugin directory
// and all of its subdirectories.
func (prober *flexVolumeProber) initWatcher() error {
	var err error
	if prober.watcher, err = fsnotify.NewWatcher(); err != nil {
		return fmt.Errorf("Error creating new watcher: %s", err)
	}

	if err = prober.addWatchRecursive(prober.pluginDir); err != nil {
		return fmt.Errorf("Error adding watch on Flexvolume directory: %s", err)
	}

	return nil
}

// Creates the plugin directory, if it doesn't already exist.
func (prober *flexVolumeProber) createPluginDir() error {
	if _, err := os.Stat(prober.pluginDir); os.IsNotExist(err) {
		err := os.MkdirAll(prober.pluginDir, 0755)
		if err != nil {
			return fmt.Errorf("Error (re-)creating driver directory: %s", err)
		}
	}

	return nil
}

func (prober *flexVolumeProber) testAndSetProbeNeeded(newval bool) (oldval bool) {
	prober.mutex.Lock()
	defer prober.mutex.Unlock()
	oldval, prober.probeNeeded = prober.probeNeeded, newval
	return
}

func eventOpIs(event fsnotify.Event, op fsnotify.Op) bool {
	return event.Op&op == op
}
