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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kvcore "kubevirt.io/client-go/api/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// VMIBackupItemAction is a backup item action for backing up DataVolumes
type VMIBackupItemAction struct {
	log    logrus.FieldLogger
	client kubernetes.Interface
}

const (
	AnnIsOwned = "cdi.kubevirt.io/velero.isOwned"
)

// NewVMIBackupItemAction instantiates a VMIBackupItemAction.
func NewVMIBackupItemAction(log logrus.FieldLogger, client kubernetes.Interface) *VMIBackupItemAction {
	return &VMIBackupItemAction{log: log, client: client}
}

// AppliesTo returns information about which resources this action should be invoked for.
// The IncludedResources and ExcludedResources slices can include both resources
// and resources with group names. These work: "ingresses", "ingresses.extensions".
// A VMIBackupItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.
func (p *VMIBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"VirtualMachineInstance",
			},
		},
		nil
}

// Execute returns VM's DataVolumes as extra items to back up.
func (p *VMIBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMIBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	extra := []velero.ResourceIdentifier{}

	vmi := new(kvcore.VirtualMachineInstance)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vmi); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	if !util.IsVMIPaused(vmi) {
		if !util.IsResourceInBackup("pods", backup) && util.IsResourceInBackup("persistentvolumeclaims", backup) {
			return nil, nil, fmt.Errorf("VM is running but launcher pod is not included in the backup")
		}

		excluded, err := p.isPodExcludedByLabel(vmi)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}

		if excluded {
			return nil, nil, fmt.Errorf("VM is running but launcher pod is not included in the backup")
		}
	}

	if isVMIOwned(vmi) {
		if !util.IsResourceInBackup("virtualmachines", backup) {
			return nil, nil, fmt.Errorf("VMI owned by a VM and the VM is not included in the backup")
		}

		excluded, err := isVMExcludedByLabel(vmi)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}

		if excluded {
			return nil, nil, fmt.Errorf("VMI owned by a VM and the VM is not included in the backup")
		}

		util.AddAnnotation(item, AnnIsOwned, "true")
	} else {
		restore, err := util.RestorePossible(vmi.Spec.Volumes, backup, vmi.Namespace, func(volume kvcore.Volume) bool { return false }, p.log)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		if !restore {
			return nil, nil, fmt.Errorf("VM has DataVolume or PVC volumes and DataVolumes/PVCs is not included in the backup")
		}
	}

	extra, err := p.addLauncherPod(vmi, extra)
	if err != nil {
		return nil, nil, err
	}

	extra = addVolumes(vmi, extra)

	return item, extra, nil
}

func isVMIOwned(vmi *kvcore.VirtualMachineInstance) bool {
	return len(vmi.OwnerReferences) > 0
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var isVMExcludedByLabel = func(vmi *kvcore.VirtualMachineInstance) (bool, error) {
	client, err := util.GetKubeVirtclient()
	if err != nil {
		return false, err
	}

	vm, err := (*client).VirtualMachine(vmi.Namespace).Get(vmi.Name, &metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	label, ok := vm.GetLabels()[util.VELERO_EXCLUDE_LABEL]
	return ok && label == "true", nil
}

func (p *VMIBackupItemAction) isPodExcludedByLabel(vmi *kvcore.VirtualMachineInstance) (bool, error) {
	pod, err := p.getLauncherPod(vmi)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, fmt.Errorf("pod for running VMI not found")
	}

	labels := pod.GetLabels()
	if labels == nil {
		return false, nil
	}

	label, ok := labels[util.VELERO_EXCLUDE_LABEL]
	return ok && label == "true", nil
}

func (p *VMIBackupItemAction) getLauncherPod(vmi *kvcore.VirtualMachineInstance) (*core.Pod, error) {
	pods, err := p.client.CoreV1().Pods(vmi.GetNamespace()).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "kubevirt.io=virt-launcher",
	})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if pod.Annotations["kubevirt.io/domain"] == vmi.GetName() {
			return &pod, nil
		}
	}

	return nil, nil
}

func (p *VMIBackupItemAction) addLauncherPod(vmi *kvcore.VirtualMachineInstance, extra []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	pod, err := p.getLauncherPod(vmi)
	if err != nil {
		return nil, err
	}
	if pod != nil {
		extra = append(extra, velero.ResourceIdentifier{
			GroupResource: kuberesource.Pods,
			Namespace:     vmi.GetNamespace(),
			Name:          pod.GetName(),
		})
	}

	return extra, nil
}

func addVolumes(vmi *kvcore.VirtualMachineInstance, extra []velero.ResourceIdentifier) []velero.ResourceIdentifier {
	for _, volume := range vmi.Spec.Volumes {
		if volume.DataVolume != nil {
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
				Namespace:     vmi.GetNamespace(),
				Name:          volume.DataVolume.Name,
			})
		}
		if volume.PersistentVolumeClaim != nil {
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.PersistentVolumeClaims,
				Namespace:     vmi.GetNamespace(),
				Name:          volume.PersistentVolumeClaim.ClaimName,
			})
		}
		// TODO what about other types of volumes?
	}

	return extra
}
