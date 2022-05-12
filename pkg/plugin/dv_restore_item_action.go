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
 * Copyright 2022 Red Hat, Inc.
 *
 */

package plugin

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
)

// DVRestoreItemAction is a backup item action for restoring DataVolumes
type DVRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewDVRestoreItemAction instantiates a DVRestoreItemAction.
func NewDVRestoreItemAction(log logrus.FieldLogger) *DVRestoreItemAction {
	return &DVRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *DVRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"DataVolume",
			},
		},
		nil
}

// Execute - if the DV is not SUCCESSFULL - then reset the phase
func (p *DVRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Executing DVRestoreItemAction")
	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	var dv cdiv1.DataVolume
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &dv); err != nil {
		return nil, errors.WithStack(err)
	}

	p.log.Infof("handling DV %v/%v", dv.GetNamespace(), dv.GetName())

	if dv.Status.Phase != cdiv1.Succeeded {
		dv.Status.Phase = cdiv1.Unknown

		item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dv)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: item}), nil
	}

	return velero.NewRestoreItemActionExecuteOutput(input.Item), nil

}
