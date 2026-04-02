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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/restoreitemaction/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/nativebackup"
)

// VMRestorePlugin is a v2 VM restore item action plugin for Velero.
// It supports AreAdditionalItemsReady to wait for PVCs to be bound before
// the VM is created by the API server.
type VMRestorePlugin struct {
	log logrus.FieldLogger
}

// NewVMRestoreItemAction instantiates a RestorePlugin.
func NewVMRestoreItemAction(log logrus.FieldLogger) *VMRestorePlugin {
	return &VMRestorePlugin{log: log}
}

// Name returns the name of this RIA (required by v2 interface).
func (p *VMRestorePlugin) Name() string {
	return "kubevirt-velero-plugin/restore-vm-action"
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMRestorePlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"VirtualMachine",
		},
	}, nil
}

// Execute restores a VirtualMachine with optional modifications:
// - Run strategy override
// - MAC address clearing
// - Firmware UUID regeneration
// - Native backup volume remapping
func (p *VMRestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running VMRestorePlugin (v2)")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	vm := new(kvcore.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), vm); err != nil {
		return nil, errors.WithStack(err)
	}

	if runStrategy, ok := util.GetRestoreRunStrategy(input.Restore); ok {
		p.log.Infof("Setting virtual machine run strategy to %s", runStrategy)
		vm.Spec.RunStrategy = ptr.To(runStrategy)
		vm.Spec.Running = nil
	}

	if util.ShouldClearMacAddress(input.Restore) {
		p.log.Info("Clear virtual machine MAC addresses")
		util.ClearMacAddress(&vm.Spec.Template.Spec)
	}

	if util.ShouldGenerateNewFirmwareUUID(input.Restore) {
		p.log.Info("Generate new firmware UUID")
		util.GenerateNewFirmwareUUID(&vm.Spec.Template.Spec, vm.Name, vm.Namespace, string(vm.UID))
	}

	// Handle native backup volume remapping
	if ann := vm.GetAnnotations(); ann != nil && ann[nativebackup.BackupUsedAnnotation] == "true" {
		p.log.Info("VM was backed up using native KubeVirt backup, annotations preserved for restore")
		// The scratch PVC is restored by Velero with its original name.
		// Volume references already point to the correct PVCs in the backup.
		// Clean up native backup annotations to avoid confusion on the restored VM.
		delete(ann, nativebackup.BackupUsedAnnotation)
		delete(ann, nativebackup.BackupCRAnnotation)
		delete(ann, nativebackup.BackupTypeAnnotation)
		delete(ann, nativebackup.BackupCheckpointAnnotation)
		delete(ann, nativebackup.BackupVolumesAnnotation)
		delete(ann, nativebackup.TrackerAnnotation)
		vm.SetAnnotations(ann)
	}

	item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	output := velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: item})
	output.AdditionalItems, err = kvgraph.NewVirtualMachineRestoreGraph(vm)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// v2: Wait for PVCs to be ready before VM is created
	if len(output.AdditionalItems) > 0 {
		output.WaitForAdditionalItems = true
		output.AdditionalItemsReadyTimeout = 10 * time.Minute
	}

	return output, nil
}

// AreAdditionalItemsReady checks if all PVCs in the additional items are bound.
// This prevents the VM from being created before its storage is ready.
func (p *VMRestorePlugin) AreAdditionalItemsReady(
	additionalItems []velero.ResourceIdentifier,
	restore *v1.Restore,
) (bool, error) {
	for _, item := range additionalItems {
		if item.Resource != "persistentvolumeclaims" {
			continue
		}

		pvc, err := util.GetPVC(item.Namespace, item.Name)
		if err != nil {
			p.log.Infof("PVC %s/%s not yet available: %v", item.Namespace, item.Name, err)
			return false, nil
		}

		if pvc.Status.Phase != corev1.ClaimBound {
			p.log.Infof("PVC %s/%s not yet bound (phase: %s)", item.Namespace, item.Name, pvc.Status.Phase)
			return false, nil
		}
	}

	p.log.Info("All additional PVCs are bound, VM restore can proceed")
	return true, nil
}

// Progress reports on async restore operations (not used for VM restore).
func (p *VMRestorePlugin) Progress(operationID string, restore *v1.Restore) (velero.OperationProgress, error) {
	return velero.OperationProgress{}, v2.AsyncOperationsNotSupportedError()
}

// Cancel cancels an async restore operation (not used for VM restore).
func (p *VMRestorePlugin) Cancel(operationID string, restore *v1.Restore) error {
	return nil
}
