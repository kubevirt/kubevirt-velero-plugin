package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kvcore "kubevirt.io/client-go/api/v1"
)

// VirtualMachineStatusStopped VirtualMachinePrintableStatus = "Stopped"
// 	VirtualMachineStatusProvisioning VirtualMachinePrintableStatus = "Provisioning"
// 	VirtualMachineStatusStarting VirtualMachinePrintableStatus = "Starting"
// 	VirtualMachineStatusRunning VirtualMachinePrintableStatus = "Running"
// 	VirtualMachineStatusPaused VirtualMachinePrintableStatus = "Paused"
// 	VirtualMachineStatusStopping VirtualMachinePrintableStatus = "Stopping"
// 	VirtualMachineStatusTerminating VirtualMachinePrintableStatus = "Terminating"
// 	VirtualMachineStatusMigrating VirtualMachinePrintableStatus = "Migrating"
// 	VirtualMachineStatusUnknown VirtualMachinePrintableStatus = "Unknown"

func TestCanBeSafelyBackedUp(t *testing.T) {
	testCases := []struct {
		name     string
		vm       kvcore.VirtualMachine
		backup   v1.Backup
		expected bool
	}{
		{"Stopped VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStopped,
				},
			},
			v1.Backup{},
			true,
		},
		{"Provisioning VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusProvisioning,
				},
			},
			v1.Backup{},
			true,
		},
		{"Paused VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusPaused,
				},
			},
			v1.Backup{},
			true,
		},
		{"Stopping VM can be safely backed up", // TODO: Can it really!?
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStopping,
				},
			},
			v1.Backup{},
			true,
		},
		{"Terminating VM can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusTerminating,
				},
			},
			v1.Backup{},
			true,
		},
		{"Migrating VM can be safely backed up", // TODO: Can it really?
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusMigrating,
				},
			},
			v1.Backup{},
			true,
		},
		{"VM with unknown status can be safely backed up",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusUnknown,
				},
			},
			v1.Backup{},
			true,
		},
		{"Running VM can be safely backed up when IncludeResource is empty",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			v1.Backup{},
			true,
		},
		{"Starting VM can be safely backed up when IncludeResource is empty",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStarting,
				},
			},
			v1.Backup{},
			true,
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
			true,
		},
		{"Starting VM can be safely backed up when IncludeResource contains both pods and VMIs",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStarting,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"pods", "virtualmachineinstances"},
				},
			},
			true,
		},
		{"Running VM cannot be safely backed up when IncludeResource do not contains pods",
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
			false,
		},
		{"Running VM cannot be safely backed up when IncludeResource do not contains VMIs",
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
			false,
		},
		{"Starting VM cannot be safely backed up when IncludeResource do not contains pods",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStarting,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			false,
		},
		{"Starting VM cannot be safely backed up when IncludeResource do not contains VMIs",
			kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusStarting,
				},
			},
			v1.Backup{
				Spec: v1.BackupSpec{
					IncludedResources: []string{"pods"},
				},
			},
			false,
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := canBeSafelyBackedUp(&tc.vm, &tc.backup)
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
						"namespace": "test-namespace",
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
		{"Created VM needs to include VMI",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{},
					"status": map[string]interface{}{
						"created": true,
					},
				},
			},
			v1.Backup{},
			false,
			[]velero.ResourceIdentifier{{
				GroupResource: schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"},
				Namespace:     "test-namespace",
				Name:          "test-vm",
			}},
		},
		{"All DVs from DataVolumeTemplates needs to be included",
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": "test-namespace",
					},
					"spec": map[string]interface{}{
						"dataVolumeTemplates": []interface{}{
							map[string]interface{}{
								"apiVersion": "cdi.kubevirt.io/v1beta1",
								"kind":       "DataVolume",
								"metadata": map[string]interface{}{
									"name": "test-dv",
								},
							},
							map[string]interface{}{
								"apiVersion": "cdi.kubevirt.io/v1beta1",
								"kind":       "DataVolume",
								"metadata": map[string]interface{}{
									"name":      "test-dv",
									"namespace": "another-namespace",
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
					Namespace:     "test-namespace",
					Name:          "test-dv",
				},
				{
					GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
					Namespace:     "another-namespace",
					Name:          "test-dv",
				},
			},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMBackupItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, extra, err := action.Execute(&tc.vm, &tc.backup)

			if !tc.errorExpected {
				assert.Nil(t, err)
			}
			for _, item := range tc.expectedExtra {
				assert.Contains(t, extra, item)
			}
		})
	}
}
