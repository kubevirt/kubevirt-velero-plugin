package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestPvcRestoreExecute(t *testing.T) {
	testCases := []struct {
		name           string
		input          velero.RestoreItemActionExecuteInput
		expectSkip     bool
		expectedLabels map[string]string
	}{
		{
			"Skip the unfinished PVC",
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
			true,
			nil,
		},
		{
			"Remove resource UID label from PVC",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "PersistentVolumeClaim",
						"metadata": map[string]interface{}{
							"name":      "test-pvc",
							"namespace": "test-namespace",
							"uid":       "633ab84c-8529-487c-8848-99b40fbda9f5",
							"labels": map[string]interface{}{
								util.PVCUIDLabel: "633ab84c-8529-487c-8848-99b40fbda9f5",
								"other-label":    "other-value",
							},
						},
						"spec": map[string]interface{}{},
					},
				},
			},
			false,
			map[string]string{
				"other-label": "other-value",
			},
		},
		{
			"Restore original UID label value from collision annotation",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "PersistentVolumeClaim",
						"metadata": map[string]interface{}{
							"name":      "collision-pvc",
							"namespace": "test-namespace",
							"uid":       "633ab84c-8529-487c-8848-99b40fbda9f5",
							"labels": map[string]interface{}{
								util.PVCUIDLabel: "633ab84c-8529-487c-8848-99b40fbda9f5", // Plugin-added during backup
								"other-label":    "other-value",
							},
							"annotations": map[string]interface{}{
								util.OriginalPVCUIDAnnotation: "original-user-uid-value", // User's original value
							},
						},
						"spec": map[string]interface{}{},
					},
				},
			},
			false,
			map[string]string{
				util.PVCUIDLabel: "original-user-uid-value", // Should be restored to original
				"other-label":    "other-value",
			},
		},
		{
			"Handle PVC without resource UID label",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "PersistentVolumeClaim",
						"metadata": map[string]interface{}{
							"name":      "test-pvc",
							"namespace": "test-namespace",
							"uid":       "789def01-2345-6789-abcd-ef0123456789",
							"labels": map[string]interface{}{
								"existing-label": "existing-value",
							},
						},
						"spec": map[string]interface{}{},
					},
				},
			},
			false,
			map[string]string{
				"existing-label": "existing-value",
			},
		},
		{
			"Handle PVC without any labels",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "PersistentVolumeClaim",
						"metadata": map[string]interface{}{
							"name":      "test-pvc",
							"namespace": "test-namespace",
						},
						"spec": map[string]interface{}{},
					},
				},
			},
			false,
			map[string]string{},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewPVCRestoreItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := action.Execute(&tc.input)
			if !assert.NoError(t, err) {
				return
			}

			if tc.expectSkip {
				assert.True(t, result.SkipRestore)
				return
			}

			assert.False(t, result.SkipRestore)

			// Extract the result PVC
			var resultPVC corev1api.PersistentVolumeClaim
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(result.UpdatedItem.UnstructuredContent(), &resultPVC)
			if !assert.NoError(t, err) {
				return
			}

			// Verify expected labels are present and resource name label is removed
			if tc.expectedLabels == nil {
				tc.expectedLabels = make(map[string]string)
			}

			if resultPVC.Labels == nil {
				resultPVC.Labels = make(map[string]string)
			}

			assert.Equal(t, len(tc.expectedLabels), len(resultPVC.Labels), "Unexpected number of labels")

			for expectedKey, expectedValue := range tc.expectedLabels {
				actualValue, exists := resultPVC.Labels[expectedKey]
				assert.True(t, exists, "Expected label %s not found", expectedKey)
				assert.Equal(t, expectedValue, actualValue, "Label %s value mismatch", expectedKey)
			}

			// Verify resource UID label was removed (unless it was restored to original)
			if tc.expectedLabels[util.PVCUIDLabel] == "" {
				_, exists := resultPVC.Labels[util.PVCUIDLabel]
				assert.False(t, exists, "Resource UID label should have been removed")
			}

			// Verify collision annotation was removed if it existed
			_, hasCollisionAnnotation := resultPVC.Annotations[util.OriginalPVCUIDAnnotation]
			assert.False(t, hasCollisionAnnotation, "Collision annotation should have been removed")
		})
	}
}

