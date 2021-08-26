package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kvcore "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
)

func TestVMIBackupItemAction(t *testing.T) {
	notPaused := func(vmi *kvcore.VirtualMachineInstance) (bool, error) { return false, nil }
	nullValidator := func(runtime.Unstructured, []velero.ResourceIdentifier) bool { return true }

	ownedVMI := map[string]interface{}{
		"apiVersion": "kubevirt.io",
		"kind":       "VirtualMachineInterface",
		"metadata": map[string]interface{}{
			"name":      "test-vmi",
			"namespace": "test-namespace",
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"name": "test-owner",
				},
			},
		},
	}
	nonOwnedVMI := map[string]interface{}{
		"apiVersion": "kubevirt.io",
		"kind":       "VirtualMachineInterface",
		"metadata": map[string]interface{}{
			"name":      "test-vmi",
			"namespace": "test-namespace",
		},
	}

	testCases := []struct {
		name        string
		item        unstructured.Unstructured
		backup      velerov1.Backup
		pod         v1.Pod
		isVmPaused  func(vmi *kvcore.VirtualMachineInstance) (bool, error)
		expectError bool
		validator   func(runtime.Unstructured, []velero.ResourceIdentifier) bool
	}{
		{"Owned VMI must include VM in backup",
			unstructured.Unstructured{
				Object: ownedVMI,
			},
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			v1.Pod{},
			notPaused,
			true,
			nullValidator,
		},
		{"Owned and not paused VMI must include VM and Pod in backup",
			unstructured.Unstructured{
				Object: ownedVMI,
			},
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances,virtualmachines"},
				},
			},
			v1.Pod{},
			notPaused,
			true,
			nullValidator,
		},
		{"Owned VMI must add 'is owned' annotation",
			unstructured.Unstructured{
				Object: ownedVMI,
			},
			velerov1.Backup{},
			v1.Pod{},
			notPaused,
			false,
			func(item runtime.Unstructured, extra []velero.ResourceIdentifier) bool {
				metadata, err := meta.Accessor(item)
				assert.NoError(t, err)

				return assert.Equal(t, map[string]string{"cdi.kubevirt.io/velero.isOwned": "true"}, metadata.GetAnnotations())
			},
		},
		{"Not owned VMI must include Pod in backup",
			unstructured.Unstructured{
				Object: nonOwnedVMI,
			},
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			v1.Pod{},
			notPaused,
			true,
			nullValidator,
		},
		{"Not owned VMI with DV volumes must include DataVolumes in backup",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachineInterface",
					"metadata": map[string]interface{}{
						"name":      "test-vmi",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"volumes": []interface{}{
							map[string]interface{}{
								"dataVolume": map[string]interface{}{},
							},
						},
					},
				},
			},
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			v1.Pod{},
			notPaused,
			true,
			nullValidator,
		},
		{"Not owned VMI with PVC volumes must include PVCs in backup",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachineInterface",
					"metadata": map[string]interface{}{
						"name":      "test-vmi",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"volumes": []interface{}{
							map[string]interface{}{
								"persistentVolumeClaim": map[string]interface{}{},
							},
						},
					},
				},
			},
			velerov1.Backup{
				Spec: velerov1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			v1.Pod{},
			notPaused,
			true,
			nullValidator,
		},
		{"Launcher pod included in extra resources",
			unstructured.Unstructured{
				Object: nonOwnedVMI,
			},
			velerov1.Backup{},
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi-launcher-pod",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"kubevirt.io/domain": "test-vmi",
					},
				},
			},
			notPaused,
			false,
			func(_ runtime.Unstructured, extra []velero.ResourceIdentifier) bool {
				podResource := velero.ResourceIdentifier{
					GroupResource: kuberesource.Pods,
					Namespace:     "test-namespace",
					Name:          "test-vmi-launcher-pod",
				}
				return assert.Contains(t, extra, podResource)
			},
		},
		{"Volumes included in extra resources",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachineInterface",
					"metadata": map[string]interface{}{
						"name":      "test-vmi",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"volumes": []interface{}{
							map[string]interface{}{
								"persistentVolumeClaim": map[string]interface{}{
									"claimName": "test-pvc",
								},
							},
							map[string]interface{}{
								"dataVolume": map[string]interface{}{
									"name": "test-dv",
								},
							},
						},
					},
				},
			},
			velerov1.Backup{},
			v1.Pod{},
			notPaused,
			false,
			func(_ runtime.Unstructured, extra []velero.ResourceIdentifier) bool {
				pvcResource := velero.ResourceIdentifier{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     "test-namespace",
					Name:          "test-pvc",
				}
				dvResource := velero.ResourceIdentifier{
					GroupResource: schema.GroupResource{
						Group:    "cdi.kubevirt.io",
						Resource: "datavolumes",
					},
					Namespace: "test-namespace",
					Name:      "test-dv",
				}

				assert.Contains(t, extra, pvcResource)
				assert.Contains(t, extra, dvResource)
				return true
			},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	for _, tc := range testCases {
		kubeobjects := []runtime.Object{}
		kubeobjects = append(kubeobjects, &tc.pod)
		client := k8sfake.NewSimpleClientset(kubeobjects...)
		action := NewVMIBackupItemAction(logrus.StandardLogger(), client)
		isVMPaused = tc.isVmPaused

		t.Run(tc.name, func(t *testing.T) {
			output, extra, err := action.Execute(&tc.item, &tc.backup)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tc.validator(output, extra)
			}
		})
	}
}

func TestAddLauncherPod(t *testing.T) {
	testCases := []struct {
		name     string
		vmi      kvcore.VirtualMachineInstance
		pods     v1.Pod
		expected []velero.ResourceIdentifier
	}{
		{"Should include launcher pod if present",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi",
				},
			},
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi-launcher-pod",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"kubevirt.io/domain": "test-vmi",
					},
				},
			},
			[]velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.Pods,
					Namespace:     "test-namespace",
					Name:          "test-vmi-launcher-pod",
				},
			},
		},
		{"Should include only own launcher pod",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi",
				},
			},
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi-launcher-pod",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"kubevirt.io/domain": "another-vmi",
					},
				},
			},
			[]velero.ResourceIdentifier{},
		},
		{"Should not include other pods",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi",
				},
			},
			v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi-launcher-pod",
				},
			},
			[]velero.ResourceIdentifier{},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	kubecli.GetKubevirtClientFromClientConfig = kubecli.GetMockKubevirtClientFromClientConfig
	for _, tc := range testCases {
		kubeobjects := []runtime.Object{}
		kubeobjects = append(kubeobjects, &tc.pods)
		client := k8sfake.NewSimpleClientset(kubeobjects...)
		action := NewVMIBackupItemAction(logrus.StandardLogger(), client)

		t.Run(tc.name, func(t *testing.T) {
			output, err := action.addLauncherPod(&tc.vmi, []velero.ResourceIdentifier{})

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, output)
		})
	}
}

func TestAddVolumes(t *testing.T) {
	testCases := []struct {
		name     string
		vmi      kvcore.VirtualMachineInstance
		expected []velero.ResourceIdentifier
	}{
		{"Should include all DV and PVC volumes",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: "test-pvc",
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
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := addVolumes(&tc.vmi, []velero.ResourceIdentifier{})

			assert.Equal(t, tc.expected, output)
		})
	}
}

func TestHasDVVolumes(t *testing.T) {
	testCases := []struct {
		name     string
		vmi      kvcore.VirtualMachineInstance
		expected bool
	}{
		{"Should return false if VMI has no DV Volume",
			kvcore.VirtualMachineInstance{
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{},
							},
						},
					},
				},
			},
			false,
		},
		{"Should return false if VMI has any DV Volume",
			kvcore.VirtualMachineInstance{
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								DataVolume: &kvcore.DataVolumeSource{},
							},
						},
					},
				},
			},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := hasDVVolumes(&tc.vmi)

			assert.Equal(t, tc.expected, output)
		})
	}
}

func TestHasPVCVolumes(t *testing.T) {
	testCases := []struct {
		name     string
		vmi      kvcore.VirtualMachineInstance
		expected bool
	}{
		{"Should return true if VMI has any PVC Volume",
			kvcore.VirtualMachineInstance{
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{},
							},
						},
					},
				},
			},
			true,
		},
		{"Should return false if VMI has no PVC Volumes",
			kvcore.VirtualMachineInstance{
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								DataVolume: &kvcore.DataVolumeSource{},
							},
						},
					},
				},
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := hasPVCVolumes(&tc.vmi)

			assert.Equal(t, tc.expected, output)
		})
	}
}
