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

package cpumanager

//import (
//	"io/ioutil"
//	"reflect"
//	"testing"
//
//	"k8s.io/kubernetes/pkg/kubelet/cpumanager/topo"
//)
//
//func Test_discoverTopology(t *testing.T) {
//	// TODO(CD): Need to provide canned input to make this test portable
//	//           across systems and architectures.
//	cpuInfoFile, err := ioutil.ReadFile("/proc/cpuinfo")
//	if err != nil {
//		t.Errorf("couldn't read /proc/cpuinfo: %v", err)
//		return
//	}
//	type args struct {
//		cpuinfo []byte
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    *topo.CPUTopology
//		wantErr bool
//	}{
//		{
//			name:    "test",
//			args:    args{cpuinfo: cpuInfoFile},
//			want:    &topo.CPUTopology{NumCPUs: 4, Hyperthreading: false},
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := discoverTopology(tt.args.cpuinfo)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("discoverTopology() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("discoverTopology() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
