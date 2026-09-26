## This image's only task is to copy plugin executable.
## Therefore a minimal base image is used

FROM alpine:3.13
ARG TARGETOS
ARG TARGETARCH
# no RUN steps: foreign-arch images must build without emulation
COPY ${TARGETOS}/${TARGETARCH}/kubevirt-velero-plugin /plugins/
USER nobody:nogroup
ENTRYPOINT ["/bin/sh", "-c", "cp /plugins/* /target/."]
