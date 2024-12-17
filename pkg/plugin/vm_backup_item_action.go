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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// VMBackupItemAction is a backup item action for backing up DataVolumes
type VMBackupItemAction struct {
	log logrus.FieldLogger
}

// NewVMBackupItemAction instantiates a VMBackupItemAction.
func NewVMBackupItemAction(log logrus.FieldLogger) *VMBackupItemAction {
	return &VMBackupItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
// The IncludedResources and ExcludedResources slices can include both resources
// and resources with group names. These work: "ingresses", "ingresses.extensions".
// A VMBackupItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.
func (p *VMBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"VirtualMachine",
			},
		},
		nil
}

// Execute returns VM's DataVolumes as extra items to back up.
func (p *VMBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	vm := new(kvcore.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vm); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	safe, err := p.canBeSafelyBackedUp(vm, backup)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if !safe {
		return nil, nil, fmt.Errorf("VM cannot be safely backed up")
	}

	// we can skip all checks that ensure consistency
	// if we just want to backup for metadata purposes
	if !util.IsMetadataBackup(backup) {
		skipVolume := func(volume kvcore.Volume) bool {
			return volumeInDVTemplates(volume, vm)
		}

		restore, err := util.RestorePossible(vm.Spec.Template.Spec.Volumes, backup, vm.Namespace, skipVolume, p.log)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		if !restore {
			return nil, nil, fmt.Errorf("VM would not be restored correctly")
		}
	}

	extra, err := kvgraph.NewVirtualMachineBackupGraph(vm)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	return item, extra, nil
}

// returns false for all cases when backup might end up with a broken PVC snapshot
func (p *VMBackupItemAction) canBeSafelyBackedUp(vm *kvcore.VirtualMachine, backup *v1.Backup) (bool, error) {
	isRuning := vm.Status.PrintableStatus == kvcore.VirtualMachineStatusStarting || vm.Status.PrintableStatus == kvcore.VirtualMachineStatusRunning
	if !isRuning {
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
