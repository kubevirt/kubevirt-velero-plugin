package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestVmRestoreExecute(t *testing.T) {
	input := velero.RestoreItemActionExecuteInput{
		Item: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kubevirt.io/v1alpha3",
				"kind":       "VirtualMachine",
				"metadata": map[string]interface{}{
					"name": "test-vm",
				},
				"spec": map[string]interface{}{
					"running": true,
					"dataVolumeTemplates": []map[string]interface{}{
						{"metadata": map[string]interface{}{
							"name": "test-dv-1",
						},
						},
						{"metadata": map[string]interface{}{
							"name": "test-dv-2",
						},
						},
					},
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"volumes": []map[string]interface{}{
								{
									"dataVolume": map[string]interface{}{
										"name": "test-dv-1",
									},
								},
								{
									"dataVolume": map[string]interface{}{
										"name": "test-dv-2",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	logrus.SetLevel(logrus.InfoLevel)
	action := NewVMRestoreItemAction(logrus.StandardLogger())
	t.Run("Running VM should be restored running", func(t *testing.T) {
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		spec := output.UpdatedItem.UnstructuredContent()["spec"].(map[string]interface{})
		assert.Equal(t, true, spec["running"])
	})

	t.Run("Stopped VM should be restored stopped", func(t *testing.T) {
		spec := input.Item.UnstructuredContent()["spec"].(map[string]interface{})
		spec["running"] = false
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		spec = output.UpdatedItem.UnstructuredContent()["spec"].(map[string]interface{})
		assert.Equal(t, false, spec["running"])
	})

	t.Run("VM should return DVs as additional items", func(t *testing.T) {
		output, _ := action.Execute(&input)

		dvs := output.AdditionalItems
		assert.Equal(t, 2, len(dvs))
		assert.Equal(t, "test-dv-1", dvs[0].Name)
		assert.Equal(t, "test-dv-2", dvs[1].Name)
	})
}
