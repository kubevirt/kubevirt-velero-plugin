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
	"regexp"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kvv1 "kubevirt.io/api/core/v1"
)

// nonStringParameterPlaceholder matches a virt-template "${{PARAM}}" placeholder, used for a
// non-string typed field (e.g. a number or boolean). Unlike "${PARAM}", which substitutes into
// a string field, this syntax can't decode into its target field's type until the template is
// processed and the placeholder is actually substituted.
var nonStringParameterPlaceholder = regexp.MustCompile(`^\$\{\{.+\}\}$`)

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

	// Drop non-string parameter placeholders before decoding: their real value is unknown
	// until the template is processed, and decoding them as-is into their target field's
	// (non-string) type would fail, incorrectly preventing the whole item from being backed
	// up or restored.
	stripNonStringParameterPlaceholders(vmMap)

	vm := new(kvv1.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(vmMap, vm); err != nil {
		return nil, errors.WithStack(err)
	}
	return vm, nil
}

// stripNonStringParameterPlaceholders removes, in place, every map entry whose value is a
// "${{PARAM}}" placeholder.
func stripNonStringParameterPlaceholders(obj map[string]interface{}) {
	for key, value := range obj {
		switch v := value.(type) {
		case string:
			if nonStringParameterPlaceholder.MatchString(v) {
				delete(obj, key)
			}
		case map[string]interface{}:
			stripNonStringParameterPlaceholders(v)
		case []interface{}:
			for _, item := range v {
				if nested, ok := item.(map[string]interface{}); ok {
					stripNonStringParameterPlaceholders(nested)
				}
			}
		}
	}
}
