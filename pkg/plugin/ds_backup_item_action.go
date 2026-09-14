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

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util/kvgraph"
)

// DSBackupItemAction is a backup item action for backing up DataSources
type DSBackupItemAction struct {
	log logrus.FieldLogger
}

// NewDSBackupItemAction instantiates a DSBackupItemAction.
func NewDSBackupItemAction(log logrus.FieldLogger) *DSBackupItemAction {
	return &DSBackupItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *DSBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"datasources.cdi.kubevirt.io",
		},
	}, nil
}

// Execute returns the DataSource's backing PVC/VolumeSnapshot, or another DataSource it
// points to, as extra items to back up.
func (p *DSBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing DSBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	ds := new(cdiv1.DataSource)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), ds); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	extra, err := kvgraph.NewDataSourceBackupGraph(ds)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return item, extra, nil
}
