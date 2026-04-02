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
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

const (
	// Labels on Velero Backup object to control behavior
	NativeBackupLabel   = "velero.kubevirt.io/native-backup"
	IncrementalLabel    = "velero.kubevirt.io/incremental-backup"
	SkipQuiesceLabel    = "velero.kubevirt.io/skip-quiesce"
	ScratchStorageLabel = "velero.kubevirt.io/scratch-storage-class"
	ConcurrencyLabel    = "velero.kubevirt.io/native-backup-concurrency"
	ForceFullEveryLabel = "velero.kubevirt.io/force-full-every-n"

	// Annotations added to VM during backup
	BackupUsedAnnotation       = "velero.kubevirt.io/native-backup-used"
	BackupCRAnnotation         = "velero.kubevirt.io/native-backup-cr"
	BackupTypeAnnotation       = "velero.kubevirt.io/native-backup-type"
	BackupCheckpointAnnotation = "velero.kubevirt.io/native-backup-checkpoint"
	BackupVolumesAnnotation    = "velero.kubevirt.io/native-backup-volumes"
	TrackerAnnotation          = "velero.kubevirt.io/backup-tracker"

	// Labels/annotations on scratch PVCs
	NativeBackupPVCLabel     = "velero.kubevirt.io/native-backup-pvc"
	ScratchPVCBackupLabel    = "velero.kubevirt.io/scratch-for-backup"
	OriginalVolumeAnnotation = "velero.kubevirt.io/original-volume-name"
	OriginalPVCAnnotation    = "velero.kubevirt.io/original-pvc-name"
	ScratchPVCTTLAnnotation  = "velero.kubevirt.io/scratch-pvc-ttl"

	// Labels on source PVCs during native backup to prevent CSI double-snapshot
	NativeBackedPVCLabel = "velero.kubevirt.io/native-backed"

	// Annotation on tracker to count incremental backups since last full
	IncrementalCountAnnotation = "velero.kubevirt.io/incremental-count"

	// ConfigMap defaults
	ConfigMapName      = "kubevirt-velero-plugin-config"
	ConfigMapNamespace = "velero"
)

// Config holds plugin configuration from ConfigMap + label overrides
type Config struct {
	NativeBackupEnabled        bool
	IncrementalEnabled         bool
	DefaultScratchStorageClass string
	MaxConcurrentBackups       int
	ForceFullEveryN            int
	BackupTimeout              time.Duration
	AutoSkipQuiesceNoAgent     bool
}

// DefaultConfig returns configuration defaults
func DefaultConfig() Config {
	return Config{
		NativeBackupEnabled:        false,
		IncrementalEnabled:         false,
		DefaultScratchStorageClass: "",
		MaxConcurrentBackups:       0,
		ForceFullEveryN:            0,
		BackupTimeout:              30 * time.Minute,
		AutoSkipQuiesceNoAgent:     true,
	}
}

// LoadConfig reads configuration from the ConfigMap and overrides with Backup labels
func LoadConfig(backup *v1.Backup) Config {
	cfg := DefaultConfig()

	// Read from ConfigMap (best-effort)
	cfg = loadConfigMap(cfg)

	// Override with Backup labels
	if backup != nil {
		labels := backup.GetLabels()
		if labels != nil {
			if _, ok := labels[NativeBackupLabel]; ok {
				cfg.NativeBackupEnabled = true
			}
			if _, ok := labels[IncrementalLabel]; ok {
				cfg.IncrementalEnabled = true
			}
			if sc, ok := labels[ScratchStorageLabel]; ok && sc != "" {
				cfg.DefaultScratchStorageClass = sc
			}
			if v, ok := labels[ConcurrencyLabel]; ok {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.MaxConcurrentBackups = n
				}
			}
			if v, ok := labels[ForceFullEveryLabel]; ok {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					cfg.ForceFullEveryN = n
				}
			}
		}
	}

	return cfg
}

// IsEnabled returns true if native backup is enabled for this backup
func IsEnabled(backup *v1.Backup) bool {
	if backup == nil {
		return false
	}
	return metav1.HasLabel(backup.ObjectMeta, NativeBackupLabel)
}

// loadConfigMap reads defaults from the plugin ConfigMap (best-effort)
func loadConfigMap(cfg Config) Config {
	client, err := util.GetK8sClient()
	if err != nil {
		return cfg
	}

	cm, err := client.CoreV1().ConfigMaps(ConfigMapNamespace).Get(
		context.TODO(), ConfigMapName, metav1.GetOptions{},
	)
	if err != nil {
		return cfg
	}

	if v, ok := cm.Data["nativeBackupEnabled"]; ok && v == "true" {
		cfg.NativeBackupEnabled = true
	}
	if v, ok := cm.Data["incrementalBackupEnabled"]; ok && v == "true" {
		cfg.IncrementalEnabled = true
	}
	if v, ok := cm.Data["defaultScratchStorageClass"]; ok && v != "" {
		cfg.DefaultScratchStorageClass = v
	}
	if v, ok := cm.Data["maxConcurrentBackups"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxConcurrentBackups = n
		}
	}
	if v, ok := cm.Data["forceFullEveryN"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ForceFullEveryN = n
		}
	}
	if v, ok := cm.Data["backupTimeout"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.BackupTimeout = d
		}
	}
	if v, ok := cm.Data["autoSkipQuiesceWithoutAgent"]; ok {
		cfg.AutoSkipQuiesceNoAgent = v == "true"
	}

	return cfg
}
