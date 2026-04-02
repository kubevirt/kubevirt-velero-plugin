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
 * Copyright The KubeVirt Velero Plugin Authors.
 *
 */

package plugin

import (
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/nativebackup"
)

// VMDeleteItemAction cleans up native backup artifacts when a Velero backup is deleted.
type VMDeleteItemAction struct {
	log logrus.FieldLogger
}

// NewVMDeleteItemAction instantiates a VMDeleteItemAction.
func NewVMDeleteItemAction(log logrus.FieldLogger) *VMDeleteItemAction {
	return &VMDeleteItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMDeleteItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"VirtualMachine",
		},
	}, nil
}

// Execute cleans up native backup CRs and scratch PVCs when a Velero backup is deleted.
func (p *VMDeleteItemAction) Execute(input *velero.DeleteItemActionExecuteInput) error {
	p.log.Info("Executing VMDeleteItemAction")

	if input == nil || input.Item == nil {
		return nil
	}

	metadata, err := meta.Accessor(input.Item)
	if err != nil {
		return nil
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		return nil
	}

	// Check if this VM used native backup
	if annotations[nativebackup.BackupUsedAnnotation] != "true" {
		return nil
	}

	crRef := annotations[nativebackup.BackupCRAnnotation]
	if crRef == "" {
		return nil
	}

	ns, name := nativebackup.ParseOperationID(crRef)
	if ns == "" || name == "" {
		return nil
	}

	p.log.Infof("Cleaning up native backup artifacts for VM %s/%s (CR: %s)",
		metadata.GetNamespace(), metadata.GetName(), crRef)

	// Delete VirtualMachineBackup CR if it still exists
	if err := nativebackup.DeleteVMBackupCR(ns, name); err != nil {
		p.log.WithError(err).Warn("Failed to delete VirtualMachineBackup CR during backup deletion")
	}

	// Delete orphaned scratch PVCs
	if err := nativebackup.CleanupScratchPVCsByBackup(ns, name); err != nil {
		p.log.WithError(err).Warn("Failed to clean up scratch PVCs during backup deletion")
	}

	// Garbage collect stale scratch PVCs in this namespace
	if err := nativebackup.GarbageCollectStaleScratchPVCs(ns, p.log); err != nil {
		p.log.WithError(err).Warn("Failed to garbage collect stale scratch PVCs")
	}

	return nil
}

// Ensure VMDeleteItemAction implements the interface at compile time
var _ velero.DeleteItemAction = &VMDeleteItemAction{}

// isNativeBackupVM checks if a runtime.Unstructured VM has native backup annotations
func isNativeBackupVM(item runtime.Unstructured) bool {
	metadata, err := meta.Accessor(item)
	if err != nil {
		return false
	}
	annotations := metadata.GetAnnotations()
	return annotations != nil && annotations[nativebackup.BackupUsedAnnotation] == "true"
}
