#!/usr/bin/env bash

#Copyright 2021 The CDI Authors.
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

set -ex

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/../config.sh


source ${KUBEVIRTCI_PATH}cluster/$KUBEVIRT_PROVIDER/provider.sh

LOCAL_CLUSTER_REGISTRY_PREFIX=localhost:${PORT}/kubevirt

${OCI_BIN} tag ${DOCKER_PREFIX}/${IMAGE_NAME}:${DOCKER_TAG}  ${LOCAL_CLUSTER_REGISTRY_PREFIX}/${IMAGE_NAME}:${DOCKER_TAG}
${OCI_BIN} push ${TLS_SETTING} ${LOCAL_CLUSTER_REGISTRY_PREFIX}/${IMAGE_NAME}:${DOCKER_TAG}

# fetch latest version so it is available when container starts
${_ssh} node01 "sudo crictl pull ${KUBEVIRTCI_REGISTRY_PREFIX}/${IMAGE_NAME}:${DOCKER_TAG}"
