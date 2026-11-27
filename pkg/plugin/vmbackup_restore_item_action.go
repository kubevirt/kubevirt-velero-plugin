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
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// VMBackupRestoreItemAction is a restore item action for KubeVirt changed block tracking
// backup resources: VirtualMachineBackups and VirtualMachineBackupTrackers.
type VMBackupRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewVMBackupRestoreItemAction instantiates a VMBackupRestoreItemAction.
func NewVMBackupRestoreItemAction(log logrus.FieldLogger) *VMBackupRestoreItemAction {
	return &VMBackupRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMBackupRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"virtualmachinebackups.backup.kubevirt.io",
			"virtualmachinebackuptrackers.backup.kubevirt.io",
		},
	}, nil
}

// Execute – VirtualMachineBackups and VirtualMachineBackupTrackers are created by a
// changed block tracking backup application to drive and track its own backups. They
// describe backup state at the time the Velero backup was taken, so restoring them would
// start a new backup or leave the tracker pointing at checkpoints that no longer match
// the restored disks. They are therefore always skipped on restore.
func (p *VMBackupRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running VMBackupRestoreItemAction")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	return velero.NewRestoreItemActionExecuteOutput(input.Item).WithoutRestore(), nil
}
