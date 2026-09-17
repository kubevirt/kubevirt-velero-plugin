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
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestDSRestoreItemAction_AppliesTo(t *testing.T) {
	action := NewDSRestoreItemAction(logrus.StandardLogger())

	selector, err := action.AppliesTo()
	assert.NoError(t, err)
	assert.Contains(t, selector.IncludedResources, "datasources.cdi.kubevirt.io")
}

func TestDSRestoreItemAction_Execute(t *testing.T) {
	logrus.SetLevel(logrus.ErrorLevel)
	action := NewDSRestoreItemAction(logrus.StandardLogger())

	t.Run("nil input returns error", func(t *testing.T) {
		_, err := action.Execute(nil)
		assert.Error(t, err)
	})

	t.Run("returns backing Snapshot as an additional item", func(t *testing.T) {
		item := dataSourceItem(map[string]interface{}{
			"snapshot": map[string]interface{}{"name": "golden-snap"},
		})

		input := &velero.RestoreItemActionExecuteInput{
			Item:           item,
			ItemFromBackup: item,
			Restore:        &velerov1.Restore{},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)
		assert.Equal(t, []velero.ResourceIdentifier{
			{GroupResource: schema.GroupResource{Group: "snapshot.storage.k8s.io", Resource: "volumesnapshots"}, Namespace: testNamespace, Name: "golden-snap"},
		}, output.AdditionalItems)
	})

	t.Run("returns backing PVC and DataVolume without consulting the target cluster", func(t *testing.T) {
		origGetDV := util.GetDV
		defer func() { util.GetDV = origGetDV }()
		// The golden image has not been restored yet, so asking the target cluster whether
		// its DataVolume exists would always answer "no" and drop it from the restore.
		util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
			t.Fatalf("GetDV must not be called on the restore path")
			return nil, nil
		}

		item := dataSourceItem(map[string]interface{}{
			"pvc": map[string]interface{}{"name": "golden-image"},
		})

		input := &velero.RestoreItemActionExecuteInput{
			Item:           item,
			ItemFromBackup: item,
			Restore:        &velerov1.Restore{},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)
		assert.Equal(t, []velero.ResourceIdentifier{
			{GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}, Namespace: testNamespace, Name: "golden-image"},
			{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, Namespace: testNamespace, Name: "golden-image"},
		}, output.AdditionalItems)
	})

	t.Run("remaps cross-namespace source references and reports pre-remap additional items", func(t *testing.T) {
		item := dataSourceItem(map[string]interface{}{
			"pvc": map[string]interface{}{"name": "golden-image", "namespace": "golden-ns"},
		})

		input := &velero.RestoreItemActionExecuteInput{
			Item:           item,
			ItemFromBackup: item,
			Restore: &velerov1.Restore{
				Spec: velerov1.RestoreSpec{NamespaceMapping: map[string]string{"golden-ns": "restored-golden-ns"}},
			},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)

		// AdditionalItems are resolved against the backup namespace, so they stay unmapped.
		assert.Equal(t, []velero.ResourceIdentifier{
			{GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}, Namespace: "golden-ns", Name: "golden-image"},
			{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, Namespace: "golden-ns", Name: "golden-image"},
		}, output.AdditionalItems)

		// The restored DataSource itself must point at the mapped namespace.
		remapped, found, err := unstructured.NestedString(
			output.UpdatedItem.UnstructuredContent(), "spec", "source", "pvc", "namespace")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "restored-golden-ns", remapped)
	})

	t.Run("leaves namespaces absent from the mapping alone", func(t *testing.T) {
		item := dataSourceItem(map[string]interface{}{
			"snapshot": map[string]interface{}{"name": "golden-snap", "namespace": "golden-ns"},
		})

		input := &velero.RestoreItemActionExecuteInput{
			Item:           item,
			ItemFromBackup: item,
			Restore: &velerov1.Restore{
				Spec: velerov1.RestoreSpec{NamespaceMapping: map[string]string{"unrelated-ns": "somewhere-else"}},
			},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)

		unchanged, found, err := unstructured.NestedString(
			output.UpdatedItem.UnstructuredContent(), "spec", "source", "snapshot", "namespace")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "golden-ns", unchanged)
	})
}
