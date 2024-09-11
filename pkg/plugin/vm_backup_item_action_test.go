package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

const (
	testNamespace = "test-namespace"
)

func returnTrue(vm *kvcore.VirtualMachine) (bool, error)  { return true, nil }
func returnFalse(vm *kvcore.VirtualMachine) (bool, error) { return false, nil }

func TestCanBeSafelyBackedUp(t *testing.T) {
	testCases := []struct {
		name                 string
		vm                   kvcore.VirtualMachine
		backup               v1.Backup
		isVMIExcludedByLabel func(vm *kvcore.VirtualMachine) (bool, error)
		expected             bool
	}{
		{"Stopped VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStopped,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Provisioning VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusProvisioning,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Paused VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusPaused,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Stopping VM can be safely backed up", // TODO: Can it really!?
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStopping,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Terminating VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusTerminating,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Migrating VM can be safely backed up", // TODO: Can it really?
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusMigrating,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"VM with unknown status can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusUnknown,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Running VM can be safely backed up when IncludeResources and ExcludedResources is empty and VMI not excluded by label",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Starting VM can be safely backed up when IncludeResources and ExcludedResrouces is empty and VMI not excluded by label",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStarting,
				},
			},
			v1.Backup{},
			returnFalse,
			true,
		},
		{"Running VM can be safely backed up when IncludeResources and ExcludedResources is empty and VMI is excluded by label",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{},
			returnTrue,
			false,
		},
		{"Running VM can be safely backed up when IncludeResource contains both pods and VMIs",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"pods", "virtualmachineinstances"},
				},
			},
			returnFalse,
			true,
		},
		{"Running VM can be safely backed up when ExcludeResource contains both pods and PVCs",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					ExcludedResources: []string{"pods", "persistentvolumeclaims"},
				},
			},
			returnFalse,
			true,
		},
		{"Running VM cannot be safely backed up when IncludeResource do not contain pods",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances", "persistentvolumeclaims"},
				},
			},
			returnFalse,
			false,
		},
		{"Running VM can be safely backed up when IncludeResource do not contain pods or PVCs",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			returnFalse,
			true,
		},
		{"Running VM cannot be safely backed up when IncludeResource do not contain VMIs",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"pods"},
				},
			},
			returnFalse,
			false,
		},
		{"Running VM cannot be safely backed up when ExcludeResource contains pods",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					ExcludedResources: []string{"pods"},
				},
			},
			returnFalse,
			false,
		},
		{"Running VM cannot be safely backed up when ExcludeResource contains VMIs",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					ExcludedResources: []string{"virtualmachineinstances"},
				},
			},
			returnFalse,
			false,
		},
		{"Running VM cannot be safely backed up when VMI is excluded by label",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{},
			returnTrue,
			false,
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMBackupItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		isVMIExcludedByLabel = tc.isVMIExcludedByLabel
		t.Run(tc.name, func(t *testing.T) {
			actual, err := action.canBeSafelyBackedUp(&tc.vm, &tc.backup)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestVMBackupAction(t *testing.T) {
	testCases := []struct {
		name          string
		vm            unstructured.Unstructured
		backup        v1.Backup
		errorExpected bool
		expectedExtra []velero.ResourceIdentifier
	}{
		{"Action should return err for a VM that cannot be safely backed up",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": testNamespace,
					},
					"spec": map[string]interface{}{},
					"status": map[string]interface{}{
						"created":         true,
						"printableStatus": "Running",
					},
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"pods"},
				},
			},
			true,
			[]velero.ResourceIdentifier{},
		},
		{"Action should return err for a VM would not be safely restored",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": testNamespace,
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"volumes": []map[string]interface{}{
									{
										"volumeSource": map[string]interface{}{
											"dataVolume": map[string]interface{}{},
										},
									},
								},
							},
						},
					},
					"status": map[string]interface{}{
						"created":         true,
						"printableStatus": "Running",
					},
				},
			},
			v1.Backup{Spec: v1.BackupSpec{
				IncludedResources: []string{"datavolume"},
			}},
			true,
			[]velero.ResourceIdentifier{},
		},
		{"Action should succeed when VM with excluded volumes has metadataBackup label",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": testNamespace,
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"volumes": []map[string]interface{}{
									map[string]interface{}{
										"name": "vol",
										"dataVolume": map[string]interface{}{
											"name": "test-dv",
										},
									},
								},
							},
						},
					},
					"status": map[string]interface{}{
						"created":         true,
						"printableStatus": "Running",
					},
				},
			},
			v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"velero.kubevirt.io/metadataBackup": "true",
					},
				},
				Spec: v1.BackupSpec{
					ExcludedResources: []string{"datavolumes", "persistentvolumeclaims"},
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			false,
			[]velero.ResourceIdentifier{
				{
					GroupResource: schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"},
					Namespace:     testNamespace,
					Name:          "test-vm",
				},
				// Our plugin still returns the PVC and DV even though they are excluded.
				// Velero will filter them during backup.
				{
					GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
					Namespace:     testNamespace,
					Name:          "test-dv",
				},
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     testNamespace,
					Name:          "test-dv",
				},
			},
		},
		{"Created VM needs to include VMI",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": testNamespace,
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"volumes": []map[string]interface{}{},
							},
						},
					},
					"status": map[string]interface{}{
						"created": true,
					},
				},
			},
			v1.Backup{},
			false,
			[]velero.ResourceIdentifier{{
				GroupResource: schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"},
				Namespace:     testNamespace,
				Name:          "test-vm",
			}},
		},
		{"All volumes needs to be included",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": testNamespace,
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"volumes": []interface{}{
									map[string]interface{}{
										"name": "vol0",
										"dataVolume": map[string]interface{}{
											"name": "test-dv",
										},
									},
									map[string]interface{}{
										"name": "vol1",
										"persistentVolumeClaim": map[string]interface{}{
											"claimName": "test-pvc",
										},
									},
									map[string]interface{}{
										"name": "vol2",
										"memoryDump": map[string]interface{}{
											"claimName": "test-memory-dump",
										},
									},
									map[string]interface{}{
										"name": "vol3",
										"configMap": map[string]interface{}{
											"name": "test-config-map",
										},
									},
									map[string]interface{}{
										"name": "vol4",
										"secret": map[string]interface{}{
											"secretName": "test-secret",
										},
									},
									map[string]interface{}{
										"name": "vol5",
										"serviceAccount": map[string]interface{}{
											"serviceAccountName": "test-service-account",
										},
									},
								},
							},
						},
					},
				},
			},
			v1.Backup{},
			false,
			[]velero.ResourceIdentifier{
				{
					GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
					Namespace:     testNamespace,
					Name:          "test-dv",
				},
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     testNamespace,
					Name:          "test-dv",
				},
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     testNamespace,
					Name:          "test-pvc",
				},
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     testNamespace,
					Name:          "test-memory-dump",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "configmaps"},
					Namespace:     testNamespace,
					Name:          "test-config-map",
				},
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     testNamespace,
					Name:          "test-secret",
				},
				{
					GroupResource: kuberesource.ServiceAccounts,
					Namespace:     testNamespace,
					Name:          "test-service-account",
				},
			},
		},
		{"Created VM needs to include instancetype and preference",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": testNamespace,
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"volumes": []map[string]interface{}{},
							},
						},
						"instancetype": map[string]interface{}{
							"kind":         "virtualmachineinstancetype",
							"name":         "test-instancetype",
							"revisionName": "test-revision1",
						},
						"preference": map[string]interface{}{
							"kind":         "virtualmachinepreference",
							"name":         "test-preference",
							"revisionName": "test-revision2",
						},
					},
				},
			},
			v1.Backup{},
			false,
			[]velero.ResourceIdentifier{
				{
					GroupResource: schema.GroupResource{Group: "instancetype.kubevirt.io", Resource: "virtualmachineinstancetype"},
					Namespace:     testNamespace,
					Name:          "test-instancetype",
				},
				{
					GroupResource: schema.GroupResource{Group: "apps", Resource: "controllerrevisions"},
					Namespace:     testNamespace,
					Name:          "test-revision1",
				},
				{
					GroupResource: schema.GroupResource{Group: "instancetype.kubevirt.io", Resource: "virtualmachinepreference"},
					Namespace:     testNamespace,
					Name:          "test-preference",
				},
				{
					GroupResource: schema.GroupResource{Group: "apps", Resource: "controllerrevisions"},
					Namespace:     testNamespace,
					Name:          "test-revision2",
				},
			},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMBackupItemAction(logrus.StandardLogger())
	isVMIExcludedByLabel = returnFalse
	for _, tc := range testCases {
		util.IsDVExcludedByLabel = func(namespace, pvcName string) (bool, error) { return false, nil }
		util.IsPVCExcludedByLabel = func(namespace, pvcName string) (bool, error) { return false, nil }
		t.Run(tc.name, func(t *testing.T) {
			_, extra, err := action.Execute(&tc.vm, &tc.backup)

			if tc.errorExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			// make sure the resulted extra and the expected 1 are equal
			for _, item := range tc.expectedExtra {
				assert.Contains(t, extra, item)
			}
			for _, item := range extra {
				assert.Contains(t, tc.expectedExtra, item)
			}
		})
	}
}

func TestRestorePossible_VM(t *testing.T) {

}
