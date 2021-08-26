package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestVmiRestoreExecute(t *testing.T) {
	testCases := []struct {
		name   string
		input  velero.RestoreItemActionExecuteInput
		skip   bool
		labels map[string]string
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
							"labels": map[string]string{},
						},
					},
				},
			},
			true,
			map[string]string{},
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
							"labels":    map[string]string{},
						},
					},
				},
			},
			false,
			map[string]string{},
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
			},
			false,
			map[string]string{
				"some.other/label": "test-value",
			},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMIRestoreItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := action.Execute(&tc.input)

			assert.NoError(t, err)
			assert.Equal(t, tc.skip, output.SkipRestore)
			if !tc.skip {
				metadata, err := meta.Accessor(output.UpdatedItem)
				assert.NoError(t, err)
				assert.Equal(t, tc.labels, metadata.GetLabels())
			}
		})
	}
}
