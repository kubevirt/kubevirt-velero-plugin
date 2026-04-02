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
	"strconv"

	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	kvcore "kubevirt.io/api/core/v1"
)

var vmBackupTrackerGVR = schema.GroupVersionResource{
	Group:    "backup.kubevirt.io",
	Version:  "v1alpha1",
	Resource: "virtualmachinebackuptrackers",
}

// TrackerName returns a deterministic name for a backup tracker
func TrackerName(vmName string) string {
	name := fmt.Sprintf("%s-backup-tracker", vmName)
	if len(name) > 253 {
		name = name[:253]
	}
	return name
}

// EnsureTracker creates a VirtualMachineBackupTracker if it doesn't exist,
// or returns the existing one. Trackers are created once per VM and persist
// across backups to maintain checkpoint state for incremental backups.
var EnsureTracker = func(vmName, ns string) (*unstructured.Unstructured, error) {
	trackerName := TrackerName(vmName)
	client, err := getDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic client for tracker: %w", err)
	}

	// Try to get existing tracker
	ctx, cancel := apiContext()
	defer cancel()
	existing, err := client.Resource(vmBackupTrackerGVR).Namespace(ns).Get(
		ctx, trackerName, metav1.GetOptions{},
	)
	if err == nil {
		return existing, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check tracker %s/%s: %w", ns, trackerName, err)
	}

	// Create new tracker
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "backup.kubevirt.io/v1alpha1",
			"kind":       "VirtualMachineBackupTracker",
			"metadata": map[string]interface{}{
				"name":      trackerName,
				"namespace": ns,
				"annotations": map[string]interface{}{
					IncrementalCountAnnotation: "0",
				},
			},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"apiGroup": "kubevirt.io",
					"kind":     "VirtualMachine",
					"name":     vmName,
				},
			},
		},
	}

	createCtx, createCancel := apiContext()
	defer createCancel()
	created, err := client.Resource(vmBackupTrackerGVR).Namespace(ns).Create(
		createCtx, obj, metav1.CreateOptions{},
	)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			// Race condition: another backup created it
			getCtx, getCancel := apiContext()
			defer getCancel()
			return client.Resource(vmBackupTrackerGVR).Namespace(ns).Get(
				getCtx, trackerName, metav1.GetOptions{},
			)
		}
		return nil, fmt.Errorf("failed to create tracker %s/%s: %w", ns, trackerName, err)
	}

	return created, nil
}

// IsCheckpointHealthy checks if the tracker's checkpoint is valid for incremental backup.
// Returns false if:
// - checkpointRedefinitionRequired is true (VM restarted, libvirt checkpoint invalid)
// - no latestCheckpoint exists (first backup, or checkpoint cleared)
func IsCheckpointHealthy(tracker *unstructured.Unstructured) bool {
	if tracker == nil {
		return false
	}

	// Check checkpointRedefinitionRequired flag
	required, found, _ := unstructured.NestedBool(
		tracker.Object, "status", "checkpointRedefinitionRequired",
	)
	if found && required {
		return false
	}

	// Check latestCheckpoint exists
	checkpoint, found, _ := unstructured.NestedMap(tracker.Object, "status", "latestCheckpoint")
	return found && checkpoint != nil
}

// ResolveSource determines the backup source (VM for full, Tracker for incremental)
// and whether a full backup should be forced.
func ResolveSource(
	vm *kvcore.VirtualMachine,
	backup *v1.Backup,
	log logrus.FieldLogger,
) (source SourceRef, forceFullBackup bool, err error) {
	cfg := LoadConfig(backup)

	// If incremental is not enabled, always do full backup via VM source
	if !cfg.IncrementalEnabled {
		return SourceRef{
			APIGroup: "kubevirt.io",
			Kind:     "VirtualMachine",
			Name:     vm.Name,
		}, false, nil
	}

	// Ensure tracker exists
	tracker, err := EnsureTracker(vm.Name, vm.Namespace)
	if err != nil {
		return SourceRef{}, false, err
	}

	// Check if checkpoint needs redefinition (VM restarted)
	if !IsCheckpointHealthy(tracker) {
		log.Infof("Tracker checkpoint unhealthy for VM %s/%s (VM may have restarted), forcing full backup",
			vm.Namespace, vm.Name)
		// Reset counter on full backup
		updateIncrementalCount(tracker, 0, vm.Namespace, log)
		return SourceRef{
			APIGroup: "kubevirt.io",
			Kind:     "VirtualMachine",
			Name:     vm.Name,
		}, true, nil
	}

	// Check forceFullEveryN
	if cfg.ForceFullEveryN > 0 {
		count := getIncrementalCount(tracker)
		if count >= cfg.ForceFullEveryN {
			log.Infof("Forcing full backup for VM %s/%s after %d incrementals",
				vm.Namespace, vm.Name, count)
			// Reset counter on full backup
			updateIncrementalCount(tracker, 0, vm.Namespace, log)
			return SourceRef{
				APIGroup: "kubevirt.io",
				Kind:     "VirtualMachine",
				Name:     vm.Name,
			}, true, nil
		}
	}

	// Incremental via tracker — increment counter
	count := getIncrementalCount(tracker)
	updateIncrementalCount(tracker, count+1, vm.Namespace, log)

	log.Infof("Using incremental backup via tracker for VM %s/%s (incremental #%d)", vm.Namespace, vm.Name, count+1)
	return SourceRef{
		APIGroup: "backup.kubevirt.io",
		Kind:     "VirtualMachineBackupTracker",
		Name:     TrackerName(vm.Name),
	}, false, nil
}

// getIncrementalCount reads the incremental backup counter from the tracker annotation.
func getIncrementalCount(tracker *unstructured.Unstructured) int {
	annotations, _, _ := unstructured.NestedStringMap(tracker.Object, "metadata", "annotations")
	if annotations == nil {
		return 0
	}
	countStr, ok := annotations[IncrementalCountAnnotation]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(countStr)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// updateIncrementalCount writes the incremental counter annotation on the tracker CR.
func updateIncrementalCount(tracker *unstructured.Unstructured, count int, ns string, log logrus.FieldLogger) {
	annotations, _, _ := unstructured.NestedStringMap(tracker.Object, "metadata", "annotations")
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[IncrementalCountAnnotation] = strconv.Itoa(count)

	// Update in-memory object
	_ = unstructured.SetNestedStringMap(tracker.Object, annotations, "metadata", "annotations")

	// Persist to cluster
	client, err := getDynamicClient()
	if err != nil {
		log.WithError(err).Warn("Failed to get dynamic client for tracker annotation update")
		return
	}

	ctx, cancel := apiContext()
	defer cancel()
	_, err = client.Resource(vmBackupTrackerGVR).Namespace(ns).Update(
		ctx, tracker, metav1.UpdateOptions{},
	)
	if err != nil {
		log.WithError(err).Warnf("Failed to update incremental count annotation on tracker (count=%d)", count)
	}
}
