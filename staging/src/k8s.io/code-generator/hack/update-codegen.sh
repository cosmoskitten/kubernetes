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

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname ${BASH_SOURCE})/lib/codegen.sh"

codegen::generate-internal-groups "$(dirname ${BASH_SOURCE})/../../.." k8s.io/code-generator/examples/apiserver k8s.io/code-generator/examples/apiserver/apis k8s.io/code-generator/examples/apiserver/apis example:v1
codegen::generate-groups "$(dirname ${BASH_SOURCE})/../../.." k8s.io/code-generator/examples/crd k8s.io/code-generator/examples/crd/apis example:v1
