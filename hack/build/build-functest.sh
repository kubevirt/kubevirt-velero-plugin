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
(cd $test_path; go install github.com/onsi/ginkgo/ginkgo@latest)
(cd $test_path; GOFLAGS= go get github.com/onsi/gomega)
(cd $test_path; go mod  tidy; go mod vendor)
test_out_path=${test_path}/_out
mkdir -p ${test_out_path}
(cd $test_path; ginkgo build .)
mv ${test_path}/tests.test ${TESTS_OUT_DIR}

