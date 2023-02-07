#!/usr/bin/env bash
#
# This file is part of the KubeVirt project
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
#
# Copyright 2021 Red Hat, Inc.
#
set -ex

KUBEVIRT_STORAGE=rook-ceph-default
CDI_DV_GC=${CDI_DV_GC:--1}
source ./hack/config.sh
source cluster-up/cluster/$KUBEVIRT_PROVIDER/provider.sh

# Deploy KubeVirt
_kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml
_kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml

_kubectl wait -n kubevirt deployment/virt-operator   --for=condition=Available --timeout=${KUBEVIRT_DEPLOYMENT_TIMEOUT}s


${_ssh} node01 "sudo docker pull quay.io/kubevirtci/alpine-with-test-tooling-container-disk:2205291325-d8fc489"


# Ensure the KubeVirt CR is created
count=0
until _kubectl -n kubevirt get kv kubevirt; do
    ((count++)) && ((count == 30)) && echo "KubeVirt CR not found" && exit 1
    echo "waiting for KubeVirt CR"
    sleep 1
done

# Wait until KubeVirt is ready
count=0
until _kubectl wait -n kubevirt kv kubevirt --for condition=Available --timeout 5m; do
    ((count++)) && ((count == 5)) && echo "KubeVirt not ready in time" && exit 1
    echo "Error waiting for KubeVirt to be Available, sleeping 1m and retrying"
    sleep 1m
done

# Patch kubevirt with hotplug feature gate enabled
_kubectl patch -n kubevirt kubevirt kubevirt --type merge -p '{"spec": {"configuration": { "developerConfiguration": { "featureGates": ["HotplugVolumes"] }}}}'

if [[ "$KUBEVIRT_DEPLOY_CDI" != "false" ]] && [[ $CDI_DV_GC != "0" ]]; then
    _kubectl patch cdi cdi --type merge -p '{"spec": {"config": {"dataVolumeTTLSeconds": '"$CDI_DV_GC"'}}}'
fi
