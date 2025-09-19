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
 * Copyright 2025 Red Hat, Inc.
 *
 */

package plugin

import (
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// PVCBackupItemAction is a backup item action for backing up PersistentVolumeClaims
type PVCBackupItemAction struct {
	log logrus.FieldLogger
}

// NewPVCBackupItemAction instantiates a PVCBackupItemAction.
func NewPVCBackupItemAction(log logrus.FieldLogger) *PVCBackupItemAction {
	return &PVCBackupItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *PVCBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"PersistentVolumeClaim",
			},
		},
		nil
}

// Execute allows the ItemAction to perform arbitrary logic with the item being backed up,
// in this case, adding UID labels to PVCs for selective restore functionality.
func (p *PVCBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing PVCBackupItemAction")

	metadata, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, err
	}

	// Add UID label for selective restore
	labels := metadata.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	pvcUID := string(metadata.GetUID())
	if pvcUID == "" {
		extra := []velero.ResourceIdentifier{}
		return item, extra, nil
	}

	// Handle collision detection - preserve original value if it exists
	// Even if the existing value matches the UID, we need to preserve it
	// because the user might have legitimately set this label themselves
	if existingValue, exists := labels[util.PVCUIDLabel]; exists {
		annotations := metadata.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[util.OriginalPVCUIDAnnotation] = existingValue
		metadata.SetAnnotations(annotations)
	}

	labels[util.PVCUIDLabel] = pvcUID
	metadata.SetLabels(labels)

	extra := []velero.ResourceIdentifier{}
	return item, extra, nil
}
