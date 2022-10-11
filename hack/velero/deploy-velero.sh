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

PLUGINS=velero/velero-plugin-for-aws:v1.3.0
FEATURES=""

if [[ "${USE_CSI}" == "1" ]]; then
  PLUGINS="${PLUGINS},${CSI_PLUGIN}"
  FEATURES="--features=EnableCSI"
fi

if [[ "${USE_RESTIC}" == "1" ]]; then
  FEATURES="${FEATURES} --use-restic"
fi

if [[ ! $(_kubectl get deployments -n velero | grep velero) ]]; then
  echo "Plugins: ${PLUGINS}"
  echo "Features: ${FEATURES}"

  ${VELERO_DIR}/velero install \
    --provider aws \
    --plugins ${PLUGINS} \
    --bucket velero \
    --secret-file ${velero_resources_dir}/credentials-velero \
    --use-volume-snapshots=true \
    --kubeconfig $(pwd)/_ci-configs/${KUBEVIRT_PROVIDER}/.kubeconfig \
    --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://minio.velero.svc:9000 \
    --snapshot-location-config region=minio \
    ${FEATURES}

  _kubectl wait -n velero deployment/velero --for=condition=Available --timeout=${DEPLOYMENT_TIMEOUT}s

  if [[ "${USE_CSI}" == "1" ]]; then
    _kubectl label volumesnapshotclass/csi-rbdplugin-snapclass velero.io/csi-volumesnapshot-class=true --overwrite=true
  fi
fi