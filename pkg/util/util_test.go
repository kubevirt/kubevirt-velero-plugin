package util

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	kvcore "kubevirt.io/client-go/api/v1"
)

func TestIsResourceIncluded(t *testing.T) {
	testCases := []struct {
		name     string
		resource string
		backup   *velerov1.Backup
		expected bool
	}{
		{"Empty include resources should succeed",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{},
				},
			},
			true,
		},
		{"Resource in incuded resources should succeed",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pods", "virtualmachines", "persistentvolumes"},
				},
			},
			true,
		},
		{"Resource not in included resources should fail",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachines", "persistentvolumes"},
				},
			},
			false,
		},
		{"Capitalization should not matter (resource)",
			"DataVolumes",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"datavolumes"},
				},
			},
			true,
		},
		{"Capitalization should not matter (backup)",
			"datavolumes",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"DataVolumes"},
				},
			},
			true,
		},
		{"Singular/plural should not matter (resource)",
			"pod",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pods"},
				},
			},
			true,
		},
		{"Singular/plural should not matter (backup)",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pod"},
				},
			},
			true,
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsResourceIncluded(tc.resource, tc.backup)

			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsResourceExcluded(t *testing.T) {
	testCases := []struct {
		name     string
		resource string
		backup   *velerov1.Backup
		expected bool
	}{
		{"Empty exclude resources should return false",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{},
				},
			},
			false,
		},
		{"Resource in excuded resources should return true",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"pods", "virtualmachines", "persistentvolumes"},
				},
			},
			true,
		},
		{"Resource not in excluded resources should fail",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"virtualmachines", "persistentvolumes"},
				},
			},
			false,
		},
		{"Capitalization should not matter (resource)",
			"DataVolumes",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"datavolumes"},
				},
			},
			true,
		},
		{"Capitalization should not matter (backup)",
			"datavolumes",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"DataVolumes"},
				},
			},
			true,
		},
		{"Singular/plural should not matter (resource)",
			"pod",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"pods"},
				},
			},
			true,
		},
		{"Singular/plural should not matter (backup)",
			"pods",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"pod"},
				},
			},
			true,
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsResourceExcluded(tc.resource, tc.backup)

			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRestorePossible(t *testing.T) {
	returnFalse := func(something ...interface{}) (bool, error) { return false, nil }
	returnTrue := func(something ...interface{}) (bool, error) { return true, nil }
	skipFalse := func(volume kvcore.Volume) bool { return false }
	skipTrue := func(volume kvcore.Volume) bool { return true }

	dvVolumes := []kvcore.Volume{
		{
			VolumeSource: kvcore.VolumeSource{
				DataVolume: &kvcore.DataVolumeSource{},
			},
		},
	}
	pvcVolumes := []kvcore.Volume{
		{
			VolumeSource: kvcore.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{},
			},
		},
	}

	testCases := []struct {
		name          string
		volumes       []kvcore.Volume
		backup        velerov1.Backup
		extraTest     func(volume kvcore.Volume) bool
		isDvExcluded  func(something ...interface{}) (bool, error)
		isPvcExcluded func(something ...interface{}) (bool, error)
		expected      bool
	}{
		{"Returns true if volumes have no volumes",
			dvVolumes,
			velerov1.Backup{},
			skipFalse,
			returnFalse,
			returnFalse,
			true,
		},
		{"Returns true if volumes have DV volumes and DVs included in backup",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"datavolumes"},
				},
			},
			skipFalse,
			returnFalse,
			returnFalse,
			true,
		},
		{"Returns true if volumes have DV volumes, backup excludes datavolumes but skipVolume returns true",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"datavolumes"},
				},
			},
			skipTrue,
			returnFalse,
			returnFalse,
			true,
		},
		{"Returns false if volumes have DV volumes and DVs not included in backup",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pods"},
				},
			},
			skipFalse,
			returnFalse,
			returnFalse,
			false,
		},
		{"Returns false if volumes have DV volumes and DVs excluded in backup",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"datavolumes"},
				},
			},
			skipFalse,
			returnFalse,
			returnFalse,
			false,
		},
		{"Returns false if volumes have DV volumes and DV excluded by label",
			dvVolumes,
			velerov1.Backup{},
			skipFalse,
			returnTrue,
			returnFalse,
			false},
		{"Returns true if volumes have PVC volumes and PVCs included in backup",
			pvcVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"persistentvolumeclaims"},
				},
			},
			skipFalse,
			returnFalse,
			returnFalse,
			true,
		},
		{"Returns false if volumes have PVC volumes and PVCs not included in backup",
			pvcVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pods"},
				},
			},
			skipFalse,
			returnFalse,
			returnFalse,
			false,
		},
		{"Returns false if volumes have PVC volumes and PVCs excluded in backup",
			pvcVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"persistentvolumeclaims"},
				},
			},
			skipFalse,
			returnFalse,
			returnFalse,
			false,
		},
		{"Returns false if volumes have PVC volumes and PVC excluded by label",
			pvcVolumes,
			velerov1.Backup{},
			skipFalse,
			returnFalse,
			returnTrue,
			false,
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	for _, tc := range testCases {
		IsDVExcludedByLabel = func(namespace, pvcName string) (bool, error) { return tc.isDvExcluded(namespace, pvcName) }
		IsPVCExcludedByLabel = func(namespace, pvcName string) (bool, error) { return tc.isPvcExcluded(namespace, pvcName) }

		t.Run(tc.name, func(t *testing.T) {
			possible, err := RestorePossible(tc.volumes, &tc.backup, "", tc.extraTest, &logrus.Logger{})

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, possible)
		})
	}
}
