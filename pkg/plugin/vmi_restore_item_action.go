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
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kvcore "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// VMIRestorePlugin is a VMI restore item action plugin for Velero (duh!)
type VMIRestorePlugin struct {
	log logrus.FieldLogger
}

// Copied over from KubeVirt
// TODO: Consider making it public in KubeVirt
var restrictedVmiLabels = []string{
	kvcore.CreatedByLabel,
	kvcore.MigrationJobLabel,
	kvcore.NodeNameLabel,
	kvcore.MigrationTargetNodeNameLabel,
	kvcore.NodeSchedulable,
	kvcore.InstallStrategyLabel,
}

// NewVMIRestorePlugin instantiates a RestorePlugin.
func NewVMIRestoreItemAction(log logrus.FieldLogger) *VMIRestorePlugin {
	return &VMIRestorePlugin{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMIRestorePlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"VirtualMachineInstance",
		},
	}, nil
}

func (p *VMIRestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running VMIRestorePlugin")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	vmi := new(kvcore.VirtualMachineInstance)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), vmi); err != nil {
		return nil, errors.WithStack(err)
	}

	owned, ok := vmi.Annotations[AnnIsOwned]
	if ok && owned == "true" {
		p.log.Info("VMI is owned by a VM, it doesn't need to be restored")
		return velero.NewRestoreItemActionExecuteOutput(input.Item).WithoutRestore(), nil
	}

	metadata, err := meta.Accessor(input.Item)
	if err != nil {
		return nil, err
	}

	if util.ShouldClearMacAddress(input.Restore) {
		p.log.Info("Clear virtual machine instance MAC addresses")
		util.ClearMacAddress(&vmi.Spec)
	}

	// Restricted labels must be cleared otherwise the VMI will be rejected.
	// The restricted labels contain runtime information about the underlying KVM object.
	labels := removeRestrictedLabels(vmi.GetLabels())
	metadata.SetLabels(labels)

	output := velero.NewRestoreItemActionExecuteOutput(input.Item)
	output.AdditionalItems = kvgraph.NewVirtualMachineInstanceRestoreGraph(vmi)
	return output, nil
}

func removeRestrictedLabels(labels map[string]string) map[string]string {
	for _, label := range restrictedVmiLabels {
		delete(labels, label)
	}
	return labels
}
