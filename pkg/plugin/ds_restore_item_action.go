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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// DSRestoreItemAction is a restore item action for DataSources
type DSRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewDSRestoreItemAction instantiates a DSRestoreItemAction.
func NewDSRestoreItemAction(log logrus.FieldLogger) *DSRestoreItemAction {
	return &DSRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *DSRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"datasources.cdi.kubevirt.io",
		},
	}, nil
}

// Execute returns the DataSource's backing PVC/VolumeSnapshot, or another DataSource it
// points to, as additional items to restore, and rewrites the explicit cross-namespace
// references in spec.source according to the restore's namespace mapping. Velero's namespace
// remapping only rewrites the DataSource's own metadata, not namespaces named inside its spec.
func (p *DSRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running DSRestoreItemAction")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	ds := new(cdiv1.DataSource)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), ds); err != nil {
		return nil, errors.WithStack(err)
	}

	// Build the additional-items graph before any remapping below: Velero resolves
	// AdditionalItems against their *backup* namespace to find them in the archive, and
	// applies the restore's namespace mapping itself afterwards.
	extra, err := kvgraph.NewDataSourceRestoreGraph(ds)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mapping := input.Restore.Spec.NamespaceMapping
	if len(mapping) > 0 {
		unstructuredItem, ok := input.Item.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("unexpected item type %T", input.Item)
		}
		remapDataSourceNamespaces(p.log, unstructuredItem, mapping)
	}

	output := velero.NewRestoreItemActionExecuteOutput(input.Item)
	output.AdditionalItems = extra

	return output, nil
}

// remapDataSourceNamespaces rewrites the namespaces of the DataSource's PVC, VolumeSnapshot
// and nested DataSource sources according to mapping, in place.
func remapDataSourceNamespaces(log logrus.FieldLogger, item *unstructured.Unstructured, mapping map[string]string) {
	remapNestedNamespace(log, item.Object, mapping, "DataSource PVC source", "spec", "source", "pvc", "namespace")
	remapNestedNamespace(log, item.Object, mapping, "DataSource Snapshot source", "spec", "source", "snapshot", "namespace")
	remapNestedNamespace(log, item.Object, mapping, "DataSource DataSource source", "spec", "source", "dataSource", "namespace")
}
