#!/usr/bin/env bash

#Copyright 2023 The CDI Authors.
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

set -euo pipefail

export PATH=$PATH:$HOME/gopath/bin

DEFAULT_BACKUP_SCRIPT_PATH="$(
    cd "$(dirname "$BASH_SOURCE[0]")/../../cmd/velero-backup-restore"
    echo "$(pwd)/main.go"
)"
BACKUP_SCRIPT_PATH=${BACKUP_SCRIPT_PATH:-$DEFAULT_BACKUP_SCRIPT_PATH}
echo $BACKUP_SCRIPT_PATH

BACKUP_SCRIPT_BIN="${TESTS_OUT_DIR}/backup-restore"

go build -a -o ${BACKUP_SCRIPT_BIN} ${BACKUP_SCRIPT_PATH}

