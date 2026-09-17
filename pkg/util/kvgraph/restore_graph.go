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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
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
	case "VirtualMachineTemplate":
		vm, err := util.GetTemplateVM(item)
		if err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		accessor, err := meta.Accessor(item)
		if err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewVirtualMachineTemplateRestoreGraph(vm, accessor.GetNamespace())
	case "VirtualMachineInstance":
		vmi := new(v1.VirtualMachineInstance)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vmi); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewVirtualMachineInstanceRestoreGraph(vmi)
	case "DataSource":
		ds := new(cdiv1.DataSource)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), ds); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewDataSourceRestoreGraph(ds)
	default:
		// No specific restore graph for the passed object
		return []velero.ResourceIdentifier{}, nil
	}
}

// NewVirtualMachineRestoreGraph returns the restore object graph for a specific VM
func NewVirtualMachineRestoreGraph(vm *v1.VirtualMachine) ([]velero.ResourceIdentifier, error) {
	var resources []velero.ResourceIdentifier

	resources = addInstanceType(vm, resources)
	resources = addPreference(vm, resources)
	return addCommonVMIObjectGraph(vm.Spec.Template.Spec, vm.GetName(), vm.GetNamespace(), resources)
}

// NewVirtualMachineInstanceRestoreGraph returns the restore object graph for a specific VMI
func NewVirtualMachineInstanceRestoreGraph(vmi *v1.VirtualMachineInstance) ([]velero.ResourceIdentifier, error) {
	return addCommonVMIObjectGraph(vmi.Spec, vmi.GetName(), vmi.GetNamespace(), []velero.ResourceIdentifier{})
}

// NewVirtualMachineTemplateRestoreGraph returns the restore object graph for a specific
// VirtualMachineTemplate. See NewVirtualMachineTemplateBackupGraph for the rationale.
func NewVirtualMachineTemplateRestoreGraph(vm *v1.VirtualMachine, namespace string) ([]velero.ResourceIdentifier, error) {
	return addCommonTemplateObjectGraph(vm, namespace, false, []velero.ResourceIdentifier{})
}

// NewDataSourceRestoreGraph returns the restore object graph for a specific DataSource. See
// NewDataSourceBackupGraph for the rationale.
func NewDataSourceRestoreGraph(ds *cdiv1.DataSource) ([]velero.ResourceIdentifier, error) {
	return addDataSourceObjectGraph(ds, false, []velero.ResourceIdentifier{})
}
