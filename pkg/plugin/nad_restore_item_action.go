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
 * Copyright 2023 Red Hat, Inc.
 *
 */

package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// NADRestoreItemAction is a restore item action for NetworkAttachmentDefinitions
type NADRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewNADRestoreItemAction instantiates a NADRestoreItemAction.
func NewNADRestoreItemAction(log logrus.FieldLogger) *NADRestoreItemAction {
	return &NADRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *NADRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"network-attachment-definition",
		},
	}, nil
}

// Execute performs the restore action for a NetworkAttachmentDefinition.
func (p *NADRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Executing NADRestoreItemAction")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	var nad netv1.NetworkAttachmentDefinition
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &nad); err != nil {
		return nil, errors.WithStack(err)
	}

	p.log.Infof("handling NetworkAttachmentDefinition %v/%v", nad.GetNamespace(), nad.GetName())

	if nad.Spec.Config != "" {
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(nad.Spec.Config), &config); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal NAD config JSON")
		}

		if val, ok := config["netAttachDefName"]; ok {
			if netAttachDefName, isString := val.(string); isString && netAttachDefName != "" {
				parts := strings.Split(netAttachDefName, "/")
				
				// If it's in the format "namespace/name"
				if len(parts) == 2 {
					ns := parts[0]
					name := parts[1]
					
					// Check if the namespace is in the Velero restore namespace mapping
					if mappedNs, mapped := input.Restore.Spec.NamespaceMapping[ns]; mapped {
						p.log.Infof("Mapping netAttachDefName namespace from %s to %s", ns, mappedNs)
						config["netAttachDefName"] = fmt.Sprintf("%s/%s", mappedNs, name)
						
						// Marshal the modified config back to JSON
						newConfigBytes, err := json.Marshal(config)
						if err != nil {
							return nil, errors.Wrap(err, "failed to marshal updated NAD config JSON")
						}
						nad.Spec.Config = string(newConfigBytes)

						// Convert back to unstructured since we modified the object
						item, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&nad)
						if err != nil {
							return nil, errors.WithStack(err)
						}
						
						return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: item}), nil
					}
				}
			}
		}
	}

	return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
}
