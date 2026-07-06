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
 * Copyright 2025 Red Hat, Inc.
 *
 */

package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func TestVMItemBlockAction_AppliesTo(t *testing.T) {
	logger := logrus.StandardLogger()
	action := NewVMItemBlockAction(logger)

	selector, err := action.AppliesTo()
	assert.NoError(t, err)
	assert.Contains(t, selector.IncludedResources, "virtualmachines.kubevirt.io")
}

func TestVMItemBlockAction_GetRelatedItems(t *testing.T) {
	logger := logrus.StandardLogger()
	action := NewVMItemBlockAction(logger)
	backup := &v1.Backup{}

	t.Run("Valid VirtualMachine", func(t *testing.T) {
		vm := &kubevirtv1.VirtualMachine{
			TypeMeta: metav1.TypeMeta{
				Kind:       "VirtualMachine",
				APIVersion: "kubevirt.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vm",
				Namespace: "test-namespace",
			},
		}

		item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
		assert.NoError(t, err)

		unstructuredItem := &unstructured.Unstructured{Object: item}
		
		// We expect no error during conversion and execution
		relatedItems, err := action.GetRelatedItems(unstructuredItem, backup)
		assert.NoError(t, err)
		assert.NotNil(t, relatedItems)
	})

	t.Run("Invalid Unstructured Data", func(t *testing.T) {
		invalidItem := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": "this-should-be-a-map-not-a-string",
			},
		}

		_, err := action.GetRelatedItems(invalidItem, backup)
		assert.Error(t, err)
	})
}
