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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// CheckProgress checks the VirtualMachineBackup CR conditions and returns progress.
func CheckProgress(operationID string, log logrus.FieldLogger) (velero.OperationProgress, error) {
	ns, name, err := ParseOperationID(operationID)
	if err != nil {
		return velero.OperationProgress{}, err
	}

	vmBackup, err := GetVMBackup(ns, name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// CR was deleted (cancelled or cleaned up)
			return velero.OperationProgress{
				Completed: true,
				Err:       "VirtualMachineBackup CR not found (may have been cancelled)",
			}, nil
		}
		return velero.OperationProgress{}, fmt.Errorf("failed to get VirtualMachineBackup %s: %w", operationID, err)
	}

	conditions, found, _ := unstructured.NestedSlice(vmBackup.Object, "status", "conditions")
	if !found {
		// No conditions yet — still initializing
		return velero.OperationProgress{
			Completed:   false,
			NTotal:      1,
			Description: "Native backup initializing",
			Updated:     time.Now(),
		}, nil
	}

	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)

		if condType == "Ready" && condStatus == "True" {
			return buildCompletedProgress(vmBackup, name, log), nil
		}

		if condType == "Failure" && condStatus == "True" {
			message, _ := cond["message"].(string)
			reason, _ := cond["reason"].(string)
			errMsg := "native backup failed"
			if message != "" || reason != "" {
				errMsg = fmt.Sprintf("native backup failed: %s (%s)", message, reason)
			}
			return velero.OperationProgress{
				Completed: true,
				Err:       errMsg,
				Updated:   time.Now(),
			}, nil
		}
	}

	// Still in progress
	return velero.OperationProgress{
		Completed:   false,
		NTotal:      1,
		Description: "Native backup in progress",
		Updated:     time.Now(),
	}, nil
}

// CancelAndCleanup deletes the VirtualMachineBackup CR and its scratch PVCs
func CancelAndCleanup(operationID string, log logrus.FieldLogger) error {
	ns, name, err := ParseOperationID(operationID)
	if err != nil {
		return err
	}

	log.Infof("Cancelling native backup %s/%s", ns, name)

	// Delete the VirtualMachineBackup CR (cancels in-progress backup)
	if err := DeleteVMBackupCR(ns, name); err != nil && !k8serrors.IsNotFound(err) {
		log.WithError(err).Warn("Failed to delete VirtualMachineBackup CR during cancel")
	}

	// Clean up scratch PVCs
	if err := CleanupScratchPVCsByBackup(ns, name); err != nil {
		log.WithError(err).Warn("Failed to clean up scratch PVCs during cancel")
	}

	return nil
}

// GetBackupMetadata extracts metadata from a completed VirtualMachineBackup for annotation
func GetBackupMetadata(vmBackup *unstructured.Unstructured) (backupType, checkpoint, volumes string) {
	backupType, _, _ = unstructured.NestedString(vmBackup.Object, "status", "type")
	checkpoint, _, _ = unstructured.NestedString(vmBackup.Object, "status", "checkpointName")

	includedVolumes, found, _ := unstructured.NestedSlice(vmBackup.Object, "status", "includedVolumes")
	if found {
		var volumeNames []string
		for _, v := range includedVolumes {
			vol, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			if name, ok := vol["volumeName"].(string); ok {
				volumeNames = append(volumeNames, name)
			}
		}
		volumes = strings.Join(volumeNames, ",")
	}

	return
}

func buildCompletedProgress(vmBackup *unstructured.Unstructured, name string, log logrus.FieldLogger) velero.OperationProgress {
	backupType, checkpoint, volumes := GetBackupMetadata(vmBackup)

	includedVolumes, _, _ := unstructured.NestedSlice(vmBackup.Object, "status", "includedVolumes")
	volumeCount := int64(len(includedVolumes))
	if volumeCount == 0 {
		volumeCount = 1
	}

	log.Infof("Native backup %s completed: type=%s, checkpoint=%s, volumes=%s",
		name, backupType, checkpoint, volumes)

	return velero.OperationProgress{
		Completed:      true,
		NCompleted:     volumeCount,
		NTotal:         volumeCount,
		OperationUnits: "Volumes",
		Description:    fmt.Sprintf("Native %s backup complete (%d volumes)", backupType, volumeCount),
		Updated:        time.Now(),
	}
}
