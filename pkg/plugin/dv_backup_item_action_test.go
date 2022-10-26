package plugin

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
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
		hasAnnPrePopulated bool
	}{
		{"Should add AnnPrePopulated to succeeded DV", &object, true},
		{"Should not add AnnPrePopulated to unfinished DV", &objectNotSucceeded, false},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewDVBackupItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item, _, _ := action.Execute(tc.dv, &v1.Backup{})

			metadata, _ := meta.Accessor(item)
			annotations := metadata.GetAnnotations()
			_, ok := annotations[AnnPrePopulated]
			assert.Equal(t, tc.hasAnnPrePopulated, ok, "hasAnnPrePopulated")
		})
	}

	t.Run("DV should request PVC to be backed up as well", func(t *testing.T) {
		_, extra, err := action.Execute(&object, &v1.Backup{})

		assert.NoError(t, err)
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
		{"Does not add AnnPopulatedFor when owned by another object", &unstructured.Unstructured{
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

	// TODO SKIP

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewDVBackupItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
				return &cdiv1.DataVolume{
						TypeMeta:   metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{},
						Spec:       cdiv1.DataVolumeSpec{},
						Status: cdiv1.DataVolumeStatus{
							Phase: "Succeeded",
						},
					},
					nil
			}
			item, _, err := action.Execute(tc.pvc, &v1.Backup{})

			assert.NoError(t, err)
			metadata, _ := meta.Accessor(item)
			annotations := metadata.GetAnnotations()
			if tc.shouldHaveAnn {
				assert.Contains(t, annotations, AnnPopulatedFor)
			} else {
				assert.NotContains(t, annotations, AnnPopulatedFor)
			}
			assert.NotContains(t, annotations, AnnInProgress)

		})
	}
}

func TestUnfinishedPVC(t *testing.T) {
	testCases := []struct {
		name string
		pvc  *unstructured.Unstructured
	}{
		{"Adds AnnInProgress when owned by an unfinished DV",
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
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewDVBackupItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
				return &cdiv1.DataVolume{
						TypeMeta:   metav1.TypeMeta{},
						ObjectMeta: metav1.ObjectMeta{},
						Spec:       cdiv1.DataVolumeSpec{},
						Status: cdiv1.DataVolumeStatus{
							Phase: "UNKNOWN",
						},
					},
					nil
			}
			item, _, err := action.Execute(tc.pvc, &v1.Backup{})

			assert.NoError(t, err)
			metadata, _ := meta.Accessor(item)
			annotations := metadata.GetAnnotations()

			assert.Contains(t, annotations, AnnInProgress)
		})
	}
}
