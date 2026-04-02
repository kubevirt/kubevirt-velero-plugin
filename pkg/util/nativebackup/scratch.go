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
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// CreateScratchPVC creates a temporary PVC for the native backup to write to (Push mode).
// The PVC is sized to the total capacity of the VM's persistent volumes and uses
// the same storage class as the source (or an override from configuration).
var CreateScratchPVC = func(vm *kvcore.VirtualMachine, backup *v1.Backup, log logrus.FieldLogger) (string, error) {
	cfg := LoadConfig(backup)
	ns := vm.Namespace
	scratchName := ScratchPVCName(backup.Name, vm.Name)

	// Calculate total capacity from VM volumes
	totalCapacity, storageClassName := calculateVolumeCapacity(vm, ns, log)
	if cfg.DefaultScratchStorageClass != "" {
		storageClassName = cfg.DefaultScratchStorageClass
	}

	// Minimum 1Gi scratch PVC
	minCapacity := resource.MustParse("1Gi")
	if totalCapacity.Cmp(minCapacity) < 0 {
		totalCapacity = minCapacity
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scratchName,
			Namespace: ns,
			Labels: map[string]string{
				NativeBackupPVCLabel:  "true",
				ScratchPVCBackupLabel: BackupCRName(backup.Name, vm.Name),
			},
			Annotations: map[string]string{
				ScratchPVCTTLAnnotation: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
				OriginalVolumeAnnotation: strings.Join(
					GetVolumeClaimNames(FilterPersistentVolumes(vm.Spec.Template.Spec.Volumes)),
					",",
				),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: totalCapacity,
				},
			},
		},
	}

	if storageClassName != "" {
		pvc.Spec.StorageClassName = &storageClassName
	}

	log.Infof("Creating scratch PVC %s/%s (capacity: %s, storageClass: %s)",
		ns, scratchName, totalCapacity.String(), storageClassName)

	client, err := util.GetK8sClient()
	if err != nil {
		return "", fmt.Errorf("failed to get k8s client for scratch PVC: %w", err)
	}

	ctx, cancel := apiContext()
	defer cancel()
	_, err = client.CoreV1().PersistentVolumeClaims(ns).Create(
		ctx, pvc, metav1.CreateOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create scratch PVC %s/%s: %w", ns, scratchName, err)
	}

	return scratchName, nil
}

// ScratchPVCName returns a deterministic name for the scratch PVC
func ScratchPVCName(backupName, vmName string) string {
	name := fmt.Sprintf("scratch-%s-%s", backupName, vmName)
	// Kubernetes name limit is 253 chars
	if len(name) > 253 {
		name = name[:253]
	}
	return name
}

// CleanupScratchPVC deletes a specific scratch PVC
var CleanupScratchPVC = func(name, ns string) error {
	client, err := util.GetK8sClient()
	if err != nil {
		return err
	}
	ctx, cancel := apiContext()
	defer cancel()
	return client.CoreV1().PersistentVolumeClaims(ns).Delete(
		ctx, name, metav1.DeleteOptions{},
	)
}

// CleanupScratchPVCsByBackup deletes all scratch PVCs associated with a backup CR name
var CleanupScratchPVCsByBackup = func(ns, backupCRName string) error {
	client, err := util.GetK8sClient()
	if err != nil {
		return err
	}
	ctx, cancel := apiContext()
	defer cancel()
	return client.CoreV1().PersistentVolumeClaims(ns).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: ScratchPVCBackupLabel + "=" + backupCRName},
	)
}

// GarbageCollectStaleScratchPVCs deletes scratch PVCs that have exceeded their TTL
func GarbageCollectStaleScratchPVCs(ns string, log logrus.FieldLogger) error {
	client, err := util.GetK8sClient()
	if err != nil {
		return err
	}

	ctx, cancel := apiContext()
	defer cancel()
	pvcs, err := client.CoreV1().PersistentVolumeClaims(ns).List(
		ctx,
		metav1.ListOptions{LabelSelector: NativeBackupPVCLabel + "=true"},
	)
	if err != nil {
		return err
	}

	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		ttlStr, ok := pvc.Annotations[ScratchPVCTTLAnnotation]
		if !ok {
			continue
		}
		ttl, err := time.Parse(time.RFC3339, ttlStr)
		if err != nil {
			log.Warnf("Invalid TTL annotation on scratch PVC %s/%s: %v", ns, pvc.Name, err)
			continue
		}
		if time.Now().After(ttl) {
			log.Infof("Garbage collecting stale scratch PVC %s/%s (TTL expired)", ns, pvc.Name)
			delCtx, delCancel := apiContext()
			if delErr := client.CoreV1().PersistentVolumeClaims(ns).Delete(
				delCtx, pvc.Name, metav1.DeleteOptions{},
			); delErr != nil {
				log.WithError(delErr).Warnf("Failed to garbage collect scratch PVC %s/%s", ns, pvc.Name)
			}
			delCancel()
		}
	}

	return nil
}

// calculateVolumeCapacity computes the total capacity needed for the scratch PVC
// by summing the sizes of all persistent volumes, and returns the storage class name
// from the first PVC found.
func calculateVolumeCapacity(vm *kvcore.VirtualMachine, ns string, log logrus.FieldLogger) (resource.Quantity, string) {
	total := resource.Quantity{}
	storageClassName := ""
	persistentVolumes := FilterPersistentVolumes(vm.Spec.Template.Spec.Volumes)

	for _, v := range persistentVolumes {
		claimName := ""
		if v.VolumeSource.PersistentVolumeClaim != nil {
			claimName = v.VolumeSource.PersistentVolumeClaim.ClaimName
		} else if v.VolumeSource.DataVolume != nil {
			claimName = v.VolumeSource.DataVolume.Name
		}

		if claimName == "" {
			continue
		}

		pvc, err := util.GetPVC(ns, claimName)
		if err != nil {
			log.WithError(err).Warnf("Failed to get PVC %s/%s for capacity calculation", ns, claimName)
			continue
		}

		if capacity, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			total.Add(capacity)
		}

		if storageClassName == "" && pvc.Spec.StorageClassName != nil {
			storageClassName = *pvc.Spec.StorageClassName
		}
	}

	return total, storageClassName
}
