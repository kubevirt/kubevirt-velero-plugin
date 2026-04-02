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
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// VMItemBlockAction ensures that a VirtualMachine and all its related resources
// (VMI, PVCs, DataVolumes, launcher pod, secrets, etc.) are processed as an
// atomic block during backup. This prevents partial backups where the VM is
// captured but a PVC is processed in a separate parallel thread.
type VMItemBlockAction struct {
	log logrus.FieldLogger
}

// NewVMItemBlockAction instantiates a VMItemBlockAction.
func NewVMItemBlockAction(log logrus.FieldLogger) *VMItemBlockAction {
	return &VMItemBlockAction{log: log}
}

// Name returns the name of this IBA (required by v1 interface).
func (p *VMItemBlockAction) Name() string {
	return "kubevirt-velero-plugin/block-vm-action"
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMItemBlockAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"VirtualMachine",
		},
	}, nil
}

// GetRelatedItems returns all resources that must be backed up atomically with this VM.
// Uses the same dependency graph as the BackupItemAction.
func (p *VMItemBlockAction) GetRelatedItems(item runtime.Unstructured, backup *v1.Backup) ([]velero.ResourceIdentifier, error) {
	p.log.Info("Computing VM item block dependencies")

	resources, err := kvgraph.NewObjectBackupGraph(item)
	if err != nil {
		// Non-fatal: return empty list, backup can still proceed without block guarantee
		p.log.WithError(err).Warn("Failed to compute VM backup graph for block action")
		return []velero.ResourceIdentifier{}, nil
	}

	p.log.Infof("VM item block contains %d related resources", len(resources))
	return resources, nil
}
