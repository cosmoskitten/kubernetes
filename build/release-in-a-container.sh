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

# Check and error if not "in-a-container"
# TODO: There's certainly something better than this
# if /workspace dir and hostname contains a . - which containes don't appear to
if [[ ! -d /workspace && $(hostname -f) =~ \. ]]; then
  echo
  echo "'make release-in-a-container' is only available from a GCB container!"
  echo
  exit 1
fi

# First run "make cross" forcing output to 'dockerized' (see top level Makefile)
KUBE_OUTPUT_SUBPATH=_output/dockerized hack/make-rules/cross.sh

# Complete the release with the standard env
KUBE_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${KUBE_ROOT}/build/common.sh"
source "${KUBE_ROOT}/build/lib/release.sh"

kube::build::ensure_tar
kube::version::get_version_vars
kube::release::package_tarballs
