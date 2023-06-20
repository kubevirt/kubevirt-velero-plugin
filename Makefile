# Copyright 2021 The Kubevirt Velero Plugin Authors.
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
	build-builder \
	build-image \
	build-dirs \
	push \
	cluster-push-image \
	test \
	modules \
	vendor \
	gomod-update \
	build-local \
	tests-local \
	test-functional-local \
	local-deploy-velero \
	add-plugin \
	remove-plugin \
	local-undeploy-velero \
	cluster-up \
	cluster-down \
	cluster-sync \
	test-functional \
	rebuild-functest \
	clean-test

GREEN=\e[0;32m
WHITE=\e[0;37m
BIN=kubevirt-velero-plugin
SRC_FILES=main.go \
	$(shell find pkg -name "*.go")

TESTS_OUT_DIR=_output/tests
TESTS_BINARY=_output/tests/tests.test
TESTS_SRC_FILES=\
	$(shell find tests -name "*.go")

DOCKER_PREFIX?=kubevirt
DOCKER_TAG?=latest
IMAGE_NAME?=kubevirt-velero-plugin

# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64

platform_temp = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(platform_temp))
GOARCH = $(word 2, $(platform_temp))

# registry prefix is the prefix usable from inside the local cluster
KUBEVIRTCI_REGISTRY_PREFIX=registry:5000/kubevirt
PORT=$(shell ./cluster-up/cli.sh ports registry)

BUILD_IMAGE ?= quay.io/konveyor/builder:v1.18
OCI_BIN ?= $(shell if podman ps >/dev/null 2>&1; then echo podman; elif docker ps >/dev/null 2>&1; then echo docker; fi)
TLS_SETTING := $(if $(filter $(OCI_BIN),podman),--tls-verify=false,)
export OCI_BIN
export TLS_SETTING

all: build-image

build-local: build-dirs
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	PKG=$(PKG) \
	BIN=$(BIN) \
	GIT_SHA=$(GIT_SHA) \
	GIT_DIRTY="$(GIT_DIRTY)" \
	OUTPUT_DIR=$$(pwd)/_output/bin/$(GOOS)/$(GOARCH) \
	GO111MODULE=on \
	GOFLAGS=-mod=readonly \
	./hack/build/build.sh

build-all: build-dirs _output/bin/$(GOOS)/$(GOARCH)/$(BIN)

build-builder:
	@echo "deprecated"

_output/bin/$(GOOS)/$(GOARCH)/$(BIN): build-dirs ${SRC_FILES}
	@echo -e "${GREEN}Building...${WHITE}"
	@echo "Building: $@"
	$(MAKE) shell CMD="-c '\
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		PKG=$(PKG) \
		BIN=$(BIN) \
		GIT_SHA=$(GIT_SHA) \
		GIT_DIRTY="$(GIT_DIRTY)" \
		OUTPUT_DIR=/output/$(GOOS)/$(GOARCH) \
		GO111MODULE=on \
 		GOFLAGS=-mod=readonly \
 		GOCACHE=/.cache/go-build \
 		GOPATH=/go \
		./hack/build/build.sh'"

TTY := $(shell tty -s && echo "-t")
PODMAN_AND_ROOTLESS := $(shell $(OCI_BIN) info -f '{{json .Host.Security.Rootless}}' 2>/dev/null ; true)
PODMAN_SPECIFIC_FLAG := $(if $(filter $(PODMAN_AND_ROOTLESS),true),--userns=keep-id,)

shell: build-dirs
	@echo "running docker: $@"
	@${OCI_BIN} run \
		-e GOFLAGS \
		-i $(TTY) \
		--rm \
		-u $$(id -u):$$(id -g) \
		$(PODMAN_SPECIFIC_FLAG) \
		-v "$$(pwd)/_output/bin:/output:delegated" \
		-v $$(pwd)/.go/pkg:/go/pkg \
		-v $$(pwd)/.go/src:/go/src \
		-v $$(pwd)/.go/std:/go/std \
		-v $$(pwd)/.go/bin:/go/bin \
		-v $$(pwd):/go/src/kubevirt-velero-plugin \
		-v $$(pwd)/.go/std/$(GOOS)_$(GOARCH):/usr/local/go/pkg/$(GOOS)_$(GOARCH)_static \
		-v "$$(pwd)/.go/go-build:/.cache/go-build:delegated" \
		-e CGO_ENABLED=0 \
		-w /go/src/kubevirt-velero-plugin \
		$(BUILD_IMAGE) \
		/bin/sh $(CMD)

build-dirs:
	@mkdir -p _output/bin/$(GOOS)/$(GOARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(GOOS)_$(GOARCH) .go/go-build

container-name:
	@echo "container: ${DOCKER_PREFIX}/${IMAGE_NAME}:${DOCKER_TAG}"

build-image: build-all
	@echo -e "${GREEN}Building plugin image${WHITE}"
	cp Dockerfile _output/bin/$(GOOS)/$(GOARCH)/Dockerfile
	${OCI_BIN} build -t ${DOCKER_PREFIX}/${IMAGE_NAME}:${DOCKER_TAG} -f _output/bin/$(GOOS)/$(GOARCH)/Dockerfile _output/bin/$(GOOS)/$(GOARCH)

push: build-image
	@echo -e "${GREEN}Pushing plugin image to local registry${WHITE}"
	@${OCI_BIN} push ${DOCKER_PREFIX}/${IMAGE_NAME}:${DOCKER_TAG}

gomod-update: modules vendor

clean-dirs:
	@echo -e "${GREEN}Removing output directories${WHITE}"
	rm -rf .container-* _output/.dockerfile-*
	rm -rf .go _output

clean: clean-dirs
	@echo "cleaning"
	${OCI_BIN} rmi $(BUILD_IMAGE)

test: build-dirs
	@echo -e "${GREEN}Testing${WHITE}"
	@$(MAKE) shell  CMD="-c 'CGO_ENABLED=0 go test -v -timeout 60s ./pkg/...'"

test-functional: ${TESTS_BINARY}
	@echo -e "${GREEN}Running functional tests${WHITE}"
	@hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

test-functional-local: tests-local
	@echo -e "${GREEN}Running functional tests${WHITE}"
	@hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

rebuild-functest: clean-test ${TESTS_BINARY}

clean-test:
	@rm -f ${TESTS_BINARY}

tests-local: build-dirs ${TESTS_SRC_FILES} ${TESTS_OUT_DIR}
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		PKG=$(PKG) \
        BIN=$(BIN) \
        GIT_SHA=$(GIT_SHA) \
        GIT_DIRTY="$(GIT_DIRTY)" \
		OUTPUT_DIR=/output/$(GOOS)/$(GOARCH) \
		GO111MODULE=on \
 		GOFLAGS=-mod=readonly \
 		TESTS_OUT_DIR=$(TESTS_OUT_DIR) \
 		JOB_TYPE="${JOB_TYPE:-}" \
		./hack/build/build-functest.sh

${TESTS_BINARY}: ${TESTS_SRC_FILES} ${TESTS_OUT_DIR}
	@echo -e "${GREEN}Building functional tests${WHITE}"
	$(MAKE) shell CMD="-c '\
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		PKG=$(PKG) \
        BIN=$(BIN) \
        GIT_SHA=$(GIT_SHA) \
        GIT_DIRTY="$(GIT_DIRTY)" \
		OUTPUT_DIR=/output/$(GOOS)/$(GOARCH) \
		GO111MODULE=on \
 		GOFLAGS=-mod=readonly \
 		GOCACHE=/.cache/go-build \
        GOPATH=/go/pkg/mod \
 		TESTS_OUT_DIR=$(TESTS_OUT_DIR) \
 		JOB_TYPE="${JOB_TYPE:-}" \
		./hack/build/build-functest.sh'"

${TESTS_OUT_DIR}:
	@mkdir -p ${TESTS_OUT_DIR}

modules:
	GO111MODULE=on go mod tidy -v

vendor:
	GO111MODULE=on go mod tidy -v
	GO111MODULE=on go mod vendor -v

goveralls: test
	${DO} "TRAVIS_JOB_ID=${TRAVIS_JOB_ID} TRAVIS_PULL_REQUEST=${TRAVIS_PULL_REQUEST} TRAVIS_BRANCH=${TRAVIS_BRANCH} ./hack/build/goveralls.sh"

# local test cluster targets
cluster-push-image: build-image
	@echo -e "${GREEN}Pushing plugin image to local K8s cluster${WHITE}"
	DOCKER_PREFIX=${DOCKER_PREFIX} IMAGE_NAME=${IMAGE_NAME} DOCKER_TAG=${DOCKER_TAG} PORT=${PORT} KUBEVIRTCI_REGISTRY_PREFIX=${KUBEVIRTCI_REGISTRY_PREFIX} \
	hack/build/cluster-push-image.sh


add-plugin: local-deploy-velero
	@echo -e "${GREEN}Adding the plugin to local Velero${WHITE}"
	IMAGE_NAME=${IMAGE_NAME} DOCKER_TAG=${DOCKER_TAG} DOCKER_PREFIX=${KUBEVIRTCI_REGISTRY_PREFIX} hack/velero/add-plugin.sh

remove-plugin:
	@echo -e "${GREEN}Removing the plugin from local Velero${WHITE}"
	IMAGE_NAME=${IMAGE_NAME} DOCKER_TAG=${DOCKER_TAG} DOCKER_PREFIX=${KUBEVIRTCI_REGISTRY_PREFIX} hack/velero/remove-plugin.sh

local-deploy-velero:
	@echo -e "${GREEN}Deploying velero to local cluster${WHITE}"
	@hack/velero/deploy-velero.sh

local-undeploy-velero:
	@echo -e "${GREEN}Removing velero from local cluster${WHITE}"
	@hack/velero/undeploy-velero.sh

cluster-up:
	@hack/cluster-up.sh
	@hack/cluster-deploy-prerequisites.sh

cluster-down:
	@cluster-up/down.sh

cluster-sync: local-undeploy-velero local-deploy-velero remove-plugin cluster-push-image add-plugin
	@echo -e "${GREEN}Plugin redeployed${WHITE}"
