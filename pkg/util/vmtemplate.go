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
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kvv1 "kubevirt.io/api/core/v1"
)

// GetTemplateVM extracts and decodes the VirtualMachine embedded in a
// VirtualMachineTemplate's "spec.virtualMachine" field.
func GetTemplateVM(item runtime.Unstructured) (*kvv1.VirtualMachine, error) {
	vmMap, found, err := unstructured.NestedMap(item.UnstructuredContent(), "spec", "virtualMachine")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !found {
		return nil, nil
	}

	vm := new(kvv1.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(vmMap, vm); err != nil {
		return nil, errors.WithStack(err)
	}
	return vm, nil
}
