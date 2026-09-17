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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// VMTRestoreItemAction is a restore item action for VirtualachineTemplates
type VMTRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewVMTRestoreItemAction instantiates a VMTRestoreItemAction.
func NewVMTRestoreItemAction(log logrus.FieldLogger) *VMTRestoreItemAction {
	return &VMTRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMTRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"virtualmachinetemplates.template.kubevirt.io",
		},
	}, nil
}

// Execute rewrites hardcoded namespace references embedded in the template's VirtualMachine
// (spec.virtualMachine) according to the restore's namespace mapping. Velero's namespace
// remapping only rewrites the VirtualMachineTemplate's own metadata; references embedded
// inside its opaque spec.virtualMachine field are invisible to it.
func (p *VMTRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running VMTRestoreItemAction")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	vm, err := util.GetTemplateVM(input.Item)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	accessor, err := meta.Accessor(input.Item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// At this point in the restore, the item's own namespace is still the *backup*
	// namespace - Velero only applies the namespace mapping to the item's metadata
	// after all RestoreItemActions have run.
	namespace := accessor.GetNamespace()

	// Build the additional-items graph from the VirtualMachine as it appears in the
	// backup, before any namespace remapping below. Velero resolves AdditionalItems
	// against their *backup* namespace to find them in the archive, and only then
	// applies the restore's namespace mapping itself; an already-remapped identifier
	// here would not be found in the archive.
	extra, err := kvgraph.NewVirtualMachineTemplateRestoreGraph(vm, namespace)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mapping := input.Restore.Spec.NamespaceMapping
	if len(mapping) > 0 {
		unstructuredItem, ok := input.Item.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("unexpected item type %T", input.Item)
		}
		if _, err := remapTemplateVM(p.log, unstructuredItem, mapping); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	output := velero.NewRestoreItemActionExecuteOutput(input.Item)
	output.AdditionalItems = extra

	return output, nil
}

func remapTemplateVM(log logrus.FieldLogger, item *unstructured.Unstructured, mapping map[string]string) (bool, error) {
	dvtsChanged, err := remapDataVolumeTemplates(log, item, mapping)
	if err != nil {
		return false, err
	}

	networksChanged, err := remapNetworks(log, item, mapping)
	if err != nil {
		return false, err
	}

	return dvtsChanged || networksChanged, nil
}

func remapDataVolumeTemplates(log logrus.FieldLogger, item *unstructured.Unstructured, mapping map[string]string) (bool, error) {
	dvts, found, err := unstructured.NestedSlice(item.Object, "spec", "virtualMachine", "spec", "dataVolumeTemplates")
	if err != nil || !found {
		return false, err
	}

	changed := false
	for i := range dvts {
		if dvt, ok := dvts[i].(map[string]interface{}); ok && remapDataVolumeTemplateNamespace(log, dvt, mapping) {
			changed = true
		}
	}
	if !changed {
		return false, nil
	}

	if err := unstructured.SetNestedSlice(item.Object, dvts, "spec", "virtualMachine", "spec", "dataVolumeTemplates"); err != nil {
		return false, err
	}
	return true, nil
}

func remapDataVolumeTemplateNamespace(log logrus.FieldLogger, dvt map[string]interface{}, mapping map[string]string) bool {
	changed := false
	if remapNestedNamespace(log, dvt, mapping, "DataVolumeTemplate PVC source", "spec", "source", "pvc", "namespace") {
		changed = true
	}
	if remapNestedNamespace(log, dvt, mapping, "DataVolumeTemplate Snapshot source", "spec", "source", "snapshot", "namespace") {
		changed = true
	}
	if remapNestedNamespace(log, dvt, mapping, "DataVolumeTemplate SourceRef", "spec", "sourceRef", "namespace") {
		changed = true
	}
	return changed
}

func remapNetworks(log logrus.FieldLogger, item *unstructured.Unstructured, mapping map[string]string) (bool, error) {
	networks, found, err := unstructured.NestedSlice(item.Object, "spec", "virtualMachine", "spec", "template", "spec", "networks")
	if err != nil || !found {
		return false, err
	}

	changed := false
	for i := range networks {
		if net, ok := networks[i].(map[string]interface{}); ok && remapNetworkNamespace(log, net, mapping) {
			changed = true
		}
	}
	if !changed {
		return false, nil
	}

	if err := unstructured.SetNestedSlice(item.Object, networks, "spec", "virtualMachine", "spec", "template", "spec", "networks"); err != nil {
		return false, err
	}
	return true, nil
}

func remapNetworkNamespace(log logrus.FieldLogger, net map[string]interface{}, mapping map[string]string) bool {
	networkName, found, err := unstructured.NestedString(net, "multus", "networkName")
	if err != nil || !found || networkName == "" {
		return false
	}
	parts := strings.SplitN(networkName, "/", 2)
	if len(parts) != 2 {
		return false
	}
	ns, name := parts[0], parts[1]
	if kvgraph.IsParameterized(ns) {
		return false
	}
	mapped, ok := mapping[ns]
	if !ok || mapped == ns {
		return false
	}
	log.Infof("Mapping Multus network namespace from %s to %s", ns, mapped)
	return unstructured.SetNestedField(net, fmt.Sprintf("%s/%s", mapped, name), "multus", "networkName") == nil
}

func remapNestedNamespace(log logrus.FieldLogger, obj map[string]interface{}, mapping map[string]string, what string, fields ...string) bool {
	namespace, found, err := unstructured.NestedString(obj, fields...)
	if err != nil || !found || namespace == "" {
		return false
	}

	remapped := remapNamespace(log, what, namespace, mapping)
	if remapped == namespace {
		return false
	}

	return unstructured.SetNestedField(obj, remapped, fields...) == nil
}

func remapNamespace(log logrus.FieldLogger, what, namespace string, mapping map[string]string) string {
	if namespace == "" || kvgraph.IsParameterized(namespace) {
		return namespace
	}
	if mapped, ok := mapping[namespace]; ok && mapped != namespace {
		log.Infof("Mapping %s namespace from %s to %s", what, namespace, mapped)
		return mapped
	}
	return namespace
}
