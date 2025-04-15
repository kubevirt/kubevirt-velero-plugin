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

package kvgraph

import (
	"github.com/pkg/errors"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"

	"k8s.io/apimachinery/pkg/runtime"
	v1 "kubevirt.io/api/core/v1"
)

// NewObjectRestoreGraph returns the restore object graph for the passed item
func NewObjectRestoreGraph(item runtime.Unstructured) ([]velero.ResourceIdentifier, error) {
	kind := item.GetObjectKind().GroupVersionKind().Kind

	switch kind {
	case "VirtualMachine":
		vm := new(v1.VirtualMachine)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vm); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewVirtualMachineRestoreGraph(vm)
	case "VirtualMachineInstance":
		vmi := new(v1.VirtualMachineInstance)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vmi); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewVirtualMachineInstanceRestoreGraph(vmi)
	default:
		// No specific restore graph for the passed object
		return []velero.ResourceIdentifier{}, nil
	}
}

// NewVirtualMachineRestoreGraph returns the restore object graph for a specific VM
func NewVirtualMachineRestoreGraph(vm *v1.VirtualMachine) ([]velero.ResourceIdentifier, error) {
	var resources []velero.ResourceIdentifier
	if vm.Spec.Instancetype != nil {
		resources = addInstanceType(*vm.Spec.Instancetype, vm.GetNamespace(), resources)
	}
	if vm.Spec.Preference != nil {
		resources = addPreferenceType(*vm.Spec.Preference, vm.GetNamespace(), resources)
	}
	return addCommonVMIObjectGraph(vm.Spec.Template.Spec, vm.GetName(), vm.GetNamespace(), resources)
}

// NewVirtualMachineInstanceRestoreGraph returns the restore object graph for a specific VMI
func NewVirtualMachineInstanceRestoreGraph(vmi *v1.VirtualMachineInstance) ([]velero.ResourceIdentifier, error) {
	return addCommonVMIObjectGraph(vmi.Spec, vmi.GetName(), vmi.GetNamespace(), []velero.ResourceIdentifier{})
}
