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

if [ -f cluster-up/hack/common.sh ]; then
    source cluster-up/hack/common.sh
fi

script_dir="$(cd "$(dirname "$0")" && pwd -P)"

PLUGIN_DIR="$(cd $(dirname $0)/../../ && pwd -P)"
BIN_DIR=${PLUGIN_DIR}/bin
OUT_DIR=${PLUGIN_DIR}/_output
TESTS_DIR=${OUT_DIR}/tests
TESTS_OUT_DIR=${OUT_DIR}/tests
BUILD_DIR=${PLUGIN_DIR}/hack/build
CACHE_DIR=${OUT_DIR}/gocache


# update this whenever builder Dockerfile is updated
BUILDER_TAG=${BUILDER_TAG:-0.1}
BUILDER_CONTAINER_NAME=kubevirt-velero-plugin-builder
UNTAGGED_BUILDER_IMAGE=${BUILDER_IMAGE:-quay.io/kubevirt/${BUILDER_CONTAINER_NAME}}
BUILDER_IMAGE=${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}
BUILDER_SPEC="${BUILD_DIR}/docker/builder"

DOCKER_HOST_SOCK=${DOCKER_HOST_SOCK:-/run/docker.sock}
DOCKER_GUEST_SOCK=${DOCKER_GUEST_SOCK:-/run/docker.sock}
DOCKER_CMD=${DOCKER_CMD:-docker -H unix://${DOCKER_HOST_SOCK}}

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
CSI_PLUGIN=${CSI_PLUGIN:-velero/velero-plugin-for-csi:main}
