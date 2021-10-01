# Kubevirt Velero Plugin

Velero is the preferred solution for OpenShift for backup, disaster recovery
and migration. Out-of-the-box it allows backing up VMs and DVs in a couple
limited scenarios and requires jumping through several hoops.

The plugin will ensure full support as well as automation of the abovementioned
hoop-jumping. ;-)

This is still a work in progress, not intended for production use.

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
