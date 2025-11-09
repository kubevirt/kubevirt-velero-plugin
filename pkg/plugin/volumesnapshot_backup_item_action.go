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
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// VolumeSnapshotBackupItemAction is a backup item action for backing up VolumeSnapshots
type VolumeSnapshotBackupItemAction struct {
	log    logrus.FieldLogger
	client kubernetes.Interface
	// Cache PVCs by namespace to avoid repeated API calls
	namespacePVCs map[string]map[string]string // namespace -> pvcName -> pvcUID
}

// NewVolumeSnapshotBackupItemAction instantiates a VolumeSnapshotBackupItemAction.
func NewVolumeSnapshotBackupItemAction(log logrus.FieldLogger) *VolumeSnapshotBackupItemAction {
	client, err := util.GetK8sClient()
	if err != nil {
		log.WithError(err).Error("Failed to get Kubernetes client")
		// Return a basic action that will handle errors during execute
		return &VolumeSnapshotBackupItemAction{
			log:           log,
			namespacePVCs: make(map[string]map[string]string),
		}
	}

	return &VolumeSnapshotBackupItemAction{
		log:           log,
		client:        client,
		namespacePVCs: make(map[string]map[string]string),
	}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VolumeSnapshotBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"VolumeSnapshot",
			},
		},
		nil
}

// Execute allows the ItemAction to perform arbitrary logic with the item being backed up,
// in this case, adding PVC UID labels to VolumeSnapshots for selective restore functionality.
func (p *VolumeSnapshotBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing VolumeSnapshotBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	var volumeSnapshot snapshotv1.VolumeSnapshot
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &volumeSnapshot); err != nil {
		return nil, nil, errors.WithStack(err)
	}

  extra := []velero.ResourceIdentifier{}

	// Check if the VolumeSnapshot has a source PVC
	if volumeSnapshot.Spec.Source.PersistentVolumeClaimName == nil {
		return item, extra, nil
	}

	pvcName := *volumeSnapshot.Spec.Source.PersistentVolumeClaimName

	// Get the PVC UID efficiently using cache
	pvcUID, err := p.getPVCUID(volumeSnapshot.GetNamespace(), pvcName)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	if pvcUID == "" {
		return item, extra, nil
	}

	// Add PVC UID label for selective restore
	labels := volumeSnapshot.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	// Handle collision detection - preserve original value if it exists
	// Even if the existing value matches the PVC UID, we need to preserve it
	// because the user might have legitimately set this label themselves
	if existingValue, exists := labels[util.PVCUIDLabel]; exists {
		annotations := volumeSnapshot.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[util.OriginalVolumeSnapshotUIDAnnotation] = existingValue
		volumeSnapshot.SetAnnotations(annotations)
	}

	labels[util.PVCUIDLabel] = pvcUID
	volumeSnapshot.SetLabels(labels)

	vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&volumeSnapshot)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: vsMap}, extra, nil
}

// getPVCUID efficiently retrieves the UID of a PVC using namespace-level caching
func (p *VolumeSnapshotBackupItemAction) getPVCUID(namespace, pvcName string) (string, error) {
	// Check if we have this namespace cached
	if namespacePVCs, exists := p.namespacePVCs[namespace]; exists {
		if pvcUID, found := namespacePVCs[pvcName]; found {
			return pvcUID, nil
		}
		// PVC not found in cache but namespace is cached, so it doesn't exist
		return "", fmt.Errorf("PVC %s not found in namespace %s", pvcName, namespace)
	}

	// Namespace not cached yet, fetch all PVCs in the namespace at once
	if p.client == nil {
		return "", fmt.Errorf("Kubernetes client not available")
	}

	pvcList, err := p.client.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", errors.WithStack(err)
	}

	// Initialize the namespace cache
	p.namespacePVCs[namespace] = make(map[string]string)

	// Cache all PVCs in this namespace
	for _, pvc := range pvcList.Items {
		p.namespacePVCs[namespace][pvc.Name] = string(pvc.UID)
	}

	// Now look up the specific PVC
	if pvcUID, found := p.namespacePVCs[namespace][pvcName]; found {
		return pvcUID, nil
	}

	return "", fmt.Errorf("PVC %s not found in namespace %s", pvcName, namespace)
}

