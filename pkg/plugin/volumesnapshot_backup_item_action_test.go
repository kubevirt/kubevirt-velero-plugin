package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestVolumeSnapshotBackupExecute(t *testing.T) {
	testCases := []struct {
		name           string
		volumeSnapshot *snapshotv1.VolumeSnapshot
		expectedLabels map[string]string
		expectedAnnotations map[string]string
		expectSkip     bool
		mockPVCUID     string
		mockError      bool
	}{
		{
			"VolumeSnapshot without PVC source should be skipped",
			&snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vs",
					Namespace: "test-namespace",
					UID:       "vs-uid-123",
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						// No PersistentVolumeClaimName set
					},
				},
			},
			map[string]string{},
			map[string]string{},
			true,
			"",
			false,
		},
		{
			"Add PVC UID label to VolumeSnapshot with PVC source",
			&snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vs",
					Namespace: "test-namespace",
					UID:       "vs-uid-123",
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: stringPtr("test-pvc"),
					},
				},
			},
			map[string]string{
				util.PVCUIDLabel: "pvc-uid-456",
			},
			map[string]string{},
			false,
			"pvc-uid-456",
			false,
		},
		{
			"Preserve existing PVC UID label in annotation when collision occurs",
			&snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "collision-vs",
					Namespace: "test-namespace",
					UID:       "vs-uid-123",
					Labels: map[string]string{
						util.PVCUIDLabel: "original-user-value",
						"other-label":   "other-value",
					},
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: stringPtr("test-pvc"),
					},
				},
			},
			map[string]string{
				util.PVCUIDLabel: "pvc-uid-456",
				"other-label":   "other-value",
			},
			map[string]string{
				util.OriginalVolumeSnapshotUIDAnnotation: "original-user-value",
			},
			false,
			"pvc-uid-456",
			false,
		},
		{
			"Handle VolumeSnapshot with existing labels but no collision",
			&snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-labels-vs",
					Namespace: "test-namespace",
					UID:       "vs-uid-789",
					Labels: map[string]string{
						"existing-label": "existing-value",
					},
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: stringPtr("test-pvc"),
					},
				},
			},
			map[string]string{
				util.PVCUIDLabel:  "pvc-uid-456",
				"existing-label": "existing-value",
			},
			map[string]string{},
			false,
			"pvc-uid-456",
			false,
		},
		{
			"Skip when PVC UID is empty",
			&snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-uid-vs",
					Namespace: "test-namespace",
					UID:       "vs-uid-789",
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: stringPtr("test-pvc"),
					},
				},
			},
			map[string]string{},
			map[string]string{},
			true,
			"", // Empty UID should cause skip
			false,
		},
	}

	logger := logrus.StandardLogger()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create action with mocked client
			action := &VolumeSnapshotBackupItemAction{
				log:           logger,
				client:        nil, // Will be mocked in the action
				namespacePVCs: make(map[string]map[string]string),
			}

			// Pre-populate cache if we have a mock PVC UID or need to simulate empty UID
			if tc.volumeSnapshot.Spec.Source.PersistentVolumeClaimName != nil {
				action.namespacePVCs[tc.volumeSnapshot.Namespace] = map[string]string{
					*tc.volumeSnapshot.Spec.Source.PersistentVolumeClaimName: tc.mockPVCUID,
				}
			}

			// Convert to unstructured
			item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.volumeSnapshot)
			if !assert.NoError(t, err) {
				return
			}

			backup := &v1.Backup{}
			result, _, err := action.Execute(&unstructured.Unstructured{Object: item}, backup)

			if tc.expectSkip {
				// For cases where we skip processing, just verify no error
				assert.NoError(t, err)
				assert.NotNil(t, result)
				return
			}

			if tc.mockError {
				assert.Error(t, err)
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			// Extract the result VolumeSnapshot
			var resultVS snapshotv1.VolumeSnapshot
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(result.UnstructuredContent(), &resultVS)
			if !assert.NoError(t, err) {
				return
			}

			// Verify expected labels are present
			if resultVS.Labels == nil {
				resultVS.Labels = make(map[string]string)
			}

			assert.Equal(t, len(tc.expectedLabels), len(resultVS.Labels), "Unexpected number of labels")

			for expectedKey, expectedValue := range tc.expectedLabels {
				actualValue, exists := resultVS.Labels[expectedKey]
				assert.True(t, exists, "Expected label %s not found", expectedKey)
				assert.Equal(t, expectedValue, actualValue, "Label %s value mismatch", expectedKey)
			}

			// Verify expected annotations are present
			if len(tc.expectedAnnotations) > 0 {
				if resultVS.Annotations == nil {
					resultVS.Annotations = make(map[string]string)
				}

				for expectedKey, expectedValue := range tc.expectedAnnotations {
					actualValue, exists := resultVS.Annotations[expectedKey]
					assert.True(t, exists, "Expected annotation %s not found", expectedKey)
					assert.Equal(t, expectedValue, actualValue, "Annotation %s value mismatch", expectedKey)
				}
			} else if len(tc.expectedAnnotations) == 0 {
				// If no annotations are expected, verify none were added by the plugin
				for key := range resultVS.Annotations {
					if key == util.OriginalVolumeSnapshotUIDAnnotation {
						assert.Fail(t, "Unexpected collision annotation found when none expected")
					}
				}
			}
		})
	}
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}