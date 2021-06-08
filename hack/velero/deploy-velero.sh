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
velero_dir=${script_dir}/../velero
source "${script_dir}"/../config.sh

kubectl apply -f ${velero_dir}/minio-deployment.yaml
kubectl wait -n velero deployment/minio --for=condition=Available --timeout=${DEPLYOMENT_TIMEOUT}s

echo ${IMAGE}:${VERSION}

${velero_dir}/velero install \
  --provider aws \
  --plugins velero/velero-plugin-for-aws:v1.0.0,velero/velero-plugin-for-csi:v0.1.0 \
  --bucket velero \
  --secret-file ${velero_dir}/credentials-velero \
  --use-volume-snapshots=true \
  --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://minio.velero.svc:9000 \
  --snapshot-location-config region=minio,s3ForcePathStyle="true",s3Url=http://minio.velero.svc:9000 \
  --features=EnableCSI

kubectl wait -n velero deployment/velero --for=condition=Available --timeout=${DEPLYOMENT_TIMEOUT}s