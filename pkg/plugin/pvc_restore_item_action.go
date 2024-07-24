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

	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PVCRestoreItemAction is a backup item action for restoring DataVolumes
type PVCRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewPVCRestoreItemAction instantiates a PVCRestoreItemAction.
func NewPVCRestoreItemAction(log logrus.FieldLogger) *PVCRestoreItemAction {
	return &PVCRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *PVCRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{"PersistentVolumeClaim"},
		},
		nil
}

//zxh： 备份是如果是处于正在导入的pvc，则跳过恢复
// Execute if the PVC and the corresponding DV is not SUCCESSFULL - then skip PVC
func (p *PVCRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Executing PVCRestoreItemAction")
	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	var pvc corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &pvc); err != nil {
		return nil, errors.WithStack(err)
	}
	p.log.Infof("handling PVC %v/%v", pvc.GetNamespace(), pvc.GetName())
	annotations := pvc.GetAnnotations()
	_, inProgress := annotations[AnnInProgress] //zxh: 备份的时候判断了如果是DV创建的PVC，并且PV处于正在导入状态，则给pvc增加该注解，恢复的是否跳过恢复该pvc
	if inProgress {
		return velero.NewRestoreItemActionExecuteOutput(input.Item).WithoutRestore(), nil
	}

	return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
}
