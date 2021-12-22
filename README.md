# Kubevirt Velero Plugin

This repository contains Velero plugins. Thanks to this plugin, velero can correctly backup and restore 
VMs, DVs and other resources managed by kubevirt - https://kubevirt.io/. 

For more information on Velero check https://velero.io/.

This is still a work in progress, not intended for production use.

## Install

To install remove the plugin check current velero documentation https://velero.io/docs/v1.7/overview-plugins/.
Below example for kubevirt-velero-plugin version v0.2.0 on Velero 1.7.0

```bash
velero plugin add quay.io/kubevirt/kubevirt-velero-plugin:v0.2.0
```
## Compatibility 

Plugin versions and respective Velero/Kubevirt/CDI versions that are tested to be compatible.

| Plugin Version  | Velero Version | Kubevirt Version | CDI Version   |
|-----------------|----------------|------------------|---------------|
| v0.2.0          | v1.6.x, v1.7.x | v0.48.x          | \>= v1.37.0 |

## Building the plugins

To build the plugin's builder, run

```bash
$ make build-builder
```

To build the plugin, run

```bash
$ make build-all
```

To build the image, run

```bash
$ make build-image
```

This builds an image tagged as `registry:5000/kubevirt-velero-plugin:0.1`. If you want to specify a different name or version/tag, run:

```bash
$ DOCKER_PREFIX=your-repo/your-name DOCKER_TAG=your-version-tag make build-image
```

## Deploying the plugin to local cluster

Development version of the plugin is intended to work in local cluster build with KubeVirt's or CDI's `make cluster-up`.
To deploy the plugin:


1. `make cluster-push-image` to build and push image to local cluster
2. `make local-deploy-velero` to deploy Velero to local cluster
3. `make add-plugin` to add the plugin to Velero.
