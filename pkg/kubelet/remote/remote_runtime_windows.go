// +build windows

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

package remote

import (
	"github.com/golang/glog"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ContainerStats requests the stats for a given container and returns them.
func (r *RemoteRuntimeService) ContainerStats(req *runtimeapi.ContainerStatsRequest) (*runtimeapi.ContainerStatsResponse, error) {
	ctx, cancel := getContextWithTimeout(r.timeout)
	defer cancel()

	resp, err := r.runtimeClient.ContainerStats(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListContainerStats requests the stats for all containers based on the filter in req and returns them.
func (r *RemoteRuntimeService) ListContainerStats(req *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	ctx, cancel := getContextWithTimeout(r.timeout)
	defer cancel()

	resp, err := r.runtimeClient.ListContainerStats(ctx, req)
	if err != nil {
		glog.Errorf("ListContainersStats with filter %q from runtime service failed: %v", req.Filter, err)
		return nil, err
	}

	return resp, nil
}
