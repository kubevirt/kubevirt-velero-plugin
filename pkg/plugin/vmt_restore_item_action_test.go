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

func TestVMTRestoreItemAction_AppliesTo(t *testing.T) {
	action := NewVMTRestoreItemAction(logrus.StandardLogger())

	selector, err := action.AppliesTo()
	assert.NoError(t, err)
	assert.Contains(t, selector.IncludedResources, "virtualmachinetemplates.template.kubevirt.io")
}

func TestVMTRestoreItemAction_Execute(t *testing.T) {
	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMTRestoreItemAction(logrus.StandardLogger())

	origGetDV := util.GetDV
	defer func() { util.GetDV = origGetDV }()
	util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) { return &cdiv1.DataVolume{}, nil }

	t.Run("nil input returns error", func(t *testing.T) {
		_, err := action.Execute(nil)
		assert.Error(t, err)
	})

	t.Run("remaps hardcoded namespaces and leaves parameterized ones alone", func(t *testing.T) {
		item := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "template.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineTemplate",
				"metadata": map[string]interface{}{
					"name":      "test-template",
					"namespace": "src-ns",
				},
				"spec": map[string]interface{}{
					"virtualMachine": map[string]interface{}{
						"metadata": map[string]interface{}{
							"namespace": "src-ns",
						},
						"spec": map[string]interface{}{
							"dataVolumeTemplates": []interface{}{
								map[string]interface{}{
									"spec": map[string]interface{}{
										"source": map[string]interface{}{
											"pvc": map[string]interface{}{
												"namespace": "src-ns",
												"name":      "golden-image",
											},
										},
									},
								},
								map[string]interface{}{
									"spec": map[string]interface{}{
										"source": map[string]interface{}{
											"pvc": map[string]interface{}{
												"namespace": "${NAMESPACE}",
												"name":      "other-image",
											},
										},
									},
								},
								map[string]interface{}{
									"spec": map[string]interface{}{
										"source": map[string]interface{}{
											"pvc": map[string]interface{}{
												"name": "same-ns-image",
											},
										},
									},
								},
							},
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"networks": []interface{}{
										map[string]interface{}{
											"name": "secondary",
											"multus": map[string]interface{}{
												"networkName": "src-ns/my-nad",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		input := &velero.RestoreItemActionExecuteInput{
			Item:           item,
			ItemFromBackup: item,
			Restore: &velerov1.Restore{
				Spec: velerov1.RestoreSpec{
					NamespaceMapping: map[string]string{"src-ns": "dst-ns"},
				},
			},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)

		vm, err := util.GetTemplateVM(output.UpdatedItem)
		assert.NoError(t, err)
		assert.Equal(t, "src-ns", vm.Namespace)
		assert.Equal(t, "dst-ns", vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC.Namespace)
		assert.Equal(t, "golden-image", vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC.Name)
		assert.Equal(t, "${NAMESPACE}", vm.Spec.DataVolumeTemplates[1].Spec.Source.PVC.Namespace)
		assert.Equal(t, "", vm.Spec.DataVolumeTemplates[2].Spec.Source.PVC.Namespace)
		assert.Equal(t, "dst-ns/my-nad", vm.Spec.Template.Spec.Networks[0].Multus.NetworkName)

		assert.Contains(t, output.AdditionalItems, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
			Namespace:     "src-ns",
			Name:          "golden-image",
		})
		assert.Contains(t, output.AdditionalItems, velero.ResourceIdentifier{
			GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
			Namespace:     "src-ns",
			Name:          "same-ns-image",
		})
	})

	t.Run("remap preserves fields unknown to the vendored KubeVirt API", func(t *testing.T) {
		item := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "template.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineTemplate",
				"metadata": map[string]interface{}{
					"name":      "test-template",
					"namespace": "src-ns",
				},
				"spec": map[string]interface{}{
					"virtualMachine": map[string]interface{}{
						"metadata": map[string]interface{}{
							"namespace": "src-ns",
						},
						"spec": map[string]interface{}{
							// A field the plugin's vendored KubeVirt API doesn't know about yet -
							// SetTemplateVM used to drop this by round-tripping through the typed
							// VirtualMachine struct; the current field-by-field remap must not.
							"futureFeature": "keep-me",
							"dataVolumeTemplates": []interface{}{
								map[string]interface{}{
									"spec": map[string]interface{}{
										"source": map[string]interface{}{
											"pvc": map[string]interface{}{
												"namespace": "src-ns",
												"name":      "golden-image",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		input := &velero.RestoreItemActionExecuteInput{
			Item:           item,
			ItemFromBackup: item,
			Restore: &velerov1.Restore{
				Spec: velerov1.RestoreSpec{
					NamespaceMapping: map[string]string{"src-ns": "dst-ns"},
				},
			},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)

		unstructuredItem, ok := output.UpdatedItem.(*unstructured.Unstructured)
		assert.True(t, ok)

		futureFeature, found, err := unstructured.NestedString(unstructuredItem.Object, "spec", "virtualMachine", "spec", "futureFeature")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "keep-me", futureFeature)

		vm, err := util.GetTemplateVM(output.UpdatedItem)
		assert.NoError(t, err)
		assert.Equal(t, "dst-ns", vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC.Namespace)
	})

	t.Run("no namespace mapping leaves the template untouched", func(t *testing.T) {
		item := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "template.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineTemplate",
				"metadata": map[string]interface{}{
					"name":      "test-template",
					"namespace": "tpl-ns",
				},
				"spec": map[string]interface{}{
					"virtualMachine": map[string]interface{}{
						"spec": map[string]interface{}{},
					},
				},
			},
		}

		input := &velero.RestoreItemActionExecuteInput{
			Item:           item,
			ItemFromBackup: item,
			Restore:        &velerov1.Restore{},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)
		assert.NotNil(t, output.UpdatedItem)
	})
}
