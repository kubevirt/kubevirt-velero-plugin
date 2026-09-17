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
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
	v1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// NewObjectBackupGraph returns the backup object graph for the passed item
func NewObjectBackupGraph(item runtime.Unstructured) ([]velero.ResourceIdentifier, error) {
	kind := item.GetObjectKind().GroupVersionKind().Kind

	switch kind {
	case "VirtualMachine":
		vm := new(v1.VirtualMachine)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vm); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewVirtualMachineBackupGraph(vm)
	case "VirtualMachineTemplate":
		vm, err := util.GetTemplateVM(item)
		if err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		accessor, err := meta.Accessor(item)
		if err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewVirtualMachineTemplateBackupGraph(vm, accessor.GetNamespace())
	case "VirtualMachineInstance":
		vmi := new(v1.VirtualMachineInstance)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vmi); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewVirtualMachineInstanceBackupGraph(vmi)
	case "DataVolume":
		dv := new(cdiv1.DataVolume)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), dv); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewDataVolumeBackupGraph(dv), nil
	case "DataSource":
		ds := new(cdiv1.DataSource)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), ds); err != nil {
			return []velero.ResourceIdentifier{}, errors.WithStack(err)
		}
		return NewDataSourceBackupGraph(ds)
	default:
		// No specific backup graph for the passed object
		return []velero.ResourceIdentifier{}, nil
	}
}

// NewVirtualMachineBackupGraph returns the backup object graph for a specific VM
func NewVirtualMachineBackupGraph(vm *v1.VirtualMachine) ([]velero.ResourceIdentifier, error) {
	var resources []velero.ResourceIdentifier
	var err error
	namespace := vm.GetNamespace()
	resources = addInstanceType(vm, resources)
	resources = addPreference(vm, resources)

	var errs []error
	if vm.Status.Created {
		resources = addVeleroResource(vm.GetName(), namespace, "virtualmachineinstances", resources)
		// Returning full backup even if there was an error retrieving the launcher pod.
		// The caller can decide whether to use the backup without launcher pod or handle the error.
		resources, err = addLauncherPod(vm.GetName(), vm.GetNamespace(), resources)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if vm.Spec.Template != nil {
		resources, err = addCommonVMIObjectGraph(vm.Spec.Template.Spec, vm.GetName(), namespace, resources)
	}
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return resources, k8serrors.NewAggregate(errs)
	}

	return resources, nil
}

// NewVirtualMachineTemplateBackupGraph returns the backup object graph for a specific
// VirtualMachineTemplate.
func NewVirtualMachineTemplateBackupGraph(vm *v1.VirtualMachine, namespace string) ([]velero.ResourceIdentifier, error) {
	return addCommonTemplateObjectGraph(vm, namespace, true, []velero.ResourceIdentifier{})
}

// NewVirtualMachineInstanceBackupGraph returns the backup object graph for a specific VMI
func NewVirtualMachineInstanceBackupGraph(vmi *v1.VirtualMachineInstance) ([]velero.ResourceIdentifier, error) {
	var resources []velero.ResourceIdentifier
	var errs []error
	// Returning full backup even if there was an error retrieving the launcher pod.
	// The caller can decide wether to use the backup without launcher pod or handle the error.
	resources, err := addLauncherPod(vmi.GetName(), vmi.GetNamespace(), resources)
	if err != nil {
		errs = append(errs, err)
	}

	resources, err = addCommonVMIObjectGraph(vmi.Spec, vmi.GetName(), vmi.GetNamespace(), resources)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return resources, k8serrors.NewAggregate(errs)
	}

	return resources, nil
}

// NewDataVolumeBackupGraph returns the backup object graph for a specific DataVolume
func NewDataVolumeBackupGraph(dv *cdiv1.DataVolume) []velero.ResourceIdentifier {
	resources := []velero.ResourceIdentifier{}
	if dv.Status.Phase == cdiv1.Succeeded {
		resources = addVeleroResource(dv.Name, dv.Namespace, "persistentvolumeclaims", resources)
	}
	return resources
}

// NewDataSourceBackupGraph returns the backup object graph for a specific DataSource: its
// backing PVC/VolumeSnapshot, or another DataSource it points to.
func NewDataSourceBackupGraph(ds *cdiv1.DataSource) ([]velero.ResourceIdentifier, error) {
	return addDataSourceObjectGraph(ds, true, []velero.ResourceIdentifier{})
}
