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

source hack/config.sh

readonly MAX_CDI_WAIT_RETRY=30
readonly CDI_WAIT_TIME=10

# when running on a cluster that was externally provided we might not have a velero command available,
# so now is the time to install it
kvp::fetch_velero

KUBEVIRTCI_CONFIG_PATH="$(
    cd "$(dirname "$BASH_SOURCE[0]")/../../"
    echo "$(pwd)/_ci-configs"
)"

BASE_PATH=${KUBEVIRTCI_CONFIG_PATH:-$PWD}
KUBECONFIG=${KUBECONFIG:-$BASE_PATH/$KUBEVIRT_PROVIDER/.kubeconfig}

if [ -z "${KUBECTL+x}" ]; then
    kubevirtci_kubectl="${BASE_PATH}/${KUBEVIRT_PROVIDER}/.kubectl"
    if [ -e ${kubevirtci_kubectl} ]; then
        KUBECTL=${kubevirtci_kubectl}
    else
        KUBECTL=$(which kubectl)
    fi
fi

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

echo $KUBECONFIG
echo $KUBECTL

arg_kubectl="${KUBECTL:+-kubectl-path=$KUBECTL}"
arg_kubeconfig="${KUBECONFIG:+-kubeconfig=$KUBECONFIG}"

test_args="${test_args} -ginkgo.v ${arg_kubectl} ${arg_kubeconfig}"

test_command="${TESTS_OUT_DIR}/tests.test -test.timeout 360m ${test_args}"
kubeconfig_arg=${KUBECONFIG:-${kubeconfig}}
velero_path=$(pwd)/${VELERO_DIR}

(
    cd ${TESTS_DIR}
    KUBECONFIG=${kubeconfig_arg} PATH=${PATH}:${velero_path} ${test_command}
)
