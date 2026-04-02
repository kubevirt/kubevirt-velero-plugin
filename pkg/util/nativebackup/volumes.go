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

package nativebackup

import (
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	kvcore "kubevirt.io/api/core/v1"
)

// FilterPersistentVolumes returns only volumes that need native backup.
// Skips ephemeral volumes: ConfigMap, Secret, CloudInit, ServiceAccount, DownwardAPI, Sysprep.
func FilterPersistentVolumes(volumes []kvcore.Volume) []kvcore.Volume {
	var persistent []kvcore.Volume
	for _, v := range volumes {
		if v.VolumeSource.DataVolume != nil ||
			v.VolumeSource.PersistentVolumeClaim != nil ||
			v.VolumeSource.MemoryDump != nil {
			persistent = append(persistent, v)
		}
		// ContainerDisk is ephemeral (backed by container image), skip it
		// ConfigMap, Secret, CloudInitNoCloud, CloudInitConfigDrive,
		// ServiceAccount, DownwardMetrics, Sysprep are all ephemeral
	}
	return persistent
}

// GetVolumeClaimNames extracts PVC/DV claim names from a list of volumes
func GetVolumeClaimNames(volumes []kvcore.Volume) []string {
	var names []string
	for _, v := range volumes {
		if v.VolumeSource.PersistentVolumeClaim != nil {
			names = append(names, v.VolumeSource.PersistentVolumeClaim.ClaimName)
		}
		if v.VolumeSource.DataVolume != nil {
			names = append(names, v.VolumeSource.DataVolume.Name)
		}
	}
	return names
}

// ExcludeNativeBackedPVCs removes PVC ResourceIdentifiers that are handled by native backup,
// preventing CSI double-snapshots of the same volumes.
func ExcludeNativeBackedPVCs(
	items []velero.ResourceIdentifier,
	nativeVolumes []kvcore.Volume,
) []velero.ResourceIdentifier {
	nativeNames := make(map[string]bool)
	for _, v := range nativeVolumes {
		if v.VolumeSource.PersistentVolumeClaim != nil {
			nativeNames[v.VolumeSource.PersistentVolumeClaim.ClaimName] = true
		}
		if v.VolumeSource.DataVolume != nil {
			nativeNames[v.VolumeSource.DataVolume.Name] = true
		}
	}

	var filtered []velero.ResourceIdentifier
	for _, item := range items {
		if item.Resource == "persistentvolumeclaims" && nativeNames[item.Name] {
			continue // Handled by native backup
		}
		filtered = append(filtered, item)
	}
	return filtered
}
