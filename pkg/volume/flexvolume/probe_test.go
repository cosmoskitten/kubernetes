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
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	utilfs "k8s.io/kubernetes/pkg/util/filesystem"
	"k8s.io/kubernetes/pkg/volume"
	"path"
	"testing"
)

const (
	pluginDir  = "/flexvolume"
	driverName = "fake-driver"
)

// Probes a driver installed before prober initialization.
func TestProberExistingDriverBeforeInit(t *testing.T) {
	// Arrange
	driverPath, _, watcher, prober := initTestEnvironment(t)

	// Act
	updated, plugins, err := prober.Probe()

	// Assert
	assert.True(t, updated)
	assert.Equal(t, 1, len(plugins))
	assert.Equal(t, pluginDir, watcher.watches[0])
	assert.Equal(t, driverPath, watcher.watches[1])
	assert.NoError(t, err)

	// Should no longer probe.

	// Act
	updated, plugins, err = prober.Probe()
	// Assert
	assert.False(t, updated)
	assert.Equal(t, 0, len(plugins))
	assert.NoError(t, err)
}

// Probes newly added drivers after prober is running.
func TestProberAddDriver(t *testing.T) {
	// Arrange
	_, fs, watcher, prober := initTestEnvironment(t)
	prober.Probe()
	updated, _, _ := prober.Probe()
	assert.False(t, updated)

	// Call probe after a file is added. Should return true.

	// Arrange
	const driverName2 = "fake-driver2"
	driverPath := path.Join(pluginDir, driverName2)
	installDriver(driverName2, fs)
	watcher.TriggerEvent(fsnotify.Create, driverPath)
	watcher.TriggerEvent(fsnotify.Create, path.Join(driverPath, driverName2))

	// Act
	updated, plugins, err := prober.Probe()

	// Assert
	assert.True(t, updated)
	assert.Equal(t, 2, len(plugins))                                     // 1 existing, 1 newly added
	assert.Equal(t, driverPath, watcher.watches[len(watcher.watches)-1]) // Checks most recent watch
	assert.NoError(t, err)

	// Call probe again, should return false.

	// Act
	updated, _, err = prober.Probe()
	// Assert
	assert.False(t, updated)
	assert.NoError(t, err)

	// Call probe after a non-driver file is added in a subdirectory. Should return true.

	// Arrange
	fp := path.Join(driverPath, "dummyfile")
	fs.Create(fp)
	watcher.TriggerEvent(fsnotify.Create, fp)

	// Act
	updated, plugins, err = prober.Probe()

	// Assert
	assert.True(t, updated)
	assert.Equal(t, 2, len(plugins)) // Number of plugins should not change.
	assert.NoError(t, err)

	// Call probe again, should return false.
	// Act
	updated, _, err = prober.Probe()
	// Assert
	assert.False(t, updated)
	assert.NoError(t, err)
}

func TestEmptyPluginDir(t *testing.T) {
	// Arrange
	fs := utilfs.NewFakeFs()
	watcher := NewMockWatcher()
	prober := &flexVolumeProber{
		pluginDir: pluginDir,
		watcher:   watcher,
		fs:        fs,
		factory:   mockPluginFactory{error: false},
	}
	prober.Init()

	// Act
	updated, plugins, err := prober.Probe()

	// Assert
	assert.True(t, updated)
	assert.Equal(t, 0, len(plugins))
	assert.NoError(t, err)
}

// Issue an event to remove plugindir. New directory should still be watched.
func TestRemovePluginDir(t *testing.T) {
	// Arrange
	driverPath, fs, watcher, _ := initTestEnvironment(t)
	fs.RemoveAll(pluginDir)
	watcher.TriggerEvent(fsnotify.Remove, path.Join(driverPath, driverName))
	watcher.TriggerEvent(fsnotify.Remove, driverPath)
	watcher.TriggerEvent(fsnotify.Remove, pluginDir)

	// Act: The handler triggered by the above events should have already handled the event appropriately.

	// Assert
	assert.Equal(t, 3, len(watcher.watches)) // 2 from initial setup, 1 from new watch.
	assert.Equal(t, pluginDir, watcher.watches[len(watcher.watches)-1])
}

// Issue multiple events and probe multiple times. Should give true, false, false...
func TestProberMultipleEvents(t *testing.T) {
	// Arrange
	_, fs, watcher, prober := initTestEnvironment(t)
	for i := 0; i < 5; i++ {
		newDriver := fmt.Sprintf("multi-event-driver%d", 1)
		installDriver(newDriver, fs)
		driverPath := path.Join(pluginDir, newDriver)
		watcher.TriggerEvent(fsnotify.Create, driverPath)
		watcher.TriggerEvent(fsnotify.Create, path.Join(driverPath, newDriver))
	}

	// Act
	updated, _, err := prober.Probe()

	// Assert
	assert.True(t, updated)
	assert.NoError(t, err)
	for i := 0; i < 4; i++ {
		updated, _, err = prober.Probe()
		assert.False(t, updated)
		assert.NoError(t, err)
	}
}

// When many events are triggered quickly in succession, events should stop triggering a probe update
// after a certain limit.
func TestProberRateLimit(t *testing.T) {
	// Arrange
	driverPath, _, watcher, prober := initTestEnvironment(t)
	for i := 0; i < watchEventLimit; i++ {
		watcher.TriggerEvent(fsnotify.Write, path.Join(driverPath, driverName))
	}

	// Act
	updated, plugins, err := prober.Probe()

	// Assert
	assert.True(t, updated)
	assert.Equal(t, 1, len(plugins))
	assert.NoError(t, err)

	// Arrange
	watcher.TriggerEvent(fsnotify.Write, path.Join(driverPath, driverName))

	// Act
	updated, _, err = prober.Probe()

	// Assert
	assert.False(t, updated)
	assert.NoError(t, err)
}

func TestProberError(t *testing.T) {
	fs := utilfs.NewFakeFs()
	watcher := NewMockWatcher()
	prober := &flexVolumeProber{
		pluginDir: pluginDir,
		watcher:   watcher,
		fs:        fs,
		factory:   mockPluginFactory{error: true},
	}
	installDriver(driverName, fs)
	prober.Init()

	_, _, err := prober.Probe()
	assert.Error(t, err)
}

// Installs a mock driver (an empty file) in the mock fs.
func installDriver(driverName string, fs utilfs.Filesystem) {
	driverPath := path.Join(pluginDir, driverName)
	fs.MkdirAll(driverPath, 0666)
	fs.Create(path.Join(driverPath, driverName))
}

// Initializes mocks, installs a single driver in the mock fs, then initializes prober.
func initTestEnvironment(t *testing.T) (
	driverPath string,
	fs utilfs.Filesystem,
	watcher *mockWatcher,
	prober volume.DynamicPluginProber) {
	fs = utilfs.NewFakeFs()
	watcher = NewMockWatcher()
	prober = &flexVolumeProber{
		pluginDir: pluginDir,
		watcher:   watcher,
		fs:        fs,
		factory:   mockPluginFactory{error: false},
	}
	driverPath = path.Join(pluginDir, driverName)
	installDriver(driverName, fs)
	prober.Init()

	assert.NotNilf(t, watcher.eventHandler,
		"Expect watch event handler to be registered after prober init, but is not.")
	return
}

// Mock filesystem watcher
type mockWatcher struct {
	watches      []string // List of watches added by the prober, ordered from least recent to most recent.
	eventHandler FSEventHandler
}

var _ FSWatcher = &mockWatcher{}

func NewMockWatcher() *mockWatcher {
	return &mockWatcher{
		watches: nil,
	}
}

func (w *mockWatcher) Init(eventHandler FSEventHandler, _ FSErrorHandler) {
	w.eventHandler = eventHandler
}

func (w *mockWatcher) AddWatch(path string) error {
	w.watches = append(w.watches, path)
	return nil
}

// Triggers a mock filesystem event.
func (w *mockWatcher) TriggerEvent(op fsnotify.Op, filename string) {
	w.eventHandler(fsnotify.Event{Op: op, Name: filename})
}

// Mock Flexvolume plugin
type mockPluginFactory struct {
	error bool // Indicates whether an error should be returned.
}

var _ PluginFactory = mockPluginFactory{}

func (m mockPluginFactory) NewFlexVolumePlugin(_, driverName string) (volume.VolumePlugin, error) {
	if m.error {
		return nil, fmt.Errorf("Flexvolume plugin error")
	}
	// Dummy Flexvolume plugin. Prober never interacts with the plugin.
	return &flexVolumePlugin{driverName: driverName}, nil
}
