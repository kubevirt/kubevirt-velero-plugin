package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					"runStrategy": "Always",
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
				},
			},
		},
		Restore: &velerov1.Restore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-restore",
				Namespace: "default",
			},
			Spec: velerov1.RestoreSpec{
				IncludedNamespaces: []string{"default"},
			},
		},
	}

	logrus.SetLevel(logrus.InfoLevel)
	action := NewVMRestoreItemAction(logrus.StandardLogger())
	t.Run("Running VM should be restored running", func(t *testing.T) {
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		spec := output.UpdatedItem.UnstructuredContent()["spec"].(map[string]interface{})
		assert.Equal(t, "Always", spec["runStrategy"])
	})

	t.Run("Stopped VM should be restored stopped", func(t *testing.T) {
		spec := input.Item.UnstructuredContent()["spec"].(map[string]interface{})
		spec["runStrategy"] = "Halted"
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		spec = output.UpdatedItem.UnstructuredContent()["spec"].(map[string]interface{})
		assert.Equal(t, "Halted", spec["runStrategy"])
	})

	t.Run("Stopped VM should be restored running when using appropriate label", func(t *testing.T) {
		spec := input.Item.UnstructuredContent()["spec"].(map[string]interface{})
		spec["runStrategy"] = "Halted"
		input.Restore.Labels = map[string]string{"velero.kubevirt.io/restore-run-strategy": "Always"}
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		spec = output.UpdatedItem.UnstructuredContent()["spec"].(map[string]interface{})
		assert.Equal(t, "Always", spec["runStrategy"])
	})

	t.Run("Running VM should be restored stopped when using appropriate label", func(t *testing.T) {
		spec := input.Item.UnstructuredContent()["spec"].(map[string]interface{})
		spec["runStrategy"] = "Always"
		input.Restore.Labels = map[string]string{"velero.kubevirt.io/restore-run-strategy": "Halted"}
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		spec = output.UpdatedItem.UnstructuredContent()["spec"].(map[string]interface{})
		assert.Equal(t, "Halted", spec["runStrategy"])
	})

	t.Run("Running field should be cleared when run strategy annotation", func(t *testing.T) {
		spec := input.Item.UnstructuredContent()["spec"].(map[string]interface{})
		spec["running"] = true
		spec["runStrategy"] = ""
		input.Restore.Labels = map[string]string{"velero.kubevirt.io/restore-run-strategy": "Halted"}
		output, err := action.Execute(&input)
		assert.Nil(t, err)

		spec = output.UpdatedItem.UnstructuredContent()["spec"].(map[string]interface{})
		assert.Equal(t, "Halted", spec["runStrategy"])
		assert.Nil(t, spec["running"])
	})

	t.Run("VM should return DVs as additional items", func(t *testing.T) {
		output, _ := action.Execute(&input)

		dvs := output.AdditionalItems
		assert.Equal(t, 2, len(dvs))
		assert.Equal(t, "test-dv-1", dvs[0].Name)
		assert.Equal(t, "test-dv-2", dvs[1].Name)
	})
}
