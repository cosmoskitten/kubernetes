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

// Reads the pod configuration from file or a directory of files.
package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
)

type sourceFileListerWatcher struct {
	path           string
	nodeName       types.NodeName
	period         time.Duration
	store          cache.Store
	fileKeyMapping map[string]string
	updates        chan<- interface{}
	locker         sync.Locker
}

func NewSourceFile(path string, nodeName types.NodeName, period time.Duration, updates chan<- interface{}) {
	// "golang.org/x/exp/inotify" requires a path without trailing "/"
	path = strings.TrimRight(path, string(os.PathSeparator))

	lw := newSourceFileListerWatcher(path, nodeName, period, updates)
	glog.V(1).Infof("Watching path %q", path)
	lw.run()
}

func newSourceFileListerWatcher(path string, nodeName types.NodeName, period time.Duration, updates chan<- interface{}) *sourceFileListerWatcher {
	send := func(objs []interface{}) {
		var pods []*v1.Pod
		for _, o := range objs {
			pods = append(pods, o.(*v1.Pod))
		}
		updates <- kubetypes.PodUpdate{Pods: pods, Op: kubetypes.SET, Source: kubetypes.FileSource}
	}
	store := cache.NewUndeltaStore(send, cache.MetaNamespaceKeyFunc)
	return &sourceFileListerWatcher{
		path:           path,
		nodeName:       nodeName,
		period:         period,
		store:          store,
		fileKeyMapping: map[string]string{},
		updates:        updates,
		locker:         &sync.Mutex{},
	}
}

func (s *sourceFileListerWatcher) run() {
	go wait.Forever(func() {
		if err := s.listConfig(); err != nil {
			glog.Errorf("unable to read config path %q: %v", s.path, err)
		}
	}, s.period)

	s.watch()
}

func (s *sourceFileListerWatcher) applyDefaults(pod *api.Pod, source string) error {
	return applyDefaults(pod, source, true, s.nodeName)
}

func (s *sourceFileListerWatcher) listConfig() error {
	path := s.path
	statInfo, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// Emit an update with an empty PodList to allow FileSource to be marked as seen
		s.updates <- kubetypes.PodUpdate{Pods: []*v1.Pod{}, Op: kubetypes.SET, Source: kubetypes.FileSource}
		return fmt.Errorf("path does not exist, ignoring")
	}

	switch {
	case statInfo.Mode().IsDir():
		pods, err := s.extractFromDir(path)
		if err != nil {
			return err
		}
		if len(pods) == 0 {
			// Emit an update with an empty PodList to allow FileSource to be marked as seen
			s.updates <- kubetypes.PodUpdate{Pods: pods, Op: kubetypes.SET, Source: kubetypes.FileSource}
			return nil
		}
		return s.replaceStore(pods...)

	case statInfo.Mode().IsRegular():
		pod, err := s.extractFromFile(path)
		if err != nil {
			return err
		}
		return s.replaceStore(pod)

	default:
		return fmt.Errorf("path is not a directory or file")
	}
}

// Get as many pod configs as we can from a directory. Return an error if and only if something
// prevented us from reading anything at all. Do not return an error if only some files
// were problematic.
func (s *sourceFileListerWatcher) extractFromDir(name string) ([]*v1.Pod, error) {
	dirents, err := filepath.Glob(filepath.Join(name, "[^.]*"))
	if err != nil {
		return nil, fmt.Errorf("glob failed: %v", err)
	}

	pods := make([]*v1.Pod, 0)
	if len(dirents) == 0 {
		return pods, nil
	}

	sort.Strings(dirents)
	for _, path := range dirents {
		statInfo, err := os.Stat(path)
		if err != nil {
			glog.V(1).Infof("Can't get metadata for %q: %v", path, err)
			continue
		}

		switch {
		case statInfo.Mode().IsDir():
			glog.V(1).Infof("Not recursing into config path %q", path)
		case statInfo.Mode().IsRegular():
			pod, err := s.extractFromFile(path)
			if err != nil {
				glog.V(1).Infof("Can't process config file %q: %v", path, err)
			} else {
				pods = append(pods, pod)
			}
		default:
			glog.V(1).Infof("Config path %q is not a directory or file: %v", path, statInfo.Mode())
		}
	}
	return pods, nil
}

func (s *sourceFileListerWatcher) extractFromFile(filename string) (pod *v1.Pod, err error) {
	glog.V(3).Infof("Reading config file %q", filename)
	defer func() {
		if err == nil && pod != nil {
			objKey, keyErr := cache.MetaNamespaceKeyFunc(pod)
			if keyErr != nil {
				err = keyErr
				return
			}
			s.locker.Lock()
			defer s.locker.Unlock()
			s.fileKeyMapping[filename] = objKey
		}
	}()

	file, err := os.Open(filename)
	if err != nil {
		return pod, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return pod, err
	}

	defaultFn := func(pod *api.Pod) error {
		return s.applyDefaults(pod, filename)
	}

	parsed, pod, podErr := tryDecodeSinglePod(data, defaultFn)
	if parsed {
		if podErr != nil {
			return pod, podErr
		}
		return pod, nil
	}

	return pod, &parseError{fmt.Sprintf("%v: couldn't parse as pod(%v), please check config file.\n", filename, podErr)}
}

func (s *sourceFileListerWatcher) replaceStore(pods ...*v1.Pod) (err error) {
	objs := []interface{}{}
	for _, pod := range pods {
		objs = append(objs, pod)
	}
	return s.store.Replace(objs, "")
}

type parseError struct {
	message string
}

func (e *parseError) Error() string {
	return e.message
}
