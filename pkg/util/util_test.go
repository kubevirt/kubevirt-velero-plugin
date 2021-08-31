package util

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
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
