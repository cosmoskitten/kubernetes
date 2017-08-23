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

# GoFmt apparently is changing @ head...

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${KUBE_ROOT}/hack/lib/init.sh"

cd "${KUBE_ROOT}"

first_merge=$(git log --merges --format='%H' -1)
if [ -z "${first_merge}" ]; then
    kube::log::error "Didn't find any merge commit in the git history. Something is odd. Cowardly failing."
    exit 1
fi
if [[ "$(git show -q --format="%s" ${first_merge})" != "Merge pull request "* ]]; then
    kube::log::error "Found merge commit ${first_merge} in the change history which is no Github merge:"
    kube::log::error ""
    kube::log::error "$(git show -q ${first_merge} | sed 's/^/    /')"
    kube::log::error ""
    kube::log::error "Consider to rebase onto the upstream branch again and avoid \"git pull\" without --rebase."
    exit 1
fi
