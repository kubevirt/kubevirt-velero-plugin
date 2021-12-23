# Kubevirt Velero Plugin

This repository contains Velero plugins. Thanks to this plugin, [Velero](https://velero.io/) can correctly backup and restore
VirtualMachines, DataVolumess and other resources managed by [KubeVirt](https://kubevirt.io/) and 
[CDI](https://github.com/kubevirt/containerized-data-importer/).

For more information on Velero check https://velero.io/.

This is still a work in progress, not intended for production use.

## Kinds of Plugins Included

Kubevirt Velero Plugin implements two kinds of plugins:
- **Backup Item Action** - performs arbitrary logic on individual items prior to storing them in the backup file.
- **Restore Item Action** - performs arbitrary logic on individual items prior to restoring them in the Kubernetes cluster.

The plugin operates on following resources: DataVolume, PersistentVolumeClaim, Pod, VirtualMachine, VirtualMachineInstance
 
## Compatibility

Plugin versions and respective Velero/Kubevirt/CDI versions that are tested to be compatible.

| Plugin Version  | Velero Version | Kubevirt Version | CDI Version  |
|-----------------|----------------|------------------|--------------|
| v0.2.0          | v1.6.x, v1.7.x | v0.48.x          | \>= v1.37.0  |

## Install

To install the plugin check current velero documentation https://velero.io/docs/v1.7/overview-plugins/.
Below example for kubevirt-velero-plugin version v0.2.0 on Velero 1.7.0

```bash
velero plugin add quay.io/kubevirt/kubevirt-velero-plugin:v0.2.0
```

## Backup/Restore Virtual Machines Using the Plugin

When the plugin is deployed it is already effective. There is nothing special required to make it work.
### 1. Create a Virtual Machine

```bash
kubectl create namespace demo
kubectl create -f example/datavolume.yaml -n demo
kubectl create -f example/vm.yaml -n demo
```
Start a VM (wait for condition AgentConnected):
```bash
virtctl start example-vm -n demo`
```
Login and add some data:
`virtctl console example-vm -n demo`
### 2. Backup

`./velero backup create demobackup1 --include-namespaces demo --wait`

### 3. Destroy something
```bash
kubectl delete vm example-vm -n demo
kubectl delete dv example-vm -n demo
```
Try to login, to find a vm a dv or a pvc
`virtctl console example-vm -n demo`
### 4. Restore

`./velero restore create --from-backup demobackup1 --wait`

The [velero-example](https://github.com/konveyor/velero-examples) repository contains some basic examples of backup/restore using Velero.

## Building the plugins

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
