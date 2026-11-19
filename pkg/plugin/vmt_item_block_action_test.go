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
)

func TestVMTItemBlockAction_AppliesTo(t *testing.T) {
	action := NewVMTItemBlockAction(logrus.StandardLogger())

	selector, err := action.AppliesTo()
	assert.NoError(t, err)
	assert.Contains(t, selector.IncludedResources, "virtualmachinetemplates.template.kubevirt.io")
}

func TestVMTItemBlockAction_GetRelatedItems(t *testing.T) {
	action := NewVMTItemBlockAction(logrus.StandardLogger())
	backup := &v1.Backup{}

	item := templateItem([]interface{}{
		map[string]interface{}{
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"pvc": map[string]interface{}{"name": "golden-image"},
				},
			},
		},
	})

	relatedItems, err := action.GetRelatedItems(item, backup)
	assert.NoError(t, err)
	assert.Len(t, relatedItems, 2)
}

func TestVMTItemBlockAction_Name(t *testing.T) {
	action := NewVMTItemBlockAction(logrus.StandardLogger())
	assert.Equal(t, "VMTItemBlockAction", action.Name())
}
