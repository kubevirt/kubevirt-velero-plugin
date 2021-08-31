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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	kvcore "kubevirt.io/client-go/api/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// VMBackupItemAction is a backup item action for backing up DataVolumes
type VMBackupItemAction struct {
	log logrus.FieldLogger
}

// NewVMBackupItemAction instantiates a VMBackupItemAction.
func NewVMBackupItemAction(log logrus.FieldLogger) *VMBackupItemAction {
	return &VMBackupItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
// The IncludedResources and ExcludedResources slices can include both resources
// and resources with group names. These work: "ingresses", "ingresses.extensions".
// A VMBackupItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.
func (p *VMBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"VirtualMachine",
			},
		},
		nil
}

// Execute returns VM's DataVolumes as extra items to back up.
func (p *VMBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMBackupItemAction")
	extra := []velero.ResourceIdentifier{}

	vm := new(kvcore.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vm); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	if !canBeSafelyBackedUp(vm, backup) {
		return nil, nil, fmt.Errorf("VM cannot be safely backed up")
	}

	for _, template := range vm.Spec.DataVolumeTemplates {
		namespace := template.GetNamespace()
		if namespace == "" {
			namespace = vm.GetNamespace()
		}
		p.log.Infof("Adding DV to backup: %s/%s", namespace, template.GetName())
		extra = append(extra, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
			Namespace:     namespace,
			Name:          template.GetName(),
		})
	}

	if vm.Status.Created {
		extra = append(extra, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"},
			Namespace:     vm.GetNamespace(),
			Name:          vm.GetName(),
		})
	}

	return item, extra, nil
}

func canBeSafelyBackedUp(vm *kvcore.VirtualMachine, backup *v1.Backup) bool {
	isRuning := vm.Status.PrintableStatus == kvcore.VirtualMachineStatusStarting || vm.Status.PrintableStatus == kvcore.VirtualMachineStatusRunning
	if !isRuning {
		return true
	}

	hasIncludeResources := len(backup.Spec.IncludedResources) > 0
	if !hasIncludeResources {
		return true
	}

	return util.IsResourceIncluded("virtualmachineinstances", backup) &&
		util.IsResourceIncluded("pods", backup)
}
