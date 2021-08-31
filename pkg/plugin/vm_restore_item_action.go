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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kvcore "kubevirt.io/client-go/api/v1"
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

	vm := new(kvcore.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), vm); err != nil {
		return nil, errors.WithStack(err)
	}

	item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	output := velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: item})
	for _, dv := range vm.Spec.DataVolumeTemplates {
		output.AdditionalItems = append(output.AdditionalItems, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{
				Group:    "cdi.kubevirt.io",
				Resource: "datavolumes",
			},
			Name:      dv.Name,
			Namespace: vm.Namespace,
		})
	}

	return output, nil
}
