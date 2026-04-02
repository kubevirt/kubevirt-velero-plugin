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

package nativebackup

import (
	"sync"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

var (
	crdCheckOnce sync.Once
	crdAvailable bool
)

// IsAvailable checks if the VirtualMachineBackup CRD is installed on the cluster.
// The result is cached for the lifetime of the plugin process.
func IsAvailable() bool {
	crdCheckOnce.Do(func() {
		crdAvailable = detectCRD()
	})
	return crdAvailable
}

// ResetDetectionCache allows tests to reset the CRD detection cache
var ResetDetectionCache = func() {
	crdCheckOnce = sync.Once{}
	crdAvailable = false
}

func detectCRD() bool {
	client, err := util.GetK8sClient()
	if err != nil {
		return false
	}

	_, resources, err := client.Discovery().ServerGroupsAndResources()
	if err != nil {
		return false
	}

	for _, list := range resources {
		if list.GroupVersion == "backup.kubevirt.io/v1alpha1" {
			for _, r := range list.APIResources {
				if r.Kind == "VirtualMachineBackup" {
					return true
				}
			}
		}
	}

	return false
}
