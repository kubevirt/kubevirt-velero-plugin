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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvcore "kubevirt.io/api/core/v1"
)

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		backup   *v1.Backup
		expected bool
	}{
		{
			name:     "nil backup",
			backup:   nil,
			expected: false,
		},
		{
			name: "backup without label",
			backup: &v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "backup with native-backup label",
			backup: &v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						NativeBackupLabel: "true",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEnabled(tt.backup)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupCRName(t *testing.T) {
	assert.Equal(t, "velero-my-backup-my-vm", BackupCRName("my-backup", "my-vm"))

	// Test length limit
	longName := ""
	for i := 0; i < 300; i++ {
		longName += "a"
	}
	result := BackupCRName(longName, "vm")
	assert.LessOrEqual(t, len(result), 253)
}

func TestScratchPVCName(t *testing.T) {
	assert.Equal(t, "scratch-my-backup-my-vm", ScratchPVCName("my-backup", "my-vm"))
}

func TestTrackerName(t *testing.T) {
	assert.Equal(t, "my-vm-backup-tracker", TrackerName("my-vm"))
}

func TestParseOperationID(t *testing.T) {
	ns, name := ParseOperationID("default/velero-backup-vm1")
	assert.Equal(t, "default", ns)
	assert.Equal(t, "velero-backup-vm1", name)

	ns, name = ParseOperationID("no-slash")
	assert.Equal(t, "", ns)
	assert.Equal(t, "no-slash", name)
}

func TestFilterPersistentVolumes(t *testing.T) {
	volumes := []kvcore.Volume{
		{
			Name: "rootdisk",
			VolumeSource: kvcore.VolumeSource{
				DataVolume: &kvcore.DataVolumeSource{Name: "rootdisk-dv"},
			},
		},
		{
			Name: "datadisk",
			VolumeSource: kvcore.VolumeSource{
				PersistentVolumeClaim: &kvcore.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"},
				},
			},
		},
		{
			Name: "cloudinit",
			VolumeSource: kvcore.VolumeSource{
				CloudInitNoCloud: &kvcore.CloudInitNoCloudSource{UserData: "test"},
			},
		},
		{
			Name: "configmap",
			VolumeSource: kvcore.VolumeSource{
				ConfigMap: &kvcore.ConfigMapVolumeSource{},
			},
		},
	}

	persistent := FilterPersistentVolumes(volumes)
	assert.Len(t, persistent, 2)
	assert.Equal(t, "rootdisk", persistent[0].Name)
	assert.Equal(t, "datadisk", persistent[1].Name)
}

func TestGetVolumeClaimNames(t *testing.T) {
	volumes := []kvcore.Volume{
		{
			Name: "rootdisk",
			VolumeSource: kvcore.VolumeSource{
				DataVolume: &kvcore.DataVolumeSource{Name: "rootdisk-dv"},
			},
		},
		{
			Name: "datadisk",
			VolumeSource: kvcore.VolumeSource{
				PersistentVolumeClaim: &kvcore.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"},
				},
			},
		},
	}

	names := GetVolumeClaimNames(volumes)
	assert.Contains(t, names, "rootdisk-dv")
	assert.Contains(t, names, "data-pvc")
}

func TestExcludeNativeBackedPVCs(t *testing.T) {
	items := []velero.ResourceIdentifier{
		{GroupResource: schema.GroupResource{Resource: "persistentvolumeclaims"}, Namespace: "ns", Name: "rootdisk-dv"},
		{GroupResource: schema.GroupResource{Resource: "persistentvolumeclaims"}, Namespace: "ns", Name: "unrelated-pvc"},
		{GroupResource: schema.GroupResource{Resource: "virtualmachineinstances"}, Namespace: "ns", Name: "my-vmi"},
	}

	nativeVolumes := []kvcore.Volume{
		{
			Name: "rootdisk",
			VolumeSource: kvcore.VolumeSource{
				DataVolume: &kvcore.DataVolumeSource{Name: "rootdisk-dv"},
			},
		},
	}

	filtered := ExcludeNativeBackedPVCs(items, nativeVolumes)
	assert.Len(t, filtered, 2) // rootdisk-dv excluded, unrelated-pvc and VMI kept
	assert.Equal(t, "unrelated-pvc", filtered[0].Name)
	assert.Equal(t, "my-vmi", filtered[1].Name)
}

func TestIsCheckpointHealthy(t *testing.T) {
	tests := []struct {
		name     string
		tracker  *unstructured.Unstructured
		expected bool
	}{
		{
			name: "healthy checkpoint",
			tracker: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"latestCheckpoint": map[string]interface{}{
							"name": "cp-1",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "checkpoint redefinition required",
			tracker: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"checkpointRedefinitionRequired": true,
						"latestCheckpoint": map[string]interface{}{
							"name": "cp-1",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "no checkpoint",
			tracker: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{},
				},
			},
			expected: false,
		},
		{
			name: "no status",
			tracker: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCheckpointHealthy(tt.tracker)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Test with nil backup
	cfg := LoadConfig(nil)
	assert.False(t, cfg.NativeBackupEnabled)
	assert.False(t, cfg.IncrementalEnabled)
	assert.Equal(t, "", cfg.DefaultScratchStorageClass)

	// Test with backup labels overriding defaults
	backup := &v1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				NativeBackupLabel:   "true",
				IncrementalLabel:    "true",
				ScratchStorageLabel: "fast-ssd",
				ConcurrencyLabel:    "3",
				ForceFullEveryLabel: "5",
			},
		},
	}
	cfg = LoadConfig(backup)
	assert.True(t, cfg.NativeBackupEnabled)
	assert.True(t, cfg.IncrementalEnabled)
	assert.Equal(t, "fast-ssd", cfg.DefaultScratchStorageClass)
	assert.Equal(t, 3, cfg.MaxConcurrentBackups)
	assert.Equal(t, 5, cfg.ForceFullEveryN)
}

func TestGetBackupMetadata(t *testing.T) {
	vmBackup := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"type":           "incremental",
				"checkpointName": "cp-2",
				"includedVolumes": []interface{}{
					map[string]interface{}{
						"volumeName": "rootdisk",
						"diskTarget": "vda",
					},
					map[string]interface{}{
						"volumeName": "datadisk",
						"diskTarget": "vdb",
					},
				},
			},
		},
	}

	backupType, checkpoint, volumes := GetBackupMetadata(vmBackup)
	assert.Equal(t, "incremental", backupType)
	assert.Equal(t, "cp-2", checkpoint)
	assert.Equal(t, "rootdisk,datadisk", volumes)
}
