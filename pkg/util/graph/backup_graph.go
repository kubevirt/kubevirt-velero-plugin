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
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// NewVirtualMachineBackupGraph returns the backup object graph for a specific VM
func NewVirtualMachineBackupGraph(vm *v1.VirtualMachine) ([]velero.ResourceIdentifier, error) {
	var resources []velero.ResourceIdentifier
	var err error
	namespace := vm.GetNamespace()

	if vm.Spec.Instancetype != nil {
		resources = addInstanceType(*vm.Spec.Instancetype, vm.GetNamespace(), resources)
	}
	if vm.Spec.Preference != nil {
		resources = addPreferenceType(*vm.Spec.Preference, vm.GetNamespace(), resources)
	}
	if vm.Status.Created {
		resources = addVeleroResource(vm.GetName(), namespace, "virtualmachineinstances", resources)
		// Returning full backup even if there was an error retrieving the launcher pod.
		// The caller can decide wether to use the backup without launcher pod or handle the error.
		resources, err = addLauncherPod(vm.GetName(), vm.GetNamespace(), resources)
	}

	return addCommonVMIObjectGraph(vm.Spec.Template.Spec, namespace, true, resources), err
}

// NewVirtualMachineInstanceBackupGraph returns the backup object graph for a specific VMI
func NewVirtualMachineInstanceBackupGraph(vmi *v1.VirtualMachineInstance) ([]velero.ResourceIdentifier, error) {
	var resources []velero.ResourceIdentifier
	var err error
	// Returning full backup even if there was an error retrieving the launcher pod.
	// The caller can decide wether to use the backup without launcher pod or handle the error.
	resources, err = addLauncherPod(vmi.GetName(), vmi.GetNamespace(), resources)
	return addCommonVMIObjectGraph(vmi.Spec, vmi.GetNamespace(), true, resources), err
}

// NewDataVolumeBackupGraph returns the backup object graph for a specific DataVolume
func NewDataVolumeBackupGraph(dv *cdiv1.DataVolume) []velero.ResourceIdentifier {
	resources := []velero.ResourceIdentifier{}
	if dv.Status.Phase == cdiv1.Succeeded {
		resources = addVeleroResource(dv.Name, dv.Namespace, "persistentvolumeclaims", resources)
	}
	return resources
}
