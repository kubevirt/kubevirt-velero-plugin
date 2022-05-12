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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime"
	//kvcore "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

const (
	AnnPrePopulated = "cdi.kubevirt.io/storage.prePopulated"
	AnnPopulatedFor = "cdi.kubevirt.io/storage.populatedFor"
)

// DVBackupItemAction is a backup item action for backing up DataVolumes
type DVBackupItemAction struct {
	log logrus.FieldLogger
}

// NewDVBackupItemAction instantiates a DVBackupItemAction.
func NewDVBackupItemAction(log logrus.FieldLogger) *DVBackupItemAction {
	return &DVBackupItemAction{log: log}
}

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

func (p *DVBackupItemAction) handleDataVolume(backup *v1.Backup, item runtime.Unstructured) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	var dv cdiv1.DataVolume
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &dv); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	p.log.Infof("handling DataVolume %v/%v", dv.GetNamespace(), dv.GetName())
	dvSucceeded := dv.Status.Phase == cdiv1.Succeeded && util.IsResourceInBackup("persistentvolumeclaims", backup)
	if !dvSucceeded {
		// PVC not in backup, that means user only wants DV. PVC can be recreated on restore
		// so - do not add the PrePopulated
		extra := []velero.ResourceIdentifier{}
		return item, extra, nil
	}
	annotations := dv.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// TODO: if DV name will ever be different that PVC name, this must be changed
	annotations[AnnPrePopulated] = dv.GetName()
	dv.SetAnnotations(annotations)

	extra := []velero.ResourceIdentifier{{
		GroupResource: kuberesource.PersistentVolumeClaims,
		Namespace:     dv.GetNamespace(),
		Name:          dv.GetName(),
	}}

	dvMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&dv)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: dvMap}, extra, nil
}

func (p *DVBackupItemAction) handlePVC(item runtime.Unstructured) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	// TODO: handle not finished PVC
	//var pvc corev1api.PersistentVolumeClaim
	//if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &pvc); err != nil {
	//	return nil, nil, errors.WithStack(err)
	//}

	metadata, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, err
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	p.log.Infof("handling PVC %v/%v", metadata.GetNamespace(), metadata.GetName())
	for _, or := range metadata.GetOwnerReferences() {
		p.log.Infof("or %+v", or)
		if or.Kind == "DataVolume" {
			// get DV, if finished then ..., else annotate with some skip!...
			annotations[AnnPopulatedFor] = or.Name
			break
		}
	}

	metadata.SetAnnotations(annotations)

	extra := []velero.ResourceIdentifier{}
	return item, extra, nil
}
