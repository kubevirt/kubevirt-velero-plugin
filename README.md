# Kubevirt Velero Plugin

This repository contains a Velero plugin. Thanks to the plugin, [Velero](https://velero.io/) can correctly backup and restore
VirtualMachines, DataVolumes and other resources managed by [KubeVirt](https://kubevirt.io/) and 
[CDI](https://github.com/kubevirt/containerized-data-importer/).

For more information on Velero check https://velero.io/.

## Plugin actions Included

The plugin registers backup and restore actions that operate on following resources: DataVolume, PersistentVolumeClaim, Pod, VirtualMachine, VirtualMachineInstance, VirtualMachineTemplate, VirtualMachineTemplateRequest.

### **DVBackupItemAction** 
An action that backs up the `PersistentVolumeClaim` and `DataVolume`
 
Finds the PVC for DV and adds the `"cdi.kubevirt.io/storage.prePopulated" or "cdi.kubevirt.io/storage.populatedFor"` annotations

### **VMBackupItemAction** 
An action that backs up the `VirtualMachine`
 
It checks if a `VM` can be safely backed up and if the backup contains all required objects for the successful restore. 
The action also makes sure that every object in the vm and VMI graph will be added to the backup, for example: instancetype, different types of volumes, access credentials, etc.
It also returns the underlying `DataVolume` if a VM has `DataVolumeTemplate` and `virtualmachineinstances` as extra items to back up.

> Note: any cluster scoped objects and network objects and configurations are not backed up and they should be available when restoring the VM.

### **VMIBackupItemAction** 
An action that backs up the `VirtualMachineInstance`
 
It checks if a `VMI` can be safely backed up and if the backup contains all required objects for the successful restore.
The action also returns the underlying VM volumes (`DataVolume` and `PersistentVolumeClaim`) and launcher `pod` as extra items to back up.

### **VMRestoreItemAction**
An action that restores the `VirtualMachine`
 
Adds a `datavolumes` to list of restored items.

### **VMIRestoreItemAction** 
An action that restores the `VirtualMachineInstance`

Skips the VMI if owned by a VM. The plugin also clears restricted labels, so the VMI is not rejected by kubevirt.  The restricted labels contain runtime information about the underlying KVM object.

### **PodRestoreItemAction**
An action that handles the virt-launcher `Pod`. It makes sure virt-launcher pod is always skipped.

### **VMTBackupItemAction**
An action that backs up a [`VirtualMachineTemplate`](https://github.com/kubevirt/virt-template).

A `VirtualMachineTemplate` holds an unprocessed `VirtualMachine` with `${PARAMETER}` placeholders, plus, when created
from an existing VM, one or more golden-image `DataVolumes` that back its `dataVolumeTemplates`. The action backs up
those golden-image `DataVolumes`/`PersistentVolumeClaims`/`VolumeSnapshots`/`DataSources`, along with any
non-parameterized secrets, config maps, service accounts and `NetworkAttachmentDefinitions` referenced by the
embedded `VirtualMachine`. References that are still parameter placeholders cannot be resolved to a concrete object
and are skipped.

### **VMTItemBlockAction**
An item block action for `VirtualMachineTemplate`, using the same object graph as `VMTBackupItemAction` to group
a template with its golden images for backup.

### **VMTRestoreItemAction**
An action that restores a `VirtualMachineTemplate`. It rewrites hardcoded (non-parameterized) namespace references
embedded in the template's `VirtualMachine` (`dataVolumeTemplates` sources and Multus network names) according to
the restore's namespace mapping, since Velero's own namespace remapping only touches the `VirtualMachineTemplate`'s
metadata, not references embedded inside its opaque spec. The embedded `VirtualMachine`'s own `metadata.namespace`
is left alone, because virt-template strips a hardcoded one when it processes the template.

### **VMTRRestoreItemAction**
An action that always skips restoring `VirtualMachineTemplateRequest`. A request is a one-shot job that snapshots
a source VM and produces a `VirtualMachineTemplate`; restoring it would cause it to re-run against the (possibly
different) restored source VM. The `VirtualMachineTemplate` it already produced is restored on its own.

## Compatibility

Plugin versions and respective Velero, KubeVirt, and CDI versions that are tested to be compatible.

| Plugin Version      | Velero Version | KubeVirt Version | CDI Version  |
|---------------------|----------------|------------------|--------------|
| v0.9.x              | v1.18.x        | >= v1.1.0        | >= v1.57.0   |
| v0.8.x              | v1.16.x        | >= v1.1.0        | >= v1.57.0   |
| v0.7.x              | v1.14.x        | >= v1.1.0        | >= v1.57.0   |
| v0.6.x              | v1.12.x        | >= v1.0.0        | >= v1.57.0   |

### Backup Methods

This plugin has been tested with the following Velero backup methods:

- **CSI volume snapshots** — using the [Velero CSI plugin](https://velero.io/docs/main/csi/)
- **CSI volume snapshots with DataMover** — for moving snapshots to remote storage using [CSI Snapshot Data Movement](https://velero.io/docs/main/csi-snapshot-data-movement/)

Other backup methods, such as file system backup (Kopia/Restic) or native cloud provider snapshots,
have **not been tested** with this plugin and may not work correctly with KubeVirt volumes.

### Known limitations

- **Restored `VirtualMachineTemplate` golden-image `DataVolumes` lose their owner reference.** virt-template makes a
  template's golden-image `DataVolumes` children of the `VirtualMachineTemplate`, but Velero clears `ownerReferences`
  on every restored object. The restored `DataVolumes` remain fully usable, but are no longer garbage-collected
  together with the template that owns them.
- **`VirtualMachineTemplates` using the non-string `${{PARAMETER}}` syntax are not fully processed.** That syntax
  deliberately stores a string where the `VirtualMachine` schema expects another type (for example
  `cpu.cores: ${{COUNT}}`), so the embedded `VirtualMachine` cannot be decoded. Such templates are still backed up
  and restored intact, but their golden images are not discovered as extra items and their embedded namespace
  references are not rewritten on a namespace-mapped restore. The plugin logs a warning when this happens.

## Install

To install the plugin check current velero documentation https://velero.io/docs/main/overview-plugins/.
Below example for kubevirt-velero-plugin version v0.9.0 on Velero 1.18.0

```bash
velero plugin add quay.io/kubevirt/kubevirt-velero-plugin:v0.9.0
```

## Backup/Restore Virtual Machines Using the Plugin

When the plugin is deployed it is already effective. There is nothing special required to make it work.
### 1. Create a Virtual Machine

```bash
kubectl create namespace demo
kubectl create -f example/datavolume.yaml -n demo
kubectl create -f example/vm.yaml -n demo
```
Wait for Vm running and with condition `AgentConnected`. Then login and add some data:
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
