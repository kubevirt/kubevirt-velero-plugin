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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// VIMRestorePlugin is a VMI restore item action plugin for Velero (duh!)
type VMRestorePlugin struct {
	log logrus.FieldLogger
}

// NewVMRestorePlugin instantiates a RestorePlugin.
func NewVMRestoreItemAction(log logrus.FieldLogger) *VMRestorePlugin {
	return &VMRestorePlugin{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMRestorePlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"VirtualMachine",
		},
	}, nil
}

// Execute – If VM was running, it must be restored as stopped
func (p *VMRestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running VMRestorePlugin")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	vm := new(kvcore.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), vm); err != nil {
		return nil, errors.WithStack(err)
	}

	if runStrategy, ok := util.GetRestoreRunStrategy(input.Restore); ok {
		p.log.Infof("Setting virtual machine run strategy to %s", runStrategy)
		vm.Spec.RunStrategy = ptr.To(runStrategy)
		vm.Spec.Running = nil
	}

	if util.ShouldClearMacAddress(input.Restore) {
		p.log.Info("Clear virtual machine MAC addresses")
		util.ClearMacAddress(&vm.Spec.Template.Spec)
	}

	if util.ShouldGenerateNewFirmwareUUID(input.Restore) {
		p.log.Info("Generate new firmware UUID")
		util.GenerateNewFirmwareUUID(&vm.Spec.Template.Spec, vm.Name, vm.Namespace, string(vm.UID))
	}

	item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	output := velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: item})
	output.AdditionalItems, err = kvgraph.NewVirtualMachineRestoreGraph(vm)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return output, nil
}

