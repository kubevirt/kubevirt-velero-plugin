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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestDSBackupItemAction_AppliesTo(t *testing.T) {
	action := NewDSBackupItemAction(logrus.StandardLogger())

	selector, err := action.AppliesTo()
	assert.NoError(t, err)
	assert.Contains(t, selector.IncludedResources, "datasources.cdi.kubevirt.io")
}

func TestDSBackupItemAction_Execute(t *testing.T) {
	logrus.SetLevel(logrus.ErrorLevel)
	action := NewDSBackupItemAction(logrus.StandardLogger())

	t.Run("nil backup returns error", func(t *testing.T) {
		_, _, err := action.Execute(dataSourceItem(nil), nil)
		assert.Error(t, err)
	})

	t.Run("returns backing PVC as an extra item", func(t *testing.T) {
		origGetDV := util.GetDV
		defer func() { util.GetDV = origGetDV }()
		util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) { return &cdiv1.DataVolume{}, nil }

		item := dataSourceItem(map[string]interface{}{
			"pvc": map[string]interface{}{"name": "golden-image"},
		})

		_, extra, err := action.Execute(item, &v1.Backup{})
		assert.NoError(t, err)
		assert.Contains(t, extra, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"},
			Namespace:     testNamespace,
			Name:          "golden-image",
		})
		assert.Contains(t, extra, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
			Namespace:     testNamespace,
			Name:          "golden-image",
		})
	})

	t.Run("returns a nested DataSource as an extra item", func(t *testing.T) {
		item := dataSourceItem(map[string]interface{}{
			"dataSource": map[string]interface{}{"name": "parent-ds", "namespace": "other-ns"},
		})

		_, extra, err := action.Execute(item, &v1.Backup{})
		assert.NoError(t, err)
		assert.Equal(t, []velero.ResourceIdentifier{
			{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datasources"}, Namespace: "other-ns", Name: "parent-ds"},
		}, extra)
	})
}

func dataSourceItem(source map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cdi.kubevirt.io/v1beta1",
			"kind":       "DataSource",
			"metadata": map[string]interface{}{
				"name":      "test-datasource",
				"namespace": testNamespace,
			},
			"spec": map[string]interface{}{
				"source": source,
			},
		},
	}
}
