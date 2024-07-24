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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

const (
	AnnPrePopulated = "cdi.kubevirt.io/storage.prePopulated"
	AnnPopulatedFor = "cdi.kubevirt.io/storage.populatedFor" //zxh: 标记pvc已经备份DV成功填充，成功填充之后CDI会自动给pvc上添加这个注解，可以防止重复填充
	AnnInProgress   = "kvp.kubevirt.io/storage.inprogress"
)

// DVBackupItemAction is a backup item action for backing up DataVolumes
type DVBackupItemAction struct {
	log logrus.FieldLogger
}

// NewDVBackupItemAction instantiates a DVBackupItemAction.
func NewDVBackupItemAction(log logrus.FieldLogger) *DVBackupItemAction {
	return &DVBackupItemAction{log: log}
}

/*
**/
// AppliesTo returns information about which resources this action should be invoked for.
// The IncludedResources and ExcludedResources slices can include both resources
// and resources with group names. These work: "ingresses", "ingresses.extensions".
// A DVBackupItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.
func (p *DVBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"PersistentVolumeClaim",
				"DataVolume",
			},
		},
		nil
}

// Execute allows the ItemAction to perform arbitrary logic with the item being backed up,
// in this case, setting a custom annotation on the item being backed up.
func (p *DVBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing DVBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	extra := []velero.ResourceIdentifier{}

	kind := item.GetObjectKind().GroupVersionKind().Kind
	switch kind {
	case "PersistentVolumeClaim":
		return p.handlePVC(item)
	case "DataVolume":
		return p.handleDataVolume(backup, item)
	}

	return item, extra, nil
}

func (p *DVBackupItemAction) handlePVC(item runtime.Unstructured) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	metadata, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, err
	}
	p.log.Infof("handling PVC %v/%v", metadata.GetNamespace(), metadata.GetName())

	dv, err := p.getOwningDataVolume(metadata)
	if err != nil {
		return nil, nil, err
	}
	if dv != nil {
		annotations := metadata.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		 //zxh: 如果DV已经是完成状态，则表明PVC已经被成功填充了，不会再重复填充了。
		// 这里定义注解和实际环境中和的pvc被成功填充之后的注解不一样，需要判断一下是否是因为升级导致不一致，还是就是不一样的注解呢
		if dv.Status.Phase == cdiv1.Succeeded {
			annotations[AnnPopulatedFor] = dv.Name
		} else {
			// The PVC is not finished, we mark it as inprogress, so it can be skipped during restore
			// so it does not conflict with CDI action
			annotations[AnnInProgress] = dv.Name //zxh：备份的时候如果DV没有填充完成，则标记为inprogress
		}
		metadata.SetAnnotations(annotations)
	}

	extra := []velero.ResourceIdentifier{}
	return item, extra, nil
}

func (p *DVBackupItemAction) handleDataVolume(backup *v1.Backup, item runtime.Unstructured) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	var dv cdiv1.DataVolume
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &dv); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	p.log.Infof("handling DataVolume %v/%v", dv.GetNamespace(), dv.GetName())
	extra := []velero.ResourceIdentifier{}
	dvSucceeded := dv.Status.Phase == cdiv1.Succeeded
	if dvSucceeded {
		annotations := dv.GetAnnotations() //zxh: 标记pvc已经完成
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[AnnPrePopulated] = dv.GetName()
		dv.SetAnnotations(annotations) 

		extra = []velero.ResourceIdentifier{{
			GroupResource: kuberesource.PersistentVolumeClaims,
			Namespace:     dv.GetNamespace(),
			Name:          dv.GetName(),
		}}
	}

	//zxh： DV如果没有处于成功填充状态也会进行备份
	dvMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&dv)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: dvMap}, extra, nil
}

//zxh: 解析pvc是否是DV创建的，如果是DV创建的则返回DV
func (p *DVBackupItemAction) getOwningDataVolume(metadata metav1.Object) (*cdiv1.DataVolume, error) {
	for _, or := range metadata.GetOwnerReferences() {
		p.log.Infof("or %+v", or)
		if or.Kind == "DataVolume" {
			dv, err := util.GetDV(metadata.GetNamespace(), or.Name)
			if err != nil {
				return nil, err
			}
			return dv, nil
		}
	}
	return nil, nil
}
