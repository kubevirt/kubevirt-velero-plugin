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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kvv1 "kubevirt.io/api/core/v1"
)

func TestGetTemplateVM(t *testing.T) {
	t.Run("extracts the embedded VirtualMachine", func(t *testing.T) {
		item := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"virtualMachine": map[string]interface{}{
						"metadata": map[string]interface{}{
							"name": "${NAME}",
						},
						"spec": map[string]interface{}{
							"runStrategy": "Always",
						},
					},
				},
			},
		}

		vm, err := GetTemplateVM(item)
		assert.NoError(t, err)
		assert.NotNil(t, vm)
		assert.Equal(t, "${NAME}", vm.Name)
		assert.Equal(t, kvv1.RunStrategyAlways, *vm.Spec.RunStrategy)
	})

	t.Run("returns nil when the field is absent", func(t *testing.T) {
		item := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
		}

		vm, err := GetTemplateVM(item)
		assert.NoError(t, err)
		assert.Nil(t, vm)
	})

	t.Run("returns an error when a non-string parameter placeholder doesn't decode", func(t *testing.T) {
		item := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"virtualMachine": map[string]interface{}{
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"domain": map[string]interface{}{
										// A "${{COUNT}}" placeholder for a non-string field
										// (cpu.cores is a uint32) decodes as a string here, since
										// substitution happens later, when the template is processed.
										"cpu": map[string]interface{}{"cores": "${{COUNT}}"},
									},
								},
							},
						},
					},
				},
			},
		}

		vm, err := GetTemplateVM(item)
		assert.Error(t, err)
		assert.Nil(t, vm)
	})
}
