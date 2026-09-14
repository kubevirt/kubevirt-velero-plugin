/*
 * This file is part of the Kubevirt Velero Plugin project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Velero Plugin Authors.
 *
 */

package plugin

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestVMTBackupItemAction_AppliesTo(t *testing.T) {
	action := NewVMTBackupItemAction(logrus.StandardLogger())

	selector, err := action.AppliesTo()
	assert.NoError(t, err)
	assert.Contains(t, selector.IncludedResources, "virtualmachinetemplates.template.kubevirt.io")
}

func TestVMTBackupItemAction_Execute(t *testing.T) {
	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMTBackupItemAction(logrus.StandardLogger())

	t.Run("nil backup returns error", func(t *testing.T) {
		_, _, err := action.Execute(templateItem(nil), nil)
		assert.Error(t, err)
	})

	t.Run("returns golden image DataVolume and PVC as extra items", func(t *testing.T) {
		origGetDV := util.GetDV
		defer func() { util.GetDV = origGetDV }()
		util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) { return &cdiv1.DataVolume{}, nil }

		item := templateItem([]interface{}{
			map[string]interface{}{
				"metadata": map[string]interface{}{"name": "rootdisk-${NAME}"},
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"pvc": map[string]interface{}{"name": "golden-image"},
					},
				},
			},
		})

		_, extra, err := action.Execute(item, &v1.Backup{})
		assert.NoError(t, err)
		assert.Len(t, extra, 2)
	})

	t.Run("template without an embedded VirtualMachine does not error", func(t *testing.T) {
		item := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "template.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineTemplate",
				"metadata": map[string]interface{}{
					"name":      "test-template",
					"namespace": testNamespace,
				},
				"spec": map[string]interface{}{},
			},
		}

		_, extra, err := action.Execute(item, &v1.Backup{})
		assert.NoError(t, err)
		assert.Empty(t, extra)
	})

	t.Run("warns if either DataVolumes or PersistentVolumeClaims are excluded from the backup", func(t *testing.T) {
		origIsDVExcludedByLabel := util.IsDVExcludedByLabel
		defer func() { util.IsDVExcludedByLabel = origIsDVExcludedByLabel }()
		util.IsDVExcludedByLabel = func(namespace, dvName string) (bool, error) { return false, nil }

		origGetDV := util.GetDV
		defer func() { util.GetDV = origGetDV }()
		util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) { return &cdiv1.DataVolume{}, nil }

		testCases := []struct {
			name             string
			excludedResource []string
			expectWarning    bool
		}{
			{"DataVolumes excluded only", []string{"datavolumes"}, true},
			{"PersistentVolumeClaims excluded only", []string{"persistentvolumeclaims"}, true},
			{"both excluded", []string{"datavolumes", "persistentvolumeclaims"}, true},
			{"neither excluded", nil, false},
		}

		item := templateItem([]interface{}{
			map[string]interface{}{
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"pvc": map[string]interface{}{"name": "golden-image"},
					},
				},
			},
		})

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var logOutput bytes.Buffer
				logger := logrus.New()
				logger.SetOutput(&logOutput)
				action := NewVMTBackupItemAction(logger)

				backup := &v1.Backup{Spec: v1.BackupSpec{ExcludedResources: tc.excludedResource}}
				_, _, err := action.Execute(item, backup)
				assert.NoError(t, err)

				if tc.expectWarning {
					assert.Contains(t, logOutput.String(), "golden image")
				} else {
					assert.NotContains(t, logOutput.String(), "golden image")
				}
			})
		}
	})
}

func templateItem(dataVolumeTemplates []interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "template.kubevirt.io/v1beta1",
			"kind":       "VirtualMachineTemplate",
			"metadata": map[string]interface{}{
				"name":      "test-template",
				"namespace": testNamespace,
			},
			"spec": map[string]interface{}{
				"virtualMachine": map[string]interface{}{
					"spec": map[string]interface{}{
						"dataVolumeTemplates": dataVolumeTemplates,
					},
				},
			},
		},
	}
}
