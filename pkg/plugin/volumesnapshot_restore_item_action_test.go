package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestVolumeSnapshotRestoreExecute(t *testing.T) {
	testCases := []struct {
		name           string
		input          velero.RestoreItemActionExecuteInput
		expectedLabels map[string]string
	}{
		{
			"Remove PVC UID label from VolumeSnapshot",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "test-vs",
							"namespace": "test-namespace",
							"uid":       "vs-uid-123",
							"labels": map[string]interface{}{
								util.PVCUIDLabel: "pvc-uid-456",
								"other-label":    "other-value",
							},
						},
						"spec": map[string]interface{}{
							"source": map[string]interface{}{
								"persistentVolumeClaimName": "test-pvc",
							},
						},
					},
				},
			},
			map[string]string{
				"other-label": "other-value",
			},
		},
		{
			"Restore original PVC UID label value from collision annotation",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "collision-vs",
							"namespace": "test-namespace",
							"uid":       "vs-uid-123",
							"labels": map[string]interface{}{
								util.PVCUIDLabel: "pvc-uid-456", // Plugin-added during backup
								"other-label":    "other-value",
							},
							"annotations": map[string]interface{}{
								util.OriginalVolumeSnapshotUIDAnnotation: "original-user-uid-value", // User's original value
							},
						},
						"spec": map[string]interface{}{
							"source": map[string]interface{}{
								"persistentVolumeClaimName": "test-pvc",
							},
						},
					},
				},
			},
			map[string]string{
				util.PVCUIDLabel: "original-user-uid-value", // Should be restored to original
				"other-label":    "other-value",
			},
		},
		{
			"Handle VolumeSnapshot without PVC UID label",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "test-vs",
							"namespace": "test-namespace",
							"uid":       "vs-uid-789",
							"labels": map[string]interface{}{
								"existing-label": "existing-value",
							},
						},
						"spec": map[string]interface{}{
							"source": map[string]interface{}{
								"persistentVolumeClaimName": "test-pvc",
							},
						},
					},
				},
			},
			map[string]string{
				"existing-label": "existing-value",
			},
		},
		{
			"Handle VolumeSnapshot without any labels",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "snapshot.storage.k8s.io/v1",
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "test-vs",
							"namespace": "test-namespace",
						},
						"spec": map[string]interface{}{
							"source": map[string]interface{}{
								"persistentVolumeClaimName": "test-pvc",
							},
						},
					},
				},
			},
			map[string]string{},
		},
	}

	logger := logrus.StandardLogger()
	action := NewVolumeSnapshotRestoreItemAction(logger)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := action.Execute(&tc.input)
			if !assert.NoError(t, err) {
				return
			}

			assert.False(t, result.SkipRestore)

			// Extract the result VolumeSnapshot
			var resultVS snapshotv1.VolumeSnapshot
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(result.UpdatedItem.UnstructuredContent(), &resultVS)
			if !assert.NoError(t, err) {
				return
			}

			// Verify expected labels are present and PVC UID label is removed (unless restored to original)
			if tc.expectedLabels == nil {
				tc.expectedLabels = make(map[string]string)
			}

			if resultVS.Labels == nil {
				resultVS.Labels = make(map[string]string)
			}

			assert.Equal(t, len(tc.expectedLabels), len(resultVS.Labels), "Unexpected number of labels")

			for expectedKey, expectedValue := range tc.expectedLabels {
				actualValue, exists := resultVS.Labels[expectedKey]
				assert.True(t, exists, "Expected label %s not found", expectedKey)
				assert.Equal(t, expectedValue, actualValue, "Label %s value mismatch", expectedKey)
			}

			// Verify PVC UID label was removed (unless it was restored to original)
			if tc.expectedLabels[util.PVCUIDLabel] == "" {
				_, exists := resultVS.Labels[util.PVCUIDLabel]
				assert.False(t, exists, "PVC UID label should have been removed")
			}

			// Verify collision annotation was removed if it existed
			_, hasCollisionAnnotation := resultVS.Annotations[util.OriginalVolumeSnapshotUIDAnnotation]
			assert.False(t, hasCollisionAnnotation, "Collision annotation should have been removed")
		})
	}
}