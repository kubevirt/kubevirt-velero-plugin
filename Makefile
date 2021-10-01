# Copyright 2018 The Kubevirt Velero Plugin Authors.
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

.PHONY: clean \
	all \
	build-all \
	build-image \
	build-dirs \
	push-image \
	cluster-push-image \
	test \
	modules \
	vendor \
	gomod-update \
	stop-builder \
	local-deploy-velero \
	add-plugin \
	remove-plugin \
	local-undeploy-velero \
	push-builder \
	cluster-up \
	cluster-down \
	cluster-sync \
	test-functional \
	rebuild-functest \
	clean-test

DOCKER?=1
ifeq (${DOCKER}, 1)
	# use entrypoint.sh (default) as your entrypoint into the container
	DO=./hack/build/in-docker.sh
else
	DO=eval
endif

GREEN=\e[0;32m
WHITE=\e[0;37m
BIN=bin/kubevirt-velero-plugin
SRC_FILES=main.go \
	$(shell find pkg -name "*.go")

TESTS_OUT_DIR=_output/tests
TESTS_BINARY=_output/tests/tests.test
TESTS_SRC_FILES=\
	$(shell find tests -name "*.go")

PORT=$(shell ./cluster-up/cli.sh ports registry)


all: clean build-image

build-image: build-all
	@echo -e "${GREEN}Building plugin image${WHITE}"
	DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} PORT=${PORT} hack/build/build-image.sh

build-all: build-dirs ${BIN}

${BIN}: ${SRC_FILES}
	@echo -e "${GREEN}Building...${WHITE}"
	@${DO} hack/build/build.sh

push-image: build-image
	@echo -e "${GREEN}Pushing plugin image to local registry${WHITE}"
	DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} PORT=${PORT} hack/build/push-image.sh

cluster-push-image: push-image
	@echo -e "${GREEN}Pushing plugin image to local K8s cluster${WHITE}"
	DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} hack/build/cluster-push-image.sh

local-deploy-velero:
	@echo -e "${GREEN}Deploying velero to local cluster${WHITE}"
	@hack/velero/deploy-velero.sh

add-plugin: local-deploy-velero
	@echo -e "${GREEN}Adding the plugin to local Velero${WHITE}"
	@hack/velero/add-plugin.sh

remove-plugin:
	@echo -e "${GREEN}Removing the plugin from local Velero${WHITE}"
	@hack/velero/remove-plugin.sh

local-undeploy-velero:
	@echo -e "${GREEN}Removing velero from local cluster${WHITE}"
	@kubectl delete deployment velero -n velero --ignore-not-found=true
	@kubectl delete deployment minio -n velero --ignore-not-found=true

gomod-update: modules vendor

build-builder: stop-builder
	@hack/build/build-builder.sh

push-builder:
	@hack/build/push-builder.sh

clean-dirs:
	@echo -e "${GREEN}Removing output directories${WHITE}"
	@${DO} "rm -rf _output bin"

clean: clean-dirs stop-builder

test: build-dirs
	@echo -e "${GREEN}Testing${WHITE}"
	@${DO} "CGO_ENABLED=0 go test -v -timeout 60s ./pkg/..."

test-functional: ${TESTS_BINARY}
	@echo -e "${GREEN}Running functional tests${WHITE}"
	@hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

rebuild-functest: clean-test ${TESTS_BINARY}

clean-test:
	@rm -f ${TESTS_BINARY}

${TESTS_BINARY}: ${TESTS_SRC_FILES} ${TESTS_OUT_DIR}
	@echo -e "${GREEN}Building functional tests${WHITE}"
	@${DO} hack/build/build-functest.sh

${TESTS_OUT_DIR}:
	@mkdir -p ${TESTS_OUT_DIR}

modules:
	@${DO} "GO111MODULE=on go mod tidy -v"

vendor:
	@${DO} "GO111MODULE=on go mod tidy -v"
	@${DO} "GO111MODULE=on go mod vendor -v"

build-dirs:
	@echo -e "${GREEN}Creating output directories${WHITE}"
	@hack/build/build-dirs.sh

stop-builder:
	@echo -n -e "${GREEN}Stopping builder...${WHITE}"
	@hack/build/stop-builder.sh > /dev/null
	@echo -e "${GREEN} done${WHITE}"

goveralls: test
	${DO} "TRAVIS_JOB_ID=${TRAVIS_JOB_ID} TRAVIS_PULL_REQUEST=${TRAVIS_PULL_REQUEST} TRAVIS_BRANCH=${TRAVIS_BRANCH} ./hack/build/goveralls.sh"

cluster-up:
	@cluster-up/up.sh

cluster-down:
	@cluster-up/down.sh

cluster-sync: local-deploy-velero remove-plugin cluster-push-image add-plugin
	@echo -e "${GREEN}Plugin redeployed${WHITE}"