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
	"context"
	"fmt"

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
	existing, err := client.Resource(vmBackupTrackerGVR).Namespace(ns).Get(
		context.TODO(), trackerName, metav1.GetOptions{},
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

	created, err := client.Resource(vmBackupTrackerGVR).Namespace(ns).Create(
		context.TODO(), obj, metav1.CreateOptions{},
	)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			// Race condition: another backup created it
			return client.Resource(vmBackupTrackerGVR).Namespace(ns).Get(
				context.TODO(), trackerName, metav1.GetOptions{},
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
			return SourceRef{
				APIGroup: "kubevirt.io",
				Kind:     "VirtualMachine",
				Name:     vm.Name,
			}, true, nil
		}
	}

	// Incremental via tracker
	log.Infof("Using incremental backup via tracker for VM %s/%s", vm.Namespace, vm.Name)
	return SourceRef{
		APIGroup: "backup.kubevirt.io",
		Kind:     "VirtualMachineBackupTracker",
		Name:     TrackerName(vm.Name),
	}, false, nil
}

// getIncrementalCount returns the number of incremental backups since the last full.
// This is approximated by checking the latestCheckpoint's creation time vs tracker creation.
func getIncrementalCount(tracker *unstructured.Unstructured) int {
	// Check if latestCheckpoint has volumes (indicates at least one successful backup)
	volumes, found, _ := unstructured.NestedSlice(
		tracker.Object, "status", "latestCheckpoint", "volumes",
	)
	if !found || len(volumes) == 0 {
		return 0
	}
	// For now, we count the presence of a checkpoint as 1 incremental.
	// A more sophisticated approach would track count in an annotation.
	return 1
}
