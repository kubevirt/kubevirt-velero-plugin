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

set -euo pipefail

export PATH=$PATH:$HOME/gopath/bin

test_path="tests"
GOBIN=$(go env GOBIN)
if [ -d "$GOBIN" ]; then
  ginkgo_path=$GOBIN/ginkgo
else
  ginkgo_path=$(go env GOPATH)/bin/ginkgo
fi
(cd $test_path; go install github.com/onsi/ginkgo/v2/ginkgo@v2.17.1)
(cd $test_path; $ginkgo_path build .)
mv ${test_path}/tests.test ${TESTS_OUT_DIR}
