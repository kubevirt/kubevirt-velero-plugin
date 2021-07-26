#!/usr/bin/env bash
set -e

if [ -z "$KUBEVIRTCI_PATH" ]; then
    KUBEVIRTCI_PATH="$(
        cd "$(dirname "$BASH_SOURCE[0]")/"
        echo "$(pwd)/"
    )"
fi
test -t 1 && USE_TTY="-it"
DOCKER_GUEST_SOCK=/var/run/docker.sock
source ${KUBEVIRTCI_PATH}/config.sh

node=$1

if [ -z "$node" ]; then
    echo "node name required as argument"
    echo "k8s example: ./ssh node01"
    exit 1
fi

${_cli} ssh "$@"
