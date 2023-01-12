package util

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v1 "k8s.io/api/core/v1"
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

func TestAddVMIObjectGraph(t *testing.T) {
	testCases := []struct {
		name     string
		vmi      kvcore.VirtualMachineInstance
		expected []velero.ResourceIdentifier
	}{
		{"Should include all volumes",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								PersistentVolumeClaim: &kvcore.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
										ClaimName: "test-pvc",
									},
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								DataVolume: &kvcore.DataVolumeSource{
									Name: "test-dv",
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								MemoryDump: &kvcore.MemoryDumpVolumeSource{
									PersistentVolumeClaimVolumeSource: kvcore.PersistentVolumeClaimVolumeSource{
										PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
											ClaimName: "test-memoryDump",
										},
									},
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								ConfigMap: &kvcore.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "test-cm",
									},
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								Secret: &kvcore.SecretVolumeSource{
									SecretName: "test-secret",
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								ServiceAccount: &kvcore.ServiceAccountVolumeSource{
									ServiceAccountName: "test-sa",
								},
							},
						},
					},
				},
			},
			[]velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     "test-namespace",
					Name:          "test-pvc",
				},
				{
					GroupResource: schema.GroupResource{
						Group:    "cdi.kubevirt.io",
						Resource: "datavolumes",
					},
					Namespace: "test-namespace",
					Name:      "test-dv",
				},
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     "test-namespace",
					Name:          "test-memoryDump",
				},
				{
					GroupResource: schema.GroupResource{
						Group:    "",
						Resource: "configmaps",
					},
					Namespace: "test-namespace",
					Name:      "test-cm",
				},
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     "test-namespace",
					Name:          "test-secret",
				},
				{
					GroupResource: kuberesource.ServiceAccounts,
					Namespace:     "test-namespace",
					Name:          "test-sa",
				},
			},
		},
		{"Should include all access credentials",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: kvcore.VirtualMachineInstanceSpec{
					AccessCredentials: []kvcore.AccessCredential{
						{
							SSHPublicKey: &kvcore.SSHPublicKeyAccessCredential{
								Source: kvcore.SSHPublicKeyAccessCredentialSource{
									Secret: &kvcore.AccessCredentialSecretSource{
										SecretName: "test-ssh-public-key",
									},
								},
							},
						},
						{
							UserPassword: &kvcore.UserPasswordAccessCredential{
								Source: kvcore.UserPasswordAccessCredentialSource{
									Secret: &kvcore.AccessCredentialSecretSource{
										SecretName: "test-user-password",
									},
								},
							},
						},
					},
				},
			},
			[]velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     "test-namespace",
					Name:          "test-ssh-public-key",
				},
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     "test-namespace",
					Name:          "test-user-password",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := AddVMIObjectGraph(tc.vmi.Spec, tc.vmi.Namespace, []velero.ResourceIdentifier{}, logrus.StandardLogger())

			assert.Equal(t, tc.expected, output)
		})
	}
}
