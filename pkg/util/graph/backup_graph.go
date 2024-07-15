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

package vmgraph

import (
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v1 "kubevirt.io/api/core/v1"
)

// NewVirtualMachineBackupGraph returns the backup object graph for a specific VM
func NewVirtualMachineBackupGraph(vm *v1.VirtualMachine) []velero.ResourceIdentifier {
	var resources []velero.ResourceIdentifier
	if vm.Spec.Instancetype != nil {
		resources = addInstanceType(*vm.Spec.Instancetype, vm.GetNamespace(), resources)
	}
	if vm.Spec.Preference != nil {
		resources = addPreferenceType(*vm.Spec.Preference, vm.GetNamespace(), resources)
	}
	if vm.Status.Created {
		resources = addVeleroResource(vm.GetName(), vm.GetNamespace(), "virtualmachineinstances", resources)
		// TODO: Add error handling
		resources, _ = addLauncherPod(vm.GetName(), vm.GetNamespace(), resources)
	}
	return addCommonVMIObjectGraph(vm.Spec.Template.Spec, vm.GetNamespace(), true, resources)
}

// NewVirtualMachineInstanceBackupGraph returns the backup object graph for a specific VMI
func NewVirtualMachineInstanceBackupGraph(vmi *v1.VirtualMachineInstance) []velero.ResourceIdentifier {
	var resources []velero.ResourceIdentifier
	// TODO: Add error handling
	resources, _ = addLauncherPod(vmi.GetName(), vmi.GetNamespace(), resources)
	return addCommonVMIObjectGraph(vmi.Spec, vmi.GetNamespace(), true, resources)
}
