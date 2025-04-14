#!/usr/bin/env bash
#Copyright 2018 The CDI Authors.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

# KUBEVIRT variables have to be set before common.sh is sourced
KUBEVIRT_MEMORY_SIZE=${KUBEVIRT_MEMORY_SIZE:-9216M}
KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER:-k8s-1.29}
KUBEVIRT_DEPLOY_CDI=true
KUBEVIRT_VERSION=${KUBEVIRT_VERSION:-v1.4.0}
KUBEVIRT_DEPLOYMENT_TIMEOUT=${KUBEVIRT_DEPLOYMENT_TIMEOUT:-480}

if [ -f cluster-up/hack/common.sh ]; then
    source cluster-up/hack/common.sh
fi

script_dir="$(cd "$(dirname "$0")" && pwd -P)"

PLUGIN_DIR="$(cd $(dirname $0)/../../ && pwd -P)"
BIN_DIR=${PLUGIN_DIR}/bin
OUT_DIR=${PLUGIN_DIR}/_output
TESTS_DIR=${PLUGIN_DIR}/tests
TESTS_OUT_DIR=${OUT_DIR}/tests
BUILD_DIR=${PLUGIN_DIR}/hack/build
CACHE_DIR=${OUT_DIR}/gocache

if [[ $(which go 2>/dev/null) ]]; then
  GOOS=$(go env GOOS)
  GOARCH=$(go env GOARCH)
else
  GOOS=linux
  GOARCH=arch64
fi
PKG=kubevirt.io/${IMAGE_NAME}
BIN=kubevirt-velero-plugin

_ssh=${KUBEVIRTCI_PATH}ssh.sh

# Test infrastructure
DEPLOYMENT_TIMEOUT=600
USE_CSI=${USE_CSI:-1}
USE_RESTIC=${USE_RESTIC:-0}
VELERO_VERSION=${VELERO_VERSION:-v1.16.0}
VELERO_DIR=_output/velero/bin

source cluster-up/hack/config.sh

function kvp::fetch_velero() {
  if [[ ! -f "${VELERO_DIR}/velero" ]]; then
    mkdir -p ${VELERO_DIR}
    echo >&2 "Downloading velero version ${VELERO_VERSION}..."
    curl -LO https://github.com/vmware-tanzu/velero/releases/download/${VELERO_VERSION}/velero-${VELERO_VERSION}-linux-amd64.tar.gz \
      && tar -xzvf velero-${VELERO_VERSION}-linux-amd64.tar.gz \
      && rm velero-${VELERO_VERSION}-linux-amd64.tar.gz \
      && chmod u+x velero-${VELERO_VERSION}-linux-amd64/velero \
      && mv velero-${VELERO_VERSION}-linux-amd64/velero ${VELERO_DIR}/velero
  fi
}
