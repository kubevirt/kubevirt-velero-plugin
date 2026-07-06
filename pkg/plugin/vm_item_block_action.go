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
 * Copyright 2025 Red Hat, Inc.
 *
 */

package plugin

import (
	"github.com/sirupsen/logrus"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt-velero-plugin/pkg/kvgraph"
)

// VMItemBlockAction is an item block action for VirtualMachines
type VMItemBlockAction struct {
	log logrus.FieldLogger
}

// NewVMItemBlockAction instantiates a VMItemBlockAction.
func NewVMItemBlockAction(log logrus.FieldLogger) *VMItemBlockAction {
	return &VMItemBlockAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMItemBlockAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"virtualmachines.kubevirt.io",
		},
	}, nil
}

// GetRelatedItems returns the related items for the VirtualMachine using the backup graph.
func (p *VMItemBlockAction) GetRelatedItems(item runtime.Unstructured, backup *v1.Backup) ([]velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMItemBlockAction GetRelatedItems")

	vm := &kubevirtv1.VirtualMachine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vm); err != nil {
		return nil, err
	}

	return kvgraph.NewVirtualMachineBackupGraph(vm), nil
}
