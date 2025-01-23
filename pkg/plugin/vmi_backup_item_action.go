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

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
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

	vmi := new(kvcore.VirtualMachineInstance)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vmi); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	// There's no point in backing up a VMI when it's owned by a VM excluded from the backup
	shouldExclude, err := shouldExcludeVMI(vmi, backup)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if shouldExclude {
		return nil, nil, nil
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
		util.AddAnnotation(item, AnnIsOwned, "true")
	} else if !util.IsMetadataBackup(backup) {
		restore, err := util.RestorePossible(vmi.Spec.Volumes, backup, vmi.Namespace, func(volume kvcore.Volume) bool { return false }, p.log)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		if !restore {
			return nil, nil, fmt.Errorf("VM has DataVolume or PVC volumes and DataVolumes/PVCs is not included in the backup")
		}
	}

	extra, err := kvgraph.NewVirtualMachineInstanceBackupGraph(vmi)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return item, extra, nil
}

// shouldExcludeVMI checks wether a VMI owned by VM should be backed up or ignored
func shouldExcludeVMI(vmi *kvcore.VirtualMachineInstance, backup *v1.Backup) (bool, error) {
	if !isVMIOwned(vmi) {
		return false, nil
	}

	if !util.IsResourceInBackup("virtualmachines", backup) {
		return true, nil
	}

	excluded, err := isVMExcludedByLabel(vmi)
	if err != nil {
		return false, err
	}

	return excluded, nil
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

	vm, err := (*client).VirtualMachine(vmi.Namespace).Get(context.Background(), vmi.Name, &metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	label, ok := vm.GetLabels()[util.VeleroExcludeLabel]
	return ok && label == "true", nil
}

func (p *VMIBackupItemAction) isPodExcludedByLabel(vmi *kvcore.VirtualMachineInstance) (bool, error) {
	pod, err := util.GetLauncherPod(vmi.GetName(), vmi.GetNamespace())
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

	label, ok := labels[util.VeleroExcludeLabel]
	return ok && label == "true", nil
}
