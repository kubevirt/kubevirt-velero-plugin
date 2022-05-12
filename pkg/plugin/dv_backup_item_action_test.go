package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDV(t *testing.T) {
	object := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cdi.kubevirt.io/v1beta1",
			"kind":       "DataVolume",
			"metadata": map[string]interface{}{
				"name": "test-datavolume",
			},
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"phase": "Succeeded",
			},
		},
	}

	objectNotSucceeded := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cdi.kubevirt.io/v1beta1",
			"kind":       "DataVolume",
			"metadata": map[string]interface{}{
				"name": "test-datavolume",
			},
			"spec": map[string]interface{}{},
		},
	}
	testCases := []struct {
		name               string
		dv                 *unstructured.Unstructured
		backup             v1.Backup
		hasAnnPrePopulated bool
	}{
		{"Should add AnnPrePopulated to succeeded DV", &object, v1.Backup{}, true},
		{"Should not add AnnPrePopulated to unfinished DV", &objectNotSucceeded, v1.Backup{}, false},
		{"Should not add AnnPrePopulated to DV without PVC",
			&object,
			v1.Backup{
				Spec: v1.BackupSpec{
					ExcludedResources: []string{"persistentvolumeclaims"},
				},
			},
			false,
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewDVBackupItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item, _, _ := action.Execute(tc.dv, &tc.backup)

			metadata, _ := meta.Accessor(item)
			annotations := metadata.GetAnnotations()
			_, ok := annotations[AnnPrePopulated]
			assert.Equal(t, tc.hasAnnPrePopulated, ok, "hasAnnPrePopulated")
		})
	}

	t.Run("DV should request PVC to be backed up as well", func(t *testing.T) {
		_, extra, _ := action.Execute(&object, &v1.Backup{})

		assert.Equal(t, 1, len(extra))
		assert.Equal(t, "persistentvolumeclaims", extra[0].Resource)
		assert.Equal(t, "test-datavolume", extra[0].Name)
	})
}

func TestPVC(t *testing.T) {
	testCases := []struct {
		name          string
		pvc           *unstructured.Unstructured
		shouldHaveAnn bool
	}{
		{"Adds AnnPopulatedFor when owned by a DV",
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "PersistentVolumeClaim",
					"metadata": map[string]interface{}{
						"name": "test-pvc",
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
			true},
		{"Does not add AnnPopulatedFor when not owned by a DV",
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "PersistentVolumeClaim",
					"metadata": map[string]interface{}{
						"name": "test-pvc",
					},
					"spec": map[string]interface{}{},
				},
			}, false},
		{"Does not add AnnPopulatedFor when not owned by a another object", &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "PersistentVolumeClaim",
				"metadata": map[string]interface{}{
					"name": "test-pvc",
					"ownerReferences": []interface{}{
						map[string]interface{}{
							"apiVersion": "v0.0.0",
							"kind":       "AnotherObject",
							"name":       "test-object",
						},
					},
				},
				"spec": map[string]interface{}{},
			},
		}, false},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewDVBackupItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item, _, _ := action.Execute(tc.pvc, &v1.Backup{})

			metadata, _ := meta.Accessor(item)
			annotations := metadata.GetAnnotations()
			if tc.shouldHaveAnn {
				assert.Contains(t, annotations, AnnPopulatedFor)
			} else {
				assert.NotContains(t, annotations, AnnPopulatedFor)
			}
		})
	}
}
