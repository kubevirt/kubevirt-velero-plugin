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

set -eo pipefail

if [ -z "$KUBEVIRTCI_PATH" ]; then
    KUBEVIRTCI_PATH="$(
        cd "$(dirname "$BASH_SOURCE[0]")/"
        echo "$(pwd)/"
    )"../../cluster-up/
fi

readonly MAX_CDI_WAIT_RETRY=30
readonly CDI_WAIT_TIME=10

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source hack/config.sh
source ${KUBEVIRTCI_PATH}hack/common.sh
source ${KUBEVIRTCI_PATH}cluster/$KUBEVIRT_PROVIDER/provider.sh
kubectl="${_cli} --prefix $provider_prefix ssh node01 -- sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf"

CONFIG=$(pwd)/_ci-configs/${KUBEVIRT_PROVIDER}/.kubeconfig
VELERO_DIR=$(pwd)/hack/velero

# parsetTestOpts sets 'pkgs' and test_args
function parseTestOpts() {
    pkgs=""
    test_args=""
    while [[ $# -gt 0 ]] && [[ $1 != "" ]]; do
        case "${1}" in
        --test-args=*)
            test_args="${1#*=}"
            shift 1
            ;;
        ./*...)
            pkgs="${pkgs} ${1}"
            shift 1
            ;;
        *)
            echo "ABORT: Unrecognized option \"$1\""
            exit 1
            ;;
        esac
    done
}

parseTestOpts "${@}"

test_args="${test_args} -ginkgo.v"

test_command="${TESTS_OUT_DIR}/tests.test -test.timeout 360m ${test_args}"
echo "$test_command"
(
    cd ${TESTS_DIR}
    KUBECONFIG=${CONFIG} PATH=${PATH}:${VELERO_DIR} ${test_command}
)