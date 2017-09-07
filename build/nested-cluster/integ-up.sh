#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

bazel run //build/nested-cluster:encapsulated-cluster-bundle
TMP_DIR=$(mktemp -d)
echo "Cluster admin kube config lives in $TMP_DIR"
CONTAINER=$(docker run -d --privileged=true --security-opt seccomp:unconfined --cap-add=SYS_ADMIN -v $TMP_DIR:/var/kubernetes -v /lib/modules:/lib/modules -v /sys/fs/cgroup:/sys/fs/cgroup:ro gcr.io/google-containers/encapsulated-cluster:0.1)
echo "The root container is $CONTAINER"
