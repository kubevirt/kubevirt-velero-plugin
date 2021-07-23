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
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// VIMRestorePlugin is a VMI restore item action plugin for Velero (duh!)
type PodRestorePlugin struct {
	log logrus.FieldLogger
}

// NewPodRestorePlugin instantiates a RestorePlugin.
func NewPodRestoreItemAction(log logrus.FieldLogger) *PodRestorePlugin {
	return &PodRestorePlugin{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *PodRestorePlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"Pod",
		},
		LabelSelector: "kubevirt.io=virt-launcher",
	}, nil
}

// Execute – Launcher Pod should be unconditionally skipped
func (p *PodRestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running PodRestorePlugin")

	return velero.NewRestoreItemActionExecuteOutput(input.Item).WithoutRestore(), nil
}
