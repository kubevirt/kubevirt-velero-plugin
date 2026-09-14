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

package plugin

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kvcore "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// VMTBackupItemAction is a backup item action for backing up VirtualMachineTemplates
type VMTBackupItemAction struct {
	log logrus.FieldLogger
}

// NewVMTBackupItemAction instantiates a VMTBackupItemAction.
func NewVMTBackupItemAction(log logrus.FieldLogger) *VMTBackupItemAction {
	return &VMTBackupItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMTBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"virtualmachinetemplates.template.kubevirt.io",
		},
	}, nil
}

// Execute returns the VirtualMachineTemplate's golden-image DataVolumes and other
// referenced objects as extra items to back up.
func (p *VMTBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMTBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	vm, err := util.GetTemplateVM(item)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	accessor, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	// Unlike a VirtualMachine, an incomplete VirtualMachineTemplate backup does not risk a
	// corrupted snapshot - it just produces a template that can't be processed into a fully
	// working VM. So missing golden images are a warning, not a hard failure.
	if vm != nil && !util.IsMetadataBackup(backup) {
		p.warnIfGoldenImagesNotBackedUp(vm, accessor.GetNamespace(), backup)
	}

	extra, err := kvgraph.NewVirtualMachineTemplateBackupGraph(vm, accessor.GetNamespace())
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return item, extra, nil
}

// warnIfGoldenImagesNotBackedUp logs a warning for every DataVolumeTemplate golden image
// PVC source that will not be restorable, either because DataVolumes/PersistentVolumeClaims
// are excluded from the backup, or because the source DataVolume itself carries the
// velero.io/exclude-from-backup label.
func (p *VMTBackupItemAction) warnIfGoldenImagesNotBackedUp(vm *kvcore.VirtualMachine, namespace string, backup *v1.Backup) {
	for _, dvt := range vm.Spec.DataVolumeTemplates {
		if dvt.Spec.Source == nil || dvt.Spec.Source.PVC == nil {
			continue
		}
		p.warnIfGoldenImageNotBackedUp(dvt.Spec.Source.PVC, namespace, backup)
	}
}

func (p *VMTBackupItemAction) warnIfGoldenImageNotBackedUp(pvc *cdiv1.DataVolumeSourcePVC, namespace string, backup *v1.Backup) {
	if kvgraph.IsParameterized(pvc.Name) || kvgraph.IsParameterized(pvc.Namespace) {
		return
	}

	ns := pvc.Namespace
	if ns == "" {
		ns = namespace
	}

	if !util.IsResourceInBackup("datavolumes", backup) || !util.IsResourceInBackup("persistentvolumeclaims", backup) {
		p.log.Warnf("golden image %s/%s is not included in the backup; the restored template will be incomplete", ns, pvc.Name)
		return
	}

	excluded, err := util.IsDVExcludedByLabel(ns, pvc.Name)
	if err != nil {
		// The DataVolume may not exist (e.g. only the PVC remains); that's fine, nothing more to check.
		return
	}
	if excluded {
		p.log.Warnf("golden image DataVolume %s/%s is excluded from the backup; the restored template will be incomplete", ns, pvc.Name)
	}
}
