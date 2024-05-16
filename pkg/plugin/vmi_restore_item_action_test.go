package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kvcore "kubevirt.io/api/core/v1"
)

func TestVmiRestoreExecute(t *testing.T) {
	testCases := []struct {
		name            string
		input           velero.RestoreItemActionExecuteInput
		skip            bool
		labels          map[string]string
		checkMac        bool
		targetNamespace string
		mac             string
	}{
		{"Owned VMI should be skipped",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "kubevirt.io",
						"kind":       "VirtualMachineInstance",
						"metadata": map[string]interface{}{
							"name":      "test-vmi",
							"namespace": "test-namespace",
							"annotations": map[string]string{
								"cdi.kubevirt.io/velero.isOwned": "true",
							},
						},
					},
				},
				Restore: &velerov1.Restore{},
			},
			true,
			map[string]string{},
			false,
			"",
			"",
		},
		{"Standalone VMI should not be skipped",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "kubevirt.io",
						"kind":       "VirtualMachineInstance",
						"metadata": map[string]interface{}{
							"name":      "test-vmi",
							"namespace": "test-namespace",
						},
					},
				},
				Restore: &velerov1.Restore{},
			},
			false,
			nil,
			false,
			"",
			"",
		},
		{"Restricted labels should be removed",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "kubevirt.io",
						"kind":       "VirtualMachineInstance",
						"metadata": map[string]interface{}{
							"name":      "test-vmi",
							"namespace": "test-namespace",
							"labels": map[string]string{
								"kubevirt.io/created-by":              "someone",
								"kubevirt.io/migrationJobUID":         "abc",
								"kubevirt.io/nodeName":                "test-node",
								"kubevirt.io/migrationTargetNodeName": "test-label",
								"kubevirt.io/schedulable":             "true",
								"kubevirt.io/install-strategy":        "test-strategy",
								"some.other/label":                    "test-value",
							},
						},
					},
				},
				Restore: &velerov1.Restore{},
			},
			false,
			map[string]string{
				"some.other/label": "test-value",
			},
			false,
			"",
			"",
		},
		{"Standalone VMI should not be skipped and mac should not be cleared",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "kubevirt.io",
						"kind":       "VirtualMachineInstance",
						"metadata": map[string]interface{}{
							"name":      "test-vmi",
							"namespace": "test-namespace",
						},
						"spec": map[string]interface{}{
							"domain": map[string]interface{}{
								"devices": map[string]interface{}{
									"interfaces": []map[string]interface{}{
										{
											"name":       "default",
											"macAddress": "00:00:00:00:00:00",
										},
									},
								},
							},
						},
					},
				},
				Restore: &velerov1.Restore{},
			},
			false,
			nil,
			true,
			"",
			"00:00:00:00:00:00",
		},
		{"Standalone VMI should not be skipped and mac should be cleared",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "kubevirt.io",
						"kind":       "VirtualMachineInstance",
						"metadata": map[string]interface{}{
							"name":      "test-vmi",
							"namespace": "test-namespace",
						},
						"spec": map[string]interface{}{
							"domain": map[string]interface{}{
								"devices": map[string]interface{}{
									"interfaces": []map[string]interface{}{
										{
											"name":       "default",
											"macAddress": "00:00:00:00:00:00",
										},
									},
								},
							},
						},
					},
				},
				Restore: &velerov1.Restore{},
			},
			false,
			nil,
			true,
			"new-namespace",
			"",
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMIRestoreItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.targetNamespace != "" {
				tc.input.Restore.Spec.NamespaceMapping = map[string]string{
					"test-namespace": tc.targetNamespace,
				}
			}
			output, err := action.Execute(&tc.input)

			assert.NoError(t, err)
			assert.Equal(t, tc.skip, output.SkipRestore)
			if !tc.skip {
				metadata, err := meta.Accessor(output.UpdatedItem)
				assert.NoError(t, err)
				assert.Equal(t, tc.labels, metadata.GetLabels())
			}
			if tc.checkMac {
				vmi := new(kvcore.VirtualMachineInstance)
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), vmi)
				assert.Nil(t, err)
				assert.Equal(t, tc.mac, vmi.Spec.Domain.Devices.Interfaces[0].MacAddress)
			}
		})
	}
}
