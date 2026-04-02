/*
 * This file is part of the Kubevirt Velero Plugin project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2021 Red Hat, Inc.
 *
 */

package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/nativebackup"
)

// VMBackupItemAction is a v2 backup item action for backing up VirtualMachines.
// It supports both the traditional CSI snapshot path and the native KubeVirt backup API
// (backup.kubevirt.io/v1alpha1) for CBT/incremental backup when available and enabled.
type VMBackupItemAction struct {
	log logrus.FieldLogger
}

// NewVMBackupItemAction instantiates a VMBackupItemAction.
func NewVMBackupItemAction(log logrus.FieldLogger) *VMBackupItemAction {
	return &VMBackupItemAction{log: log}
}

// Name returns the name of this BIA (required by v2 interface).
func (p *VMBackupItemAction) Name() string {
	return "kubevirt-velero-plugin/backup-virtualmachine-action"
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"VirtualMachine",
		},
	}, nil
}

// Execute backs up a VirtualMachine, optionally using the native KubeVirt backup API.
// Returns:
//   - item: the (possibly mutated) VM
//   - additionalItems: resources to back up immediately
//   - operationID: non-empty if an async native backup was initiated
//   - postOperationItems: resources to back up after the async operation completes (scratch PVCs)
//   - error
func (p *VMBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, string, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMBackupItemAction (v2)")

	if backup == nil {
		return nil, nil, "", nil, fmt.Errorf("backup object nil!")
	}

	vm := new(kvcore.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vm); err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}

	safe, err := p.canBeSafelyBackedUp(vm, backup)
	if err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}
	if !safe {
		return nil, nil, "", nil, fmt.Errorf("VM cannot be safely backed up")
	}

	// Consistency checks (skip for metadata-only backups)
	if !util.IsMetadataBackup(backup) {
		skipVolume := func(volume kvcore.Volume) bool {
			return volumeInDVTemplates(volume, vm)
		}

		restore, err := util.RestorePossible(vm.Spec.Template.Spec.Volumes, backup, vm.Namespace, skipVolume, p.log)
		if err != nil {
			return nil, nil, "", nil, errors.WithStack(err)
		}
		if !restore {
			return nil, nil, "", nil, fmt.Errorf("VM would not be restored correctly")
		}
	}

	// Compute dependency graph (unchanged from v1)
	extra, err := kvgraph.NewVirtualMachineBackupGraph(vm)
	if err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}

	// Preserve instancetype/preference controller revisions (unchanged from v1)
	if vm.Spec.Instancetype != nil && vm.Status.InstancetypeRef != nil && vm.Status.InstancetypeRef.ControllerRevisionRef != nil {
		vm.Spec.Instancetype.RevisionName = vm.Status.InstancetypeRef.ControllerRevisionRef.Name
	}
	if vm.Spec.Preference != nil && vm.Status.PreferenceRef != nil && vm.Status.PreferenceRef.ControllerRevisionRef != nil {
		vm.Spec.Preference.RevisionName = vm.Status.PreferenceRef.ControllerRevisionRef.Name
	}

	// Decide whether to use native backup
	if p.shouldUseNativeBackup(vm, backup) {
		return p.executeNativeBackup(vm, backup, item, extra)
	}

	// CSI path: sync return (no operationID, no postItems)
	vmMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}
	return &unstructured.Unstructured{Object: vmMap}, extra, "", nil, nil
}

// Progress reports on the status of an async native backup operation.
func (p *VMBackupItemAction) Progress(operationID string, backup *v1.Backup) (velero.OperationProgress, error) {
	if operationID == "" {
		return velero.OperationProgress{}, fmt.Errorf("empty operation ID")
	}
	return nativebackup.CheckProgress(operationID, p.log)
}

// Cancel cancels an in-progress native backup and cleans up resources.
func (p *VMBackupItemAction) Cancel(operationID string, backup *v1.Backup) error {
	if operationID == "" {
		return nil
	}
	return nativebackup.CancelAndCleanup(operationID, p.log)
}

// shouldUseNativeBackup determines if native KubeVirt backup should be used
func (p *VMBackupItemAction) shouldUseNativeBackup(vm *kvcore.VirtualMachine, backup *v1.Backup) bool {
	// Must be explicitly enabled via label
	if !nativebackup.IsEnabled(backup) {
		return false
	}

	// VM must be running (native backup requires QEMU/libvirt)
	if !vm.Status.Created {
		p.log.Infof("VM %s/%s is not running, falling back to CSI snapshot", vm.Namespace, vm.Name)
		return false
	}

	// CRD must be installed
	if !nativebackup.IsAvailable() {
		p.log.Info("VirtualMachineBackup CRD not available, falling back to CSI snapshot")
		return false
	}

	// No async operations during Finalize phase
	if backup.Status.Phase == v1.BackupPhaseFinalizing ||
		backup.Status.Phase == v1.BackupPhaseFinalizingPartiallyFailed {
		p.log.Info("Backup in Finalize phase, skipping native backup")
		return false
	}

	return true
}

// executeNativeBackup initiates a native KubeVirt backup and returns async operation info
func (p *VMBackupItemAction) executeNativeBackup(
	vm *kvcore.VirtualMachine,
	backup *v1.Backup,
	item runtime.Unstructured,
	additionalItems []velero.ResourceIdentifier,
) (runtime.Unstructured, []velero.ResourceIdentifier, string, []velero.ResourceIdentifier, error) {

	crName := nativebackup.BackupCRName(backup.Name, vm.Name)
	operationID := fmt.Sprintf("%s/%s", vm.Namespace, crName)

	// Filter to persistent volumes only
	persistentVolumes := nativebackup.FilterPersistentVolumes(vm.Spec.Template.Spec.Volumes)
	if len(persistentVolumes) == 0 {
		p.log.Infof("No persistent volumes on VM %s/%s, using CSI path", vm.Namespace, vm.Name)
		vmMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
		if err != nil {
			return nil, nil, "", nil, errors.WithStack(err)
		}
		return &unstructured.Unstructured{Object: vmMap}, additionalItems, "", nil, nil
	}

	// Idempotency: check if CR already exists (Velero retry case)
	exists, err := nativebackup.VMBackupExists(vm.Namespace, crName)
	if err != nil {
		p.log.WithError(err).Warn("Error checking existing backup CR, falling back to CSI")
		return p.fallbackToCSI(vm, additionalItems)
	}

	if exists {
		p.log.Infof("VirtualMachineBackup %s already exists (retry), reusing operation", crName)
		util.AddAnnotation(item, nativebackup.BackupUsedAnnotation, "true")
		util.AddAnnotation(item, nativebackup.BackupCRAnnotation, operationID)

		// Still need to return the scratch PVC as postOperationItem
		scratchName := nativebackup.ScratchPVCName(backup.Name, vm.Name)
		postItems := []velero.ResourceIdentifier{{
			GroupResource: schema.GroupResource{Resource: "persistentvolumeclaims"},
			Namespace:     vm.Namespace,
			Name:          scratchName,
		}}

		// Exclude native-backed PVCs from additional items
		additionalItems = nativebackup.ExcludeNativeBackedPVCs(additionalItems, persistentVolumes)

		vmMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
		if err != nil {
			return nil, nil, "", nil, errors.WithStack(err)
		}
		return &unstructured.Unstructured{Object: vmMap}, additionalItems, operationID, postItems, nil
	}

	// Create scratch PVC
	scratchName, err := nativebackup.CreateScratchPVC(vm, backup, p.log)
	if err != nil {
		p.log.WithError(err).Warn("Scratch PVC creation failed, falling back to CSI")
		return p.fallbackToCSI(vm, additionalItems)
	}

	// Resolve backup source (VM for full, Tracker for incremental)
	source, forceFullBackup, err := nativebackup.ResolveSource(vm, backup, p.log)
	if err != nil {
		_ = nativebackup.CleanupScratchPVC(scratchName, vm.Namespace)
		p.log.WithError(err).Warn("Source resolution failed, falling back to CSI")
		return p.fallbackToCSI(vm, additionalItems)
	}

	// Detect guest agent for quiesce decision
	skipQuiesce := nativebackup.ShouldSkipQuiesce(vm, backup, p.log)

	// Create VirtualMachineBackup CR
	err = nativebackup.CreateVMBackupCR(nativebackup.CreateParams{
		Name:            crName,
		Namespace:       vm.Namespace,
		Source:          source,
		PVCName:         scratchName,
		Mode:            "Push",
		SkipQuiesce:     skipQuiesce,
		ForceFullBackup: forceFullBackup,
	})
	if err != nil {
		_ = nativebackup.CleanupScratchPVC(scratchName, vm.Namespace)
		p.log.WithError(err).Warn("VirtualMachineBackup CR creation failed, falling back to CSI")
		return p.fallbackToCSI(vm, additionalItems)
	}

	p.log.Infof("Initiated native backup %s for VM %s/%s (source: %s/%s, skipQuiesce: %v, forceFullBackup: %v)",
		crName, vm.Namespace, vm.Name, source.Kind, source.Name, skipQuiesce, forceFullBackup)

	// Annotate VM with native backup metadata
	util.AddAnnotation(item, nativebackup.BackupUsedAnnotation, "true")
	util.AddAnnotation(item, nativebackup.BackupCRAnnotation, operationID)
	util.AddAnnotation(item, nativebackup.TrackerAnnotation, nativebackup.TrackerName(vm.Name))
	util.AddAnnotation(item, nativebackup.BackupVolumesAnnotation,
		strings.Join(nativebackup.GetVolumeClaimNames(persistentVolumes), ","))

	// Scratch PVC as postOperationItem — Velero snapshots it AFTER native backup completes
	postItems := []velero.ResourceIdentifier{{
		GroupResource: schema.GroupResource{Resource: "persistentvolumeclaims"},
		Namespace:     vm.Namespace,
		Name:          scratchName,
	}}

	// Exclude native-backed PVCs from additionalItems (prevent CSI double-snapshot)
	additionalItems = nativebackup.ExcludeNativeBackedPVCs(additionalItems, persistentVolumes)

	vmMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}
	return &unstructured.Unstructured{Object: vmMap}, additionalItems, operationID, postItems, nil
}

// fallbackToCSI returns the standard CSI snapshot path (sync, no native backup)
func (p *VMBackupItemAction) fallbackToCSI(
	vm *kvcore.VirtualMachine,
	additionalItems []velero.ResourceIdentifier,
) (runtime.Unstructured, []velero.ResourceIdentifier, string, []velero.ResourceIdentifier, error) {
	vmMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, nil, "", nil, errors.WithStack(err)
	}
	return &unstructured.Unstructured{Object: vmMap}, additionalItems, "", nil, nil
}

// canBeSafelyBackedUp returns false for cases when backup might end up with a broken PVC snapshot
func (p *VMBackupItemAction) canBeSafelyBackedUp(vm *kvcore.VirtualMachine, backup *v1.Backup) (bool, error) {
	isRunning := vm.Status.PrintableStatus == kvcore.VirtualMachineStatusStarting || vm.Status.PrintableStatus == kvcore.VirtualMachineStatusRunning
	if !isRunning {
		return true, nil
	}

	if !util.IsResourceInBackup("virtualmachineinstances", backup) {
		p.log.Info("Backup of a running VM does not contain VMI.")
		return false, nil
	}

	excluded, err := isVMIExcludedByLabel(vm)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if excluded {
		p.log.Info("VM is running but VMI is not included in the backup")
		return false, nil
	}

	if !util.IsResourceInBackup("pods", backup) && util.IsResourceInBackup("persistentvolumeclaims", backup) {
		p.log.Info("Backup of a running VM does not contain Pod but contains PVC")
		return false, nil
	}

	return true, nil
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var isVMIExcludedByLabel = func(vm *kvcore.VirtualMachine) (bool, error) {
	client, err := util.GetKubeVirtclient()
	if err != nil {
		return false, err
	}

	vmi, err := (*client).VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	labels := vmi.GetLabels()
	if labels == nil {
		return false, nil
	}

	label, ok := labels[util.VeleroExcludeLabel]
	return ok && label == "true", nil
}

func volumeInDVTemplates(volume kvcore.Volume, vm *kvcore.VirtualMachine) bool {
	for _, template := range vm.Spec.DataVolumeTemplates {
		if template.Name == volume.VolumeSource.DataVolume.Name {
			return true
		}
	}

	return false
}
