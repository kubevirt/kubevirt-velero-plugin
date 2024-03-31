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
 * Copyright 2021 Red Hat, Inc.
 *
 */

package plugin

import (
	"fmt"

	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// PoBackupItemAction is a backup item action for backing up Pods
type PodBackupItemAction struct {
	log logrus.FieldLogger
}

// NewDVBackupItemAction instantiates a DVBackupItemAction.
func NewPodBackupItemAction(log logrus.FieldLogger) *PodBackupItemAction {
	return &PodBackupItemAction{log: log}
}

func (p *PodBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"Pods",
				"pod",
			},
		},
		nil
}

func (p *PodBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing PodBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	return p.handlePod(item)

}

func (p *PodBackupItemAction) handlePod(item runtime.Unstructured) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	var pod corev1api.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &pod); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	value, _ := pod.Labels["kubevirt.io"]
	if value == "hotplug-disk" {
		return nil, nil, nil
	}
	p.log.Infof("handling Pod %v/%v", pod.GetNamespace(), pod.GetName())
	extra := []velero.ResourceIdentifier{}

	podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: podMap}, extra, nil
}