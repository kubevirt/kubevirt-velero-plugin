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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"kubevirt.io/api/instancetype"

	v1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

const (
	backendStoragePrefix = "persistent-state-for"
)

// KVObjectGraph represents the graph of objects that can be potentially related to a KubeVirt resource
var KVObjectGraph = map[string]schema.GroupResource{
	"virtualmachineinstances":       {Group: "kubevirt.io", Resource: "virtualmachineinstances"},
	"datavolumes":                   {Group: "cdi.kubevirt.io", Resource: "datavolumes"},
	"datasources":                   {Group: "cdi.kubevirt.io", Resource: "datasources"},
	"controllerrevisions":           {Group: "apps", Resource: "controllerrevisions"},
	"configmaps":                    {Group: "", Resource: "configmaps"},
	"networkattachmentdefinitions":  {Group: "k8s.cni.cncf.io", Resource: "network-attachment-definitions"},
	"persistentvolumeclaims":        kuberesource.PersistentVolumeClaims,
	"serviceaccounts":               kuberesource.ServiceAccounts,
	"secrets":                       kuberesource.Secrets,
	"pods":                          kuberesource.Pods,
	"volumesnapshots":               {Group: "snapshot.storage.k8s.io", Resource: "volumesnapshots"},
	instancetype.PluralResourceName: {Group: instancetype.GroupName, Resource: instancetype.PluralResourceName},
	instancetype.PluralPreferenceResourceName: {Group: instancetype.GroupName, Resource: instancetype.PluralPreferenceResourceName},
}

// IsParameterized reports whether s contains a virt-template parameter reference (e.g. "${NAME}").
func IsParameterized(s string) bool {
	return strings.Contains(s, "${")
}

func addVeleroResource(name, namespace, resource string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	if IsParameterized(name) || IsParameterized(namespace) {
		return resources
	}
	if groupResource, ok := KVObjectGraph[resource]; ok {
		resources = append(resources, velero.ResourceIdentifier{
			GroupResource: groupResource,
			Namespace:     namespace,
			Name:          name,
		})
	}
	return resources
}

func addCommonVMIObjectGraph(spec v1.VirtualMachineInstanceSpec, vmName, namespace string, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	resources, err := addVolumeGraph(spec, vmName, namespace, resources)
	resources = addAccessCredentials(spec.AccessCredentials, namespace, resources)
	resources = addNetworkGraph(spec, namespace, resources)
	return resources, err
}

func addCommonTemplateObjectGraph(vm *v1.VirtualMachine, namespace string, checkDataVolumeExists bool, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	if vm == nil {
		return resources, nil
	}

	resources = addTemplateInstancetypeGraph(vm, namespace, resources)
	resources, err := addDataVolumeTemplateGraph(vm.Spec.DataVolumeTemplates, namespace, checkDataVolumeExists, resources)
	if err != nil {
		return nil, err
	}

	if vm.Spec.Template == nil {
		return resources, nil
	}

	spec := vm.Spec.Template.Spec
	resources = addVolumeSourceGraph(spec.Volumes, namespace, resources)
	resources = addAccessCredentials(spec.AccessCredentials, namespace, resources)
	resources = addNetworkGraph(spec, namespace, resources)

	return resources, nil
}

func addNetworkGraph(vmiSpec v1.VirtualMachineInstanceSpec, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	for _, net := range vmiSpec.Networks {
		if net.Multus != nil && net.Multus.NetworkName != "" {
			nadName := net.Multus.NetworkName
			nadNamespace := namespace
			if parts := strings.SplitN(nadName, "/", 2); len(parts) == 2 {
				nadNamespace = parts[0]
				nadName = parts[1]
			}
			resources = addVeleroResource(nadName, nadNamespace, "networkattachmentdefinitions", resources)
		}
	}
	return resources
}

func addVolumeGraph(vmiSpec v1.VirtualMachineInstanceSpec, vmName, namespace string, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	resources = addVolumeSourceGraph(vmiSpec.Volumes, namespace, resources)

	// Returning full backup even if there was an error retrieving the backend PVC.
	// The caller can decide wether to use the backup or handle the error.
	var err error
	if IsBackendStorageNeededForVMI(&vmiSpec) {
		resources, err = addBackendPVC(vmName, namespace, resources)
	}
	return resources, err
}

func addVolumeSourceGraph(volumes []v1.Volume, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
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
		case volume.CloudInitNoCloud != nil:
			if volume.CloudInitNoCloud.UserDataSecretRef != nil {
				resources = addVeleroResource(volume.CloudInitNoCloud.UserDataSecretRef.Name, namespace, "secrets", resources)
			}
			if volume.CloudInitNoCloud.NetworkDataSecretRef != nil {
				resources = addVeleroResource(volume.CloudInitNoCloud.NetworkDataSecretRef.Name, namespace, "secrets", resources)
			}
		case volume.CloudInitConfigDrive != nil:
			if volume.CloudInitConfigDrive.UserDataSecretRef != nil {
				resources = addVeleroResource(volume.CloudInitConfigDrive.UserDataSecretRef.Name, namespace, "secrets", resources)
			}
			if volume.CloudInitConfigDrive.NetworkDataSecretRef != nil {
				resources = addVeleroResource(volume.CloudInitConfigDrive.NetworkDataSecretRef.Name, namespace, "secrets", resources)
			}
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

func addInstanceType(vm *v1.VirtualMachine, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	if vm.Spec.Instancetype != nil {
		return addInstanceTypeMatcherResource(vm.Spec.Instancetype, vm.Status.InstancetypeRef, vm.GetNamespace(), resources)
	}
	return resources
}

func addPreference(vm *v1.VirtualMachine, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	if vm.Spec.Preference != nil {
		return addInstanceTypeMatcherResource(vm.Spec.Preference, vm.Status.PreferenceRef, vm.GetNamespace(), resources)
	}
	return resources
}

func addInstanceTypeMatcherResource(matcher v1.Matcher, statusRef *v1.InstancetypeStatusRef, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	if statusRef != nil && statusRef.ControllerRevisionRef != nil {
		resources = addVeleroResource(statusRef.ControllerRevisionRef.Name, namespace, "controllerrevisions", resources)
	}
	// Handle cases where the VM Status hasn't been populated yet by falling back to any spec provided RevisionName
	if statusRef == nil && matcher.GetRevisionName() != "" {
		resources = addVeleroResource(matcher.GetRevisionName(), namespace, "controllerrevisions", resources)
	}
	return resources
}

func addBackendPVC(vmName, namespace string, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	labelSelector := fmt.Sprintf("%s=%s", backendStoragePrefix, vmName)
	pvcs, err := util.ListPVCs(labelSelector, namespace)
	if err != nil {
		return resources, err
	}
	if len(pvcs.Items) == 0 {
		// Kubevirt introduced the backend PVC labeling in 1.4.0.
		// If backend PVC is no labeled, let's fallback to the old naming convention.
		// TODO: Stop supporting the old naming convention in the future.
		resources = addVeleroResource(fmt.Sprintf("%s-%s", backendStoragePrefix, vmName), namespace, "persistentvolumeclaims", resources)
		return resources, nil
	}
	for _, pvc := range pvcs.Items {
		// Should only be one PVC with the label.
		// Still range to be agnostic to Kubevirt's internal logic.
		resources = addVeleroResource(pvc.Name, namespace, "persistentvolumeclaims", resources)
	}

	return resources, nil
}

func IsBackendStorageNeededForVMI(vmiSpec *v1.VirtualMachineInstanceSpec) bool {
	return HasPersistentTPMDevice(vmiSpec) || HasPersistentEFI(vmiSpec)
}

func HasPersistentTPMDevice(vmiSpec *v1.VirtualMachineInstanceSpec) bool {
	return vmiSpec.Domain.Devices.TPM != nil &&
		vmiSpec.Domain.Devices.TPM.Persistent != nil &&
		*vmiSpec.Domain.Devices.TPM.Persistent
}

func HasPersistentEFI(vmiSpec *v1.VirtualMachineInstanceSpec) bool {
	return vmiSpec.Domain.Firmware != nil &&
		vmiSpec.Domain.Firmware.Bootloader != nil &&
		vmiSpec.Domain.Firmware.Bootloader.EFI != nil &&
		vmiSpec.Domain.Firmware.Bootloader.EFI.Persistent != nil &&
		*vmiSpec.Domain.Firmware.Bootloader.EFI.Persistent
}

func addNamespacedInstancetype(m v1.Matcher, singular, plural, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	// Kind defaults to the cluster-scoped resource when unset, so only an explicit,
	// namespace-scoped Kind is followed here.
	if m == nil || m.GetName() == "" || !strings.EqualFold(m.GetKind(), singular) {
		return resources
	}
	return addVeleroResource(m.GetName(), namespace, plural, resources)
}

func addTemplateInstancetypeGraph(vm *v1.VirtualMachine, namespace string, resources []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	if vm.Spec.Instancetype != nil {
		resources = addNamespacedInstancetype(vm.Spec.Instancetype, instancetype.SingularResourceName, instancetype.PluralResourceName, namespace, resources)
	}
	if vm.Spec.Preference != nil {
		resources = addNamespacedInstancetype(vm.Spec.Preference, instancetype.SingularPreferenceResourceName, instancetype.PluralPreferenceResourceName, namespace, resources)
	}
	return resources
}

func addDataVolumeTemplateGraph(dvts []v1.DataVolumeTemplateSpec, namespace string, checkDataVolumeExists bool, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	var err error
	for _, dvt := range dvts {
		if src := dvt.Spec.Source; src != nil {
			resources, err = addPVCAndSnapshotSourceGraph(src.PVC, src.Snapshot, namespace, checkDataVolumeExists, resources)
			if err != nil {
				return nil, err
			}
		}
		if ref := dvt.Spec.SourceRef; ref != nil && ref.Kind == "DataSource" {
			ns := namespace
			if ref.Namespace != nil {
				ns = *ref.Namespace
			}
			resources = addVeleroResource(ref.Name, ns, "datasources", resources)
		}
	}
	return resources, nil
}

func addPVCAndSnapshotSourceGraph(pvc *cdiv1.DataVolumeSourcePVC, snap *cdiv1.DataVolumeSourceSnapshot, namespace string, checkDataVolumeExists bool, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	if pvc != nil {
		ns := defaultNamespace(pvc.Namespace, namespace)
		resources = addVeleroResource(pvc.Name, ns, "persistentvolumeclaims", resources)

		addDataVolume := true
		if checkDataVolumeExists {
			_, err := util.GetDV(ns, pvc.Name)
			switch {
			case err == nil:
			case apierrors.IsNotFound(err):
				// A PVC not backed by a DataVolume - the PVC identifier alone is enough.
				addDataVolume = false
			default:
				// Anything else (RBAC, API unavailable) is not evidence of absence -
				// fail rather than silently leave the DataVolume out of the backup.
				return nil, errors.Wrapf(err, "failed to check whether DataVolume %s/%s exists", ns, pvc.Name)
			}
		}
		if addDataVolume {
			resources = addVeleroResource(pvc.Name, ns, "datavolumes", resources)
		}
	}
	if snap != nil {
		resources = addVeleroResource(snap.Name, defaultNamespace(snap.Namespace, namespace), "volumesnapshots", resources)
	}
	return resources, nil
}

func addDataSourceObjectGraph(ds *cdiv1.DataSource, checkDataVolumeExists bool, resources []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	if ds == nil {
		return resources, nil
	}

	namespace := ds.GetNamespace()
	src := ds.Spec.Source
	resources, err := addPVCAndSnapshotSourceGraph(src.PVC, src.Snapshot, namespace, checkDataVolumeExists, resources)
	if err != nil {
		return nil, err
	}
	if nested := src.DataSource; nested != nil {
		resources = addVeleroResource(nested.Name, defaultNamespace(nested.Namespace, namespace), "datasources", resources)
	}
	return resources, nil
}

func defaultNamespace(namespace, fallback string) string {
	if namespace == "" {
		return fallback
	}
	return namespace
}
