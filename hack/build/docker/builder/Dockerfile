FROM registry.fedoraproject.org/fedora-minimal:32
## Fedora version cannot be higher until the runtime image for Velero
## is not upgraded. Velero runs on ubuntu:focal which has glibc 2.31
## while fedora >=33 includes glibc 2.32.

LABEL maintainer="The KubeVirt Project <kubevirt-dev@googlegroups.com>"

COPY fedora.repo /tmp/fedora_ci.dnf.repo
RUN sed -i 's/proxy = None//gI' /tmp/fedora_ci.dnf.repo && \
    cat /tmp/fedora_ci.dnf.repo && \
    mkdir /etc/yum.repos.d/old && \
	mv /etc/yum.repos.d/*.repo /etc/yum.repos.d/old  && \
	mv /tmp/fedora_ci.dnf.repo /etc/yum.repos.d/fedora.repo && \
	microdnf update -y && microdnf install -y \
	diffutils \
	git \
	gcc \
	gcc-c++ \
	glibc-devel \
	golang \
	findutils \
	autoconf \
	automake \
	libtool \
	jq \
	rsync-daemon \
	rsync \
	patch \
	unzip \
	buildah \
	crun \
	&& microdnf clean all && \
	mv /etc/yum.repos.d/old/* /etc/yum.repos.d/ && \
	rmdir /etc/yum.repos.d/old

ENV GIMME_GO_VERSION=1.16.10 GOPATH="/go" KUBEBUILDER_VERSION="2.3.1" ARCH="amd64" GO111MODULE="on"

RUN mkdir -p /gimme && curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | HOME=/gimme bash >> /etc/profile.d/gimme.sh

RUN \
    . /etc/profile.d/gimme.sh && \
    eval $(go env) && \
    go get github.com/onsi/ginkgo/ginkgo && \
    go get golang.org/x/tools/cmd/goimports && \
    go get mvdan.cc/sh/cmd/shfmt && \
    go get github.com/mattn/goveralls && \
    go get -u golang.org/x/lint/golint && \
    go get github.com/rmohr/go-swagger-utils/swagger-doc && \
    go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 && \
    go get github.com/securego/gosec/v2/cmd/gosec@0ce48a5 && \
	go get github.com/onsi/ginkgo/ginkgo && \
	go get github.com/onsi/gomega/... && \
    rm -rf "${GOPATH}/pkg"

ADD entrypoint.sh /entrypoint.sh
COPY rsyncd.conf /etc/rsyncd.conf
RUN \
    sed 's/#mount_program = "\/usr\/bin\/fuse-overlayfs"/mount_program = "\/usr\/bin\/fuse-overlayfs"/' /etc/containers/storage.conf > /etc/containers/storage.conf.new && \
    cp /etc/containers/storage.conf.new /etc/containers/storage.conf
ENV BUILDAH_ISOLATION=chroot \
    PATH="/go/bin:${PATH}"

ENTRYPOINT ["/entrypoint.sh"]
