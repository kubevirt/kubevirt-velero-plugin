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
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// VMTItemBlockAction is an item block action for VirtualMachineTemplates
type VMTItemBlockAction struct {
	log logrus.FieldLogger
}

// NewVMTItemBlockAction instantiates a VMTItemBlockAction.
func NewVMTItemBlockAction(log logrus.FieldLogger) *VMTItemBlockAction {
	return &VMTItemBlockAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMTItemBlockAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"virtualmachinetemplates.template.kubevirt.io",
		},
	}, nil
}

// GetRelatedItems returns the related items for the VirtualMachineTemplate using the backup graph.
func (p *VMTItemBlockAction) GetRelatedItems(item runtime.Unstructured, backup *v1.Backup) ([]velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMTItemBlockAction GetRelatedItems")
	return kvgraph.NewObjectBackupGraph(item)
}

func (p *VMTItemBlockAction) Name() string {
	return "VMTItemBlockAction"
}
