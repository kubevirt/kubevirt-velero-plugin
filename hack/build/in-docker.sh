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

set -e

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/../config.sh

WORK_DIR="/go/src/kubevirt.io/kubevirt-velero-plugin"

# Ensure that a build server is running
if [ -z "$(docker ps --format '{{.Image}}' | grep ${BUILDER_IMAGE})" ]; then
    docker run \
        --rm \
        -d \
        --name ${BUILDER_CONTAINER_NAME} \
        -v ${PLUGIN_DIR}:${WORK_DIR}:rw,Z \
        -v ${CACHE_DIR}:/gocache:rw,Z \
        -e RUN_UID=$(id -u) \
        -e RUN_GID=$(id -g) \
        -e KUBEVIRTCI_RUNTIME=${KUBEVIRTCI_RUNTIME} \
        -e GOCACHE=/gocache \
        -w ${WORK_DIR} \
        --privileged \
        --net=host \
        -v /var/run/docker.sock:/var/run/docker.sock \
        ${BUILDER_IMAGE} "while true; do sleep 24h; done"
fi

# Execute the build
[ -t 1 ] && USE_TTY="-it"

docker exec -it -e VERSION=${VERSION} -e IMAGE=${IMAGE} ${BUILDER_CONTAINER_NAME} bash -c "$@"
