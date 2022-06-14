package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPvcRestoreExecute(t *testing.T) {
	testCases := []struct {
		name  string
		input velero.RestoreItemActionExecuteInput
	}{
		{"Skip the unfinished PVC ",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "PersistentVolumeClaim",
						"metadata": map[string]interface{}{
							"name": "test-pvc",
							"annotations": map[string]string{
								AnnInProgress: "test-pvc",
							},
							"ownerReferences": []interface{}{
								map[string]interface{}{
									"apiVersion": "cdi.kubevirt.io/v1beta1",
									"kind":       "DataVolume",
									"name":       "test-datavolume",
								},
							},
						},
						"spec": map[string]interface{}{},
					},
				},
			},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewPVCRestoreItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item, _ := action.Execute(&tc.input)
			assert.True(t, item.SkipRestore)
		})
	}
}
