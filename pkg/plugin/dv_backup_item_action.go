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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
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
	p.log.Debugf("Item: %+v", item.GetObjectKind())
	extra := []velero.ResourceIdentifier{}

	metadata, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, err
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	kind := item.GetObjectKind().GroupVersionKind().Kind
	switch kind {
	case "PersistentVolumeClaim":
		annotations = p.handlePVC(annotations, metadata)
	case "DataVolume":
		annotations, extra = p.handleDataVolume(annotations, metadata)
	}

	metadata.SetAnnotations(annotations)

	return item, extra, nil
}

func (p *DVBackupItemAction) handleDataVolume(annotations map[string]string, metadata metav1.Object) (map[string]string, []velero.ResourceIdentifier) {
	p.log.Infof("handling DataVolume %v/%v", metadata.GetNamespace(), metadata.GetName())
	// TODO: if DV name will ever be different that PVC name, this must be changed
	annotations[AnnPrePopulated] = metadata.GetName()

	extra := []velero.ResourceIdentifier{{
		GroupResource: kuberesource.PersistentVolumeClaims,
		Namespace:     metadata.GetNamespace(),
		Name:          metadata.GetName(),
	}}

	return annotations, extra
}

func (p *DVBackupItemAction) handlePVC(annotations map[string]string, metadata metav1.Object) map[string]string {
	p.log.Infof("handling PVC %v/%v", metadata.GetNamespace(), metadata.GetName())
	for _, or := range metadata.GetOwnerReferences() {
		p.log.Infof("or %+v", or)
		if or.Kind == "DataVolume" {
			annotations[AnnPopulatedFor] = or.Name
			break
		}
	}
	return annotations
}
