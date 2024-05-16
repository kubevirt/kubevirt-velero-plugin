package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kvcore "kubevirt.io/api/core/v1"
)

func TestVmRestoreExecute(t *testing.T) {
	input := velero.RestoreItemActionExecuteInput{
		Item: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kubevirt.io/v1alpha3",
				"kind":       "VirtualMachine",
				"metadata": map[string]interface{}{
					"name":      "test-vm",
					"namespace": "test-namespace",
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
			},
		},
		Restore: &api.Restore{},
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

	t.Run("Should keep mac if restoring to same namespace", func(t *testing.T) {
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		vm := new(kvcore.VirtualMachine)
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), vm)
		assert.Nil(t, err)
		assert.Equal(t, vm.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress, "00:00:00:00:00:00")
		_, ok := vm.GetAnnotations()[AnnClearedMacs]
		assert.False(t, ok)
	})

	t.Run("Should delete mac if restoring to different namespace", func(t *testing.T) {
		input.Restore.Spec.NamespaceMapping = map[string]string{"test-namespace": "new-namespace"}
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		vm := new(kvcore.VirtualMachine)
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), vm)
		assert.Nil(t, err)
		assert.Equal(t, vm.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress, "")
		val := vm.GetAnnotations()[AnnClearedMacs]
		assert.Equal(t, val, "true")
	})
}
