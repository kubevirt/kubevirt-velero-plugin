package util

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kvcore "kubevirt.io/api/core/v1"
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
		{"Full resource name should succeed",
			"virtualmachines",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachines.kubevirt.io"},
				},
			},
			true,
		},
		{"Singular/plural full resource name should succeed",
			"virtualmachines",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachine.kubevirt.io"},
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
		{"Full resource name in excluded resources should return true",
			"virtualmachines",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"virtualmachines.kubevirt.io"},
				},
			},
			true,
		},
		{"Singular/plural full resource name in excluded resources should return true",
			"virtualmachines",
			&velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"virtualmachine.kubevirt.io"},
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
	returnDVNotFound := func(something ...interface{}) (bool, error) {
		return false, k8serrors.NewNotFound(schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, "dv")
	}
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
				PersistentVolumeClaim: &kvcore.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{}},
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
		{"Returns true if volumes have DV volumes, DVs doesnt exist, but PVCs included in backup",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"persistentvolumeclaims"},
				},
			},
			skipFalse,
			returnDVNotFound,
			returnFalse,
			true,
		},
		{"Returns false if volumes have DV volumes, DV doesnt exist and PVCs excluded in backup",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					ExcludedResources: []string{"persistentvolumeclaims"},
				},
			},
			skipFalse,
			returnDVNotFound,
			returnFalse,
			false,
		},
		{"Returns false if volumes have DV volumes, DV doesnt exist and PVCs not inclueded in backup",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pods"},
				},
			},
			skipFalse,
			returnDVNotFound,
			returnFalse,
			false,
		},
		{"Returns false if volumes have DV volumes, DV doesnt exist, PVCs included in backup but excluded by label",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pods", "persistentvolumeclaims"},
				},
			},
			skipFalse,
			returnDVNotFound,
			returnTrue,
			false,
		},
		{"Returns false if volumes have DV volumes and DVs not inclued in backup",
			dvVolumes,
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"pods", "persistentvolumeclaims"},
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

func TestIsMacAddressCleared(t *testing.T) {
	testCases := []struct {
		name     string
		resource string
		restore  velerov1.Restore
		expected bool
	}{
		{"Clear MAC address should return false with no label",
			"Restore",
			velerov1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			false,
		},
		{"Clear MAC address should return true with ClearMacAddressLabel label",
			"Restore",
			velerov1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						ClearMacAddressLabel: "",
					},
				},
			},
			true,
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ShouldClearMacAddress(&tc.restore)

			assert.Equal(t, tc.expected, result)
		})
	}
}
