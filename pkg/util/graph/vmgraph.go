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
	"strings"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
)

// VMObjectGraph represents the graph of objects that can be potentially related to a VirtualMachine
var VMObjectGraph = map[string]schema.GroupResource{
	"virtualmachineinstances":           {Group: "kubevirt.io", Resource: "virtualmachineinstances"},
	"virtualmachineinstancetype":        {Group: "instancetype.kubevirt.io", Resource: "virtualmachineinstancetype"},
	"virtualmachineclusterinstancetype": {Group: "instancetype.kubevirt.io", Resource: "virtualmachineclusterinstancetype"},
	"virtualmachinepreference":          {Group: "instancetype.kubevirt.io", Resource: "virtualmachinepreference"},
	"virtualmachineclusterpreference":   {Group: "instancetype.kubevirt.io", Resource: "virtualmachineclusterpreference"},
	"controllerrevisions":               {Group: "apps", Resource: "controllerrevisions"},
	"datavolumes":                       {Group: "cdi.kubevirt.io", Resource: "datavolumes"},
	"persistentvolumeclaims":            {Group: "", Resource: "persistentvolumeclaims"},
	"serviceaccounts":                   {Group: "", Resource: "serviceaccounts"},
	"configmaps":                        {Group: "", Resource: "configmaps"},
	"secrets":                           {Group: "", Resource: "secrets"},
}

func addVeleroResource(name, namespace, resource string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	if groupResource, ok := VMObjectGraph[resource]; ok {
		resources = append(resources, velero.ResourceIdentifier{
			GroupResource: groupResource,
			Namespace:     namespace,
			Name:          name,
		})
	}
	return resources
}

// NewVirtualMachineObjectGraph returns the VirtualMachineGraph for a specific VM
func NewVirtualMachineObjectGraph(vm *v1.VirtualMachine) []velero.ResourceIdentifier {
	var resources []velero.ResourceIdentifier
	namespace := vm.GetNamespace()

	if vm.Spec.Instancetype != nil {
		resources = addVeleroResource(vm.Spec.Instancetype.Name, namespace, strings.ToLower(vm.Spec.Instancetype.Kind), resources)
		resources = addVeleroResource(vm.Spec.Instancetype.RevisionName, namespace, "controllerrevisions", resources)
	}
	if vm.Spec.Preference != nil {
		resources = addVeleroResource(vm.Spec.Preference.Name, namespace, strings.ToLower(vm.Spec.Preference.Kind), resources)
		resources = addVeleroResource(vm.Spec.Preference.RevisionName, namespace, "controllerrevisions", resources)
	}
	if vm.Status.Created {
		resources = addVeleroResource(vm.GetName(), namespace, "virtualmachineinstances", resources)
	}

	return AddVMIObjectGraph(vm.Spec.Template.Spec, vm.GetNamespace(), resources)
}

func AddVMIObjectGraph(spec v1.VirtualMachineInstanceSpec, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	resources = addVolumeGraph(spec.Volumes, namespace, resources)
	resources = addAccessCredentials(spec.AccessCredentials, namespace, resources)
	return resources
}

func addVolumeGraph(volumes []v1.Volume, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	for _, volume := range volumes {
		switch {
		case volume.DataVolume != nil:
			resources = addVeleroResource(volume.DataVolume.Name, namespace, "datavolumes", resources)
			resources = addVeleroResource(volume.DataVolume.Name, namespace, "persistentvolumeclaims", resources)
		case volume.PersistentVolumeClaim != nil:
			resources = addVeleroResource(volume.PersistentVolumeClaim.ClaimName, namespace, "persistentvolumeclaims", resources)
		case volume.MemoryDump != nil:
			resources = addVeleroResource(volume.MemoryDump.ClaimName, namespace, "persistentvolumeclaims", resources)
		case volume.ConfigMap != nil:
			resources = addVeleroResource(volume.ConfigMap.Name, namespace, "configmaps", resources)
		case volume.Secret != nil:
			resources = addVeleroResource(volume.Secret.SecretName, namespace, "secrets", resources)
		case volume.ServiceAccount != nil:
			resources = addVeleroResource(volume.ServiceAccount.ServiceAccountName, namespace, "serviceaccounts", resources)
		}
	}
	return resources
}

func addAccessCredentials(acs []v1.AccessCredential, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	for _, ac := range acs {
		if ac.SSHPublicKey != nil && ac.SSHPublicKey.Source.Secret != nil {
			resources = addVeleroResource(ac.SSHPublicKey.Source.Secret.SecretName, namespace, "secrets", resources)
		} else if ac.UserPassword != nil && ac.UserPassword.Source.Secret != nil {
			resources = addVeleroResource(ac.UserPassword.Source.Secret.SecretName, namespace, "secrets", resources)
		}
	}
	return resources
}
