

## This image's only task is to copy plugin executable.
## Therefore a minimal base image is used

FROM alpine:3.13
RUN mkdir /plugins
COPY bin/kubevirt-velero-plugin /plugins/
USER nobody:nogroup
ENTRYPOINT ["/bin/sh", "-c", "cp /plugins/* /target/."]
