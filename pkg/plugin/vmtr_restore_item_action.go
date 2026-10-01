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

	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// VMTRRestoreItemAction is a restore item action for VirtualMachineTemplateRequests
type VMTRRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewVMTRRestoreItemAction instantiates a VMTRRestoreItemAction.
func NewVMTRRestoreItemAction(log logrus.FieldLogger) *VMTRRestoreItemAction {
	return &VMTRRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMTRRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"virtualmachinetemplaterequests.template.kubevirt.io",
		},
	}, nil
}

// Execute – A VirtualMachineTemplateRequest is a one-shot job: it snapshots its source
// VirtualMachine and clones the result into a new VirtualMachineTemplate. Velero clears an
// object's status on restore, so a restored request would lose its "Progressing"/"Ready"
// conditions and be reconciled again, re-snapshotting the (possibly now-different) source VM
// and failing with AlreadyExists against the VirtualMachineTemplate that was just restored.
// Its spec is also immutable, so there is no way to recover from that. The request should
// therefore always be skipped; the VirtualMachineTemplate it already produced is restored on
// its own.
func (p *VMTRRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running VMTRRestoreItemAction")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	return velero.NewRestoreItemActionExecuteOutput(input.Item).WithoutRestore(), nil
}
