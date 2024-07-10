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

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// VMObjectGraph represents the graph of objects that can be potentially related to a VirtualMachine
var VMObjectGraph = map[string]schema.GroupResource{
	"virtualmachineinstances":           {Group: "kubevirt.io", Resource: "virtualmachineinstances"},
	"virtualmachineinstancetype":        {Group: "instancetype.kubevirt.io", Resource: "virtualmachineinstancetype"},
	"virtualmachineclusterinstancetype": {Group: "instancetype.kubevirt.io", Resource: "virtualmachineclusterinstancetype"},
	"virtualmachinepreference":          {Group: "instancetype.kubevirt.io", Resource: "virtualmachinepreference"},
	"virtualmachineclusterpreference":   {Group: "instancetype.kubevirt.io", Resource: "virtualmachineclusterpreference"},
	"datavolumes":                       {Group: "cdi.kubevirt.io", Resource: "datavolumes"},
	"controllerrevisions":               {Group: "apps", Resource: "controllerrevisions"},
	"configmaps":                        {Group: "", Resource: "configmaps"},
	"persistentvolumeclaims":            kuberesource.PersistentVolumeClaims,
	"serviceaccounts":                   kuberesource.ServiceAccounts,
	"secrets":                           kuberesource.Secrets,
	"pods":                              kuberesource.Pods,
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

	return addVMIObjectGraph(vm, resources)
}

func addVMIObjectGraph(vm *v1.VirtualMachine, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	vmi := &v1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace},
		Spec:       vm.Spec.Template.Spec,
	}
	if vm.Status.Created {
		// Using an arbitrary phase to signify the VMI has been created and
		// ensure the launcher pod is included in the backup process.
		// TODO: This might be a bit arbitrary, maybe we can find a better alternative.
		vmi.Status.Phase = v1.Running
	}
	return append(resources, NewVirtualMachineInstanceObjectGraph(vmi)...)
}

func NewVirtualMachineInstanceObjectGraph(vmi *v1.VirtualMachineInstance) []velero.ResourceIdentifier {
	var resources []velero.ResourceIdentifier
	if vmi.Status.Phase != "" {
		// TODO: Add error handling
		resources, _ = addLauncherPod(vmi.GetName(), vmi.GetNamespace(), resources)
	}
	resources = addVolumeGraph(vmi.Spec.Volumes, vmi.GetNamespace(), resources)
	resources = addAccessCredentials(vmi.Spec.AccessCredentials, vmi.GetNamespace(), resources)
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

func addLauncherPod(vmiName, vmiNamespace string, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	pod, err := util.GetLauncherPod(vmiName, vmiNamespace)
	if err != nil || pod == nil {
		// Still return the list of the resources even if we couldn't get the launcher pod
		return resources, err
	}
	return addVeleroResource(pod.GetName(), vmiNamespace, "pods", resources), nil
}
