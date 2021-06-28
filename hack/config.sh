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

DOCKER_PREFIX=${DOCKER_PREFIX:-"quay.io/kubevirt"}
DOCKER_TAG=${DOCKER_TAG:-latest}

PROVIDER=${KUBEVIRT_PROVIDER:-k8s-1.19}
KUBEVIRTCI_TAG=2103240101-142f745

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
PKG=kubevirt.io/${IMAGE_NAME}
BIN=kubevirt-velero-plugin

_cli_container="${KUBEVIRTCI_GOCLI_CONTAINER:-quay.io/kubevirtci/gocli:${KUBEVIRTCI_TAG}}"
_cli_command="docker run --privileged --net=host --rm ${USE_TTY} -v /var/run/docker.sock:/var/run/docker.sock"
_cli="${_cli_command} ${_cli_container} --prefix ${PROVIDER}"

_ssh=hack/ssh.sh

LOCAL_REGISTRY=localhost
DOCKER_PREFIX=${LOCAL_REGISTRY}:${PORT}
REGISTRY=registry:5000
IMAGE_NAME=kubevirt-velero-plugin
DEFAULT_IMAGE=${REGISTRY}/${IMAGE_NAME}
IMAGE=${IMAGE:-${DEFAULT_IMAGE}}
VERSION=${VERSION:-0.1}

# Test infrastructure
DEPLOYMENT_TIMEOUT=480
USE_CSI=${USE_CSI:-1}
USE_RESTIC=${USE_RESTIC:-0}
CSI_PLUGIN=${CSI_PLUGIN:-velero/velero-plugin-for-csi:v0.1.2}