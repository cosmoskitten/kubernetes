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

CODEGEN_PKG=${CODEGEN_PKG:-$(ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo "k8s.io/code-generator")}

# build generators on-demand
CODEGEN_BUILT=false
function codegen::ensure-generators-built() {
  if [ ${CODEGEN_BUILT} = true ]; then
    return
  fi
  for gen in defaulter-gen conversion-gen client-gen lister-gen informer-gen deepcopy-gen conversion-gen; do
    echo "Building ${gen}"
    go install ${CODEGEN_PKG}/cmd/${gen}
  done
  CODEGEN_BUILT=true
}

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

function codegen::generate-internal-groups() {
  local GENS="$1" # the generators comma separated to run (deepcopy,defaulter,conversion,client,lister,informer) or "all"
  local BASE="$2" # the base packages are resolved against (empty or the vendor/ directory ${OUTPUT_PKG} is in)
  local OUTPUT_PKG="$3" # the output package name (without a "foo/var/vendor/" prefix, if inside vendor/)
  local INT_APIS_PKG="$4" # the internal types dir (e.g. k8s.io/kubernetes/pkg/apis)
  local EXT_APIS_PKG="$5" # the external types dir (e.g. k8s.io/api; often equals the internal types dir)
  local GROUPS_WITH_VERSIONS="$6" # groupA:v1,v2 groupB:v1 groupC:v2
  shift 6

  codegen::ensure-generators-built

  # enumerate group versions
  local ALL_FQ_APIS=() # e.g. k8s.io/kubernetes/pkg/apis/apps k8s.io/api/apps/v1
  local INT_FQ_APIS=() # e.g. k8s.io/kubernetes/pkg/apis/apps
  local EXT_FQ_APIS=() # e.g. k8s.io/api/apps/v1
  for GVs in ${GROUPS_WITH_VERSIONS}; do
    IFS=: read G Vs <<<"${GVs}"

    ALL_FQ_APIS+=(${INT_APIS_PKG}/${G})
    INT_FQ_APIS+=(${INT_APIS_PKG}/${G})

    # enumerate versions
    for V in ${Vs//,/ }; do
      ALL_FQ_APIS+=(${EXT_APIS_PKG}/${G}/${V})
      EXT_FQ_APIS+=(${EXT_APIS_PKG}/${G}/${V})
    done
  done

  if [ "${GENS}" = "all" ] || grep -qw "deepcopy" <<<"${GENS}"; then
    echo "Generating deepcopy funcs"
    ${GOPATH}/bin/deepcopy-gen -i $(codegen::join , "${ALL_FQ_APIS[@]}") -O zz_generated.deepcopy --bounding-dirs ${INT_APIS_PKG},${EXT_APIS_PKG} "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "defaulter" <<<"${GENS}"; then
    echo "Generating defaulters"
    ${GOPATH}/bin/defaulter-gen  -i $(codegen::join , "${EXT_FQ_APIS[@]}") -O zz_generated.defaults "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "conversion" <<<"${GENS}"; then
    echo "Generating conversions"
    ${GOPATH}/bin/conversion-gen -i $(codegen::join , "${ALL_FQ_APIS[@]}") -O zz_generated.conversions "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "client" <<<"${GENS}"; then
    echo "Generating clientset for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/clientset"
    ${GOPATH}/bin/client-gen --clientset-name internalversion --input-base "" --input $(codegen::join "/," "${INT_FQ_APIS[@]}")/ --clientset-path ${OUTPUT_PKG}/clientset --output-base "${BASE}" "$@"
    ${GOPATH}/bin/client-gen --clientset-name versioned --input-base "" --input $(codegen::join , "${EXT_FQ_APIS[@]}") --clientset-path ${OUTPUT_PKG}/clientset --output-base "${BASE}" "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "lister" <<<"${GENS}"; then
    echo "Generating listers for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/listers"
    ${GOPATH}/bin/lister-gen --input-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") --output-package ${OUTPUT_PKG}/listers --output-base "${BASE}" "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "informer" <<<"${GENS}"; then
    echo "Generating informers for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/informers"
    ${GOPATH}/bin/informer-gen \
             --input-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") \
             --versioned-clientset-package ${OUTPUT_PKG}/clientset/versioned \
             --internal-clientset-package ${OUTPUT_PKG}/clientset/internalversion \
             --listers-package ${OUTPUT_PKG}/listers \
             --output-package ${OUTPUT_PKG}/informers \
             --output-base ${BASE} \
             "$@"
  fi
}

function codegen::generate-groups() {
  local GENS="$1" # the generators comma separated to run (deepcopy,defaulter,conversion,client,lister,informer) or "all"
  local BASE="$2" # the base packages are resolved against (empty or the vendor/ directory ${OUTPUT_PKG} is in)
  local OUTPUT_PKG="$3" # the output package name (without a "foo/var/vendor/" prefix, if inside vendor/)
  local APIS_PKG="$4" # the external types dir (e.g. k8s.io/api; often equals the internal types dir)
  local GROUPS_WITH_VERSIONS="$5" # groupA:v1,v2,groupB,v1,groupC:v2
  shift 4

  codegen::ensure-generators-built

  # enumerate group versions
  local FQ_APIS=() # e.g. k8s.io/api/apps/v1
  for GVs in ${GROUPS_WITH_VERSIONS}; do
    IFS=: read G Vs <<<"${GVs}"

    # enumerate versions
    for V in ${Vs//,/ }; do
      FQ_APIS+=(${APIS_PKG}/${G}/${V})
    done
  done

  if [ "${GENS}" = "all" ] || grep -qw "deepcopy" <<<"${GENS}"; then
    echo "Generating deepcopy funcs"
    ${GOPATH}/bin/deepcopy-gen -i $(codegen::join , "${FQ_APIS[@]}") -O zz_generated.deepcopy --bounding-dirs ${APIS_PKG} "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "client" <<<"${GENS}"; then
    echo "Generating clientset for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/clientset"
    ${GOPATH}/bin/client-gen --clientset-name versioned --input-base "" --input $(codegen::join , "${FQ_APIS[@]}") --clientset-path ${OUTPUT_PKG}/clientset --output-base "${BASE}" "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "lister" <<<"${GENS}"; then
    echo "Generating listers for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/listers"
    ${GOPATH}/bin/lister-gen --input-dirs $(codegen::join , "${FQ_APIS[@]}") --output-package ${OUTPUT_PKG}/listers --output-base "${BASE}" "$@"
  fi

  if [ "${GENS}" = "all" ] || grep -qw "informer" <<<"${GENS}"; then
    echo "Generating informers for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/informers"
    ${GOPATH}/bin/informer-gen \
             --input-dirs $(codegen::join , "${FQ_APIS[@]}") \
             --versioned-clientset-package ${OUTPUT_PKG}/clientset/versioned \
             --listers-package ${OUTPUT_PKG}/listers \
             --output-package ${OUTPUT_PKG}/informers \
             --output-base ${BASE} \
             "$@"
  fi
}
