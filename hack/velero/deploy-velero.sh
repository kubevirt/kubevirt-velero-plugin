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

VELERO_NAMESPACE=${VELERO_NAMESPACE:-velero}
VOLUME_SNAPSHOT_CLASS=${VOLUME_SNAPSHOT_CLASS:-csi-rbdplugin-snapclass}

if [ -z "$KUBEVIRTCI_PATH" ]; then
    KUBEVIRTCI_PATH="$(
        cd "$(dirname "$BASH_SOURCE[0]")/"
        echo "$(pwd)/"
    )"../../cluster-up/
fi
script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/../config.sh

velero_resources_dir=${script_dir}/../velero

source ${KUBEVIRTCI_PATH}cluster/$KUBEVIRT_PROVIDER/provider.sh

if [[ ! $(_kubectl get deployments -n velero | grep minio) ]]; then
  _kubectl apply -f https://raw.githubusercontent.com/vmware-tanzu/velero/main/examples/minio/00-minio-deployment.yaml
  _kubectl wait -n velero deployment/minio --for=condition=Available --timeout=${DEPLOYMENT_TIMEOUT}s
fi

kvp::fetch_velero

PLUGINS=velero/velero-plugin-for-aws:v1.14.0

if [[ ! $(_kubectl get deployments -n velero | grep velero) ]]; then
  echo "Plugins: ${PLUGINS}"

  ${VELERO_DIR}/velero install \
    --namespace ${VELERO_NAMESPACE} \
    --provider aws \
    --plugins ${PLUGINS} \
    --bucket velero \
    --secret-file ${velero_resources_dir}/credentials-velero \
    --use-volume-snapshots=false \
    --features EnableCSI \
    --velero-pod-mem-request 512Mi \
    --velero-pod-mem-limit 1Gi \
    --kubeconfig $(pwd)/_ci-configs/${KUBEVIRT_PROVIDER}/.kubeconfig \
    --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://minio.velero.svc:9000

  _kubectl patch deployment velero -n velero --type='json' \
    -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--resource-timeout=20m"}]'
  _kubectl rollout status deployment/velero -n velero --timeout=${DEPLOYMENT_TIMEOUT}s
  _kubectl label volumesnapshotclass/${VOLUME_SNAPSHOT_CLASS} velero.io/csi-volumesnapshot-class=true --overwrite=true
fi
