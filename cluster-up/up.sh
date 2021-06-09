#!/usr/bin/env bash

if [ -z "$KUBEVIRTCI_PATH" ]; then
    KUBEVIRTCI_PATH="$(
        cd "$(dirname "$BASH_SOURCE[0]")/"
        echo "$(pwd)/"
    )"
fi

source ${KUBEVIRTCI_PATH}hack/common.sh
source ${KUBEVIRTCI_CLUSTER_PATH}/$KUBEVIRT_PROVIDER/provider.sh
up

# check if the environment has a corrupted host
if [[ $(${KUBEVIRTCI_PATH}kubectl.sh get nodes | grep localhost) != "" ]]; then
    echo "The environment has a corrupted host"
    exit 1
fi

kubectl="${_cli} --prefix $provider_prefix ssh node01 -- sudo kubectl --kubeconfig=/etc/kubernetes/admin.conf"

# Deploy KubeVirt
$kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml
$kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml
$kubectl wait -n kubevirt deployment/virt-operator   --for=condition=Available --timeout=${KUBEVIRT_DEPLYOMENT_TIMEOUT}s

# Deploy CDI
$kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml
$kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml
$kubectl wait -n cdi deployment/cdi-operator   --for=condition=Available --timeout=${KUBEVIRT_DEPLYOMENT_TIMEOUT}s