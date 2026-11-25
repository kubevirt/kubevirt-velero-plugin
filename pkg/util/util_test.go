package util

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

func TestGetLauncherPod(t *testing.T) {
	older := metav1.NewTime(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(time.Date(2026, 1, 1, 15, 30, 0, 0, time.UTC))
	deleting := metav1.NewTime(time.Date(2026, 1, 1, 15, 31, 0, 0, time.UTC))

	launcherPod := func(name, domain, node string, phase v1.PodPhase, created metav1.Time) v1.Pod {
		return v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:         "test-namespace",
				Name:              name,
				UID:               types.UID(name + "-uid"),
				CreationTimestamp: created,
				Labels: map[string]string{
					"kubevirt.io":         "virt-launcher",
					kvcore.CreatedByLabel: "test-vmi-uid",
				},
				Annotations: map[string]string{
					kvcore.DomainAnnotation: domain,
				},
			},
			Spec:   v1.PodSpec{NodeName: node},
			Status: v1.PodStatus{Phase: phase},
		}
	}
	withDeletion := func(pod v1.Pod) v1.Pod {
		pod.DeletionTimestamp = &deleting
		return pod
	}
	withCreatedBy := func(pod v1.Pod, uid string) v1.Pod {
		pod.Labels[kvcore.CreatedByLabel] = uid
		return pod
	}
	vmiOn := func(node string, activePods ...string) *kvcore.VirtualMachineInstance {
		vmi := &kvcore.VirtualMachineInstance{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-vmi",
				UID:       "test-vmi-uid",
			},
			Status: kvcore.VirtualMachineInstanceStatus{
				NodeName:   node,
				ActivePods: map[types.UID]string{},
			},
		}
		for _, pod := range activePods {
			vmi.Status.ActivePods[types.UID(pod+"-uid")] = node
		}
		return vmi
	}

	testCases := []struct {
		name     string
		pods     []v1.Pod
		vmi      *kvcore.VirtualMachineInstance
		expected string
	}{
		{"Should return the only launcher pod",
			[]v1.Pod{launcherPod("launcher-a", "test-vmi", "node-a", v1.PodRunning, older)},
			vmiOn("node-a", "launcher-a"),
			"launcher-a",
		},
		{"Should ignore launcher pods of other VMIs",
			[]v1.Pod{
				launcherPod("other-launcher", "other-vmi", "node-a", v1.PodRunning, older),
				launcherPod("launcher-a", "test-vmi", "node-a", v1.PodRunning, newer),
			},
			vmiOn("node-a", "launcher-a"),
			"launcher-a",
		},
		{"Should return nil when no launcher pod matches",
			[]v1.Pod{launcherPod("other-launcher", "other-vmi", "node-a", v1.PodRunning, older)},
			vmiOn("node-a"),
			"",
		},
		{"Should prefer running pod over succeeded migration source pod",
			[]v1.Pod{
				launcherPod("launcher-source", "test-vmi", "node-a", v1.PodSucceeded, older),
				launcherPod("launcher-target", "test-vmi", "node-b", v1.PodRunning, newer),
			},
			vmiOn("node-b", "launcher-target"),
			"launcher-target",
		},
		{"Should prefer running pod over succeeded pod without VMI",
			[]v1.Pod{
				launcherPod("launcher-source", "test-vmi", "node-a", v1.PodSucceeded, older),
				launcherPod("launcher-target", "test-vmi", "node-b", v1.PodRunning, newer),
			},
			nil,
			"launcher-target",
		},
		{"Should prefer running pod over failed pod",
			[]v1.Pod{
				launcherPod("launcher-failed", "test-vmi", "node-a", v1.PodFailed, older),
				launcherPod("launcher-running", "test-vmi", "node-b", v1.PodRunning, newer),
			},
			vmiOn("node-b", "launcher-running"),
			"launcher-running",
		},
		{"Should prefer pod not being deleted",
			[]v1.Pod{
				withDeletion(launcherPod("launcher-deleting", "test-vmi", "node-a", v1.PodRunning, older)),
				launcherPod("launcher-running", "test-vmi", "node-b", v1.PodRunning, newer),
			},
			nil,
			"launcher-running",
		},
		{"Should prefer migration source pod while migration is in progress",
			[]v1.Pod{
				launcherPod("launcher-target", "test-vmi", "node-b", v1.PodRunning, newer),
				launcherPod("launcher-source", "test-vmi", "node-a", v1.PodRunning, older),
			},
			vmiOn("node-a", "launcher-source", "launcher-target"),
			"launcher-source",
		},
		{"Should prefer migration target pod once VMI runs on the target node",
			[]v1.Pod{
				launcherPod("launcher-source", "test-vmi", "node-a", v1.PodRunning, older),
				launcherPod("launcher-target", "test-vmi", "node-b", v1.PodRunning, newer),
			},
			vmiOn("node-b", "launcher-target"),
			"launcher-target",
		},
		{"Should prefer active pod when VMI node is unknown",
			[]v1.Pod{
				launcherPod("launcher-a", "test-vmi", "node-a", v1.PodRunning, older),
				launcherPod("launcher-b", "test-vmi", "node-b", v1.PodRunning, newer),
			},
			vmiOn("", "launcher-b"),
			"launcher-b",
		},
		{"Should pick oldest live pod when nothing else differs",
			[]v1.Pod{
				launcherPod("launcher-b", "test-vmi", "node-b", v1.PodRunning, newer),
				launcherPod("launcher-a", "test-vmi", "node-a", v1.PodRunning, older),
			},
			nil,
			"launcher-a",
		},
		{"Should pick by name when creation timestamps are equal",
			[]v1.Pod{
				launcherPod("launcher-b", "test-vmi", "node-b", v1.PodRunning, older),
				launcherPod("launcher-a", "test-vmi", "node-a", v1.PodRunning, older),
			},
			nil,
			"launcher-a",
		},
		{"Should skip pods created for a previous VMI",
			[]v1.Pod{
				withCreatedBy(launcherPod("launcher-old-vmi", "test-vmi", "node-a", v1.PodRunning, older), "old-vmi-uid"),
				launcherPod("launcher-a", "test-vmi", "node-a", v1.PodPending, newer),
			},
			vmiOn("node-a", "launcher-a"),
			"launcher-a",
		},
		{"Should fall back to finished pod when no live pod exists",
			[]v1.Pod{launcherPod("launcher-source", "test-vmi", "node-a", v1.PodSucceeded, older)},
			vmiOn("node-a"),
			"launcher-source",
		},
	}

	origListPods, origGetVMI := ListPods, GetVMI
	defer func() { ListPods, GetVMI = origListPods, origGetVMI }()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ListPods = func(name, ns string) (*v1.PodList, error) {
				return &v1.PodList{Items: tc.pods}, nil
			}
			GetVMI = func(ns, name string) (*kvcore.VirtualMachineInstance, error) {
				if tc.vmi == nil {
					return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"}, name)
				}
				return tc.vmi, nil
			}

			pod, err := GetLauncherPod("test-vmi", "test-namespace")
			assert.NoError(t, err)
			if tc.expected == "" {
				assert.Nil(t, pod)
			} else if assert.NotNil(t, pod) {
				assert.Equal(t, tc.expected, pod.Name)
			}

			if tc.vmi != nil {
				pod, err = GetLauncherPodForVMI(tc.vmi)
				assert.NoError(t, err)
				if tc.expected == "" {
					assert.Nil(t, pod)
				} else if assert.NotNil(t, pod) {
					assert.Equal(t, tc.expected, pod.Name)
				}
			}
		})
	}
}

func TestGetLauncherPodListError(t *testing.T) {
	origListPods, origGetVMI := ListPods, GetVMI
	defer func() { ListPods, GetVMI = origListPods, origGetVMI }()

	ListPods = func(name, ns string) (*v1.PodList, error) {
		return nil, fmt.Errorf("list failed")
	}
	GetVMI = func(ns, name string) (*kvcore.VirtualMachineInstance, error) {
		return nil, fmt.Errorf("get failed")
	}

	pod, err := GetLauncherPod("test-vmi", "test-namespace")
	assert.Error(t, err)
	assert.Nil(t, pod)
}
