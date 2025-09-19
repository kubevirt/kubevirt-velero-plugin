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
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// VolumeSnapshotRestoreItemAction is a restore item action for restoring VolumeSnapshots
type VolumeSnapshotRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewVolumeSnapshotRestoreItemAction instantiates a VolumeSnapshotRestoreItemAction.
func NewVolumeSnapshotRestoreItemAction(log logrus.FieldLogger) *VolumeSnapshotRestoreItemAction {
	return &VolumeSnapshotRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VolumeSnapshotRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"VolumeSnapshot",
			},
		},
		nil
}

// Execute cleans up PVC UID labels added during backup for VolumeSnapshots
func (p *VolumeSnapshotRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Executing VolumeSnapshotRestoreItemAction")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	var volumeSnapshot snapshotv1.VolumeSnapshot
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &volumeSnapshot); err != nil {
		p.log.WithError(err).Error("Failed to convert unstructured item to VolumeSnapshot")
		return nil, errors.WithStack(err)
	}


	// Remove PVC UID labels added during backup
	if volumeSnapshot.Labels != nil {
		if _, exists := volumeSnapshot.Labels[util.PVCUIDLabel]; exists {
			// Check if we preserved an original value
			if volumeSnapshot.Annotations != nil {
				if originalValue, hasOriginal := volumeSnapshot.Annotations[util.OriginalVolumeSnapshotUIDAnnotation]; hasOriginal {
					// Restore the original value
					volumeSnapshot.Labels[util.PVCUIDLabel] = originalValue
					delete(volumeSnapshot.Annotations, util.OriginalVolumeSnapshotUIDAnnotation)
				} else {
					// No original value to restore - remove the plugin-added label completely
					delete(volumeSnapshot.Labels, util.PVCUIDLabel)
				}
			} else {
				// No annotations - remove the plugin-added label
				delete(volumeSnapshot.Labels, util.PVCUIDLabel)
			}
		}
	}

	// Convert back to unstructured
	item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&volumeSnapshot)
	if err != nil {
		p.log.WithError(err).Error("Failed to convert VolumeSnapshot back to unstructured")
		return nil, errors.WithStack(err)
	}
	return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: item}), nil
}