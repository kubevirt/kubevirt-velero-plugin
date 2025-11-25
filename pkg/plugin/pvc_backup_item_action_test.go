package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestPVCBackupExecute(t *testing.T) {
	testCases := []struct {
		name                string
		pvc                 *corev1.PersistentVolumeClaim
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		expectSkip          bool
	}{
		{
			"Add UID label to PVC without existing labels",
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test-namespace",
					UID:       "pvc-uid-123",
				},
				Spec: corev1.PersistentVolumeClaimSpec{},
			},
			map[string]string{
				util.PVCUIDLabel: "pvc-uid-123",
			},
			map[string]string{},
			false,
		},
		{
			"Add UID label to PVC with existing labels",
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-labels-pvc",
					Namespace: "test-namespace",
					UID:       "pvc-uid-456",
					Labels: map[string]string{
						"existing-label": "existing-value",
						"another-label":  "another-value",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{},
			},
			map[string]string{
				util.PVCUIDLabel:  "pvc-uid-456",
				"existing-label": "existing-value",
				"another-label":  "another-value",
			},
			map[string]string{},
			false,
		},
		{
			"Preserve existing PVC UID label in annotation when collision occurs",
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "collision-pvc",
					Namespace: "test-namespace",
					UID:       "pvc-uid-789",
					Labels: map[string]string{
						util.PVCUIDLabel: "original-user-value",
						"other-label":    "other-value",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{},
			},
			map[string]string{
				util.PVCUIDLabel: "pvc-uid-789",
				"other-label":    "other-value",
			},
			map[string]string{
				util.OriginalPVCUIDAnnotation: "original-user-value",
			},
			false,
		},
		{
			"Handle PVC with existing annotations and collision",
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-annotations-pvc",
					Namespace: "test-namespace",
					UID:       "pvc-uid-101",
					Labels: map[string]string{
						util.PVCUIDLabel: "user-set-value",
					},
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{},
			},
			map[string]string{
				util.PVCUIDLabel: "pvc-uid-101",
			},
			map[string]string{
				"existing-annotation":         "existing-value",
				util.OriginalPVCUIDAnnotation: "user-set-value",
			},
			false,
		},
		{
			"Skip PVC with empty UID",
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-uid-pvc",
					Namespace: "test-namespace",
					// UID is empty
				},
				Spec: corev1.PersistentVolumeClaimSpec{},
			},
			map[string]string{},
			map[string]string{},
			true,
		},
		{
			"Handle PVC with same UID value as existing label (preserve it anyway)",
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "same-uid-pvc",
					Namespace: "test-namespace",
					UID:       "pvc-uid-same",
					Labels: map[string]string{
						util.PVCUIDLabel: "pvc-uid-same", // Same as UID but user might have set it
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{},
			},
			map[string]string{
				util.PVCUIDLabel: "pvc-uid-same",
			},
			map[string]string{
				util.OriginalPVCUIDAnnotation: "pvc-uid-same", // Still preserve original
			},
			false,
		},
	}

	logger := logrus.StandardLogger()
	action := NewPVCBackupItemAction(logger)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert to unstructured
			item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.pvc)
			if !assert.NoError(t, err) {
				return
			}

			backup := &v1.Backup{}
			result, _, err := action.Execute(&unstructured.Unstructured{Object: item}, backup)

			if tc.expectSkip {
				// For cases where we skip processing, just verify no error and item unchanged
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, item, result.UnstructuredContent())
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			// Extract the result PVC
			var resultPVC corev1.PersistentVolumeClaim
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(result.UnstructuredContent(), &resultPVC)
			if !assert.NoError(t, err) {
				return
			}

			// Verify expected labels are present
			if resultPVC.Labels == nil {
				resultPVC.Labels = make(map[string]string)
			}

			assert.Equal(t, len(tc.expectedLabels), len(resultPVC.Labels), "Unexpected number of labels")

			for expectedKey, expectedValue := range tc.expectedLabels {
				actualValue, exists := resultPVC.Labels[expectedKey]
				assert.True(t, exists, "Expected label %s not found", expectedKey)
				assert.Equal(t, expectedValue, actualValue, "Label %s value mismatch", expectedKey)
			}

			// Verify expected annotations are present
			if len(tc.expectedAnnotations) > 0 {
				if resultPVC.Annotations == nil {
					resultPVC.Annotations = make(map[string]string)
				}

				for expectedKey, expectedValue := range tc.expectedAnnotations {
					actualValue, exists := resultPVC.Annotations[expectedKey]
					assert.True(t, exists, "Expected annotation %s not found", expectedKey)
					assert.Equal(t, expectedValue, actualValue, "Annotation %s value mismatch", expectedKey)
				}
			} else if len(tc.expectedAnnotations) == 0 {
				// If no annotations are expected, verify none were added by the plugin
				for key := range resultPVC.Annotations {
					if key == util.OriginalPVCUIDAnnotation {
						assert.Fail(t, "Unexpected collision annotation found when none expected")
					}
				}
			}
		})
	}
}

