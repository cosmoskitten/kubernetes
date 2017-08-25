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

package volume

import (
	"github.com/golang/glog"
	"os"
)

// IsSameFSGroup is called only for requests to mount an already mounted
// volume. It checks if fsGroup of new mount request is the same or not.
// It returns false if it not the same. It also returns current Gid of a path
// provided for dir variable.
func IsSameFSGroup(dir string, fsGroup int64) (bool, int, error) {
	_, err := os.Stat(dir)
	if err != nil {
		glog.Errorf("Error getting stats for %s (%v)", dir, err)
		return false, 0, err
	}
	return true, int(fsGroup), nil
}
