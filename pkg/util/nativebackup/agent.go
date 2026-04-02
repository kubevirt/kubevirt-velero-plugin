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
	"context"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// ShouldSkipQuiesce determines whether filesystem quiescing should be skipped.
// Returns true if:
// - The user explicitly set the skip-quiesce label
// - autoSkipQuiesceNoAgent is enabled and the guest agent is not connected
var ShouldSkipQuiesce = func(vm *kvcore.VirtualMachine, backup *v1.Backup, log logrus.FieldLogger) bool {
	// Explicit user override
	if metav1.HasLabel(backup.ObjectMeta, SkipQuiesceLabel) {
		log.Infof("Skip quiesce explicitly requested for VM %s/%s", vm.Namespace, vm.Name)
		return true
	}

	cfg := LoadConfig(backup)
	if !cfg.AutoSkipQuiesceNoAgent {
		return false
	}

	// Check VMI conditions for AgentConnected
	if !isGuestAgentConnected(vm.Namespace, vm.Name, log) {
		log.Warnf("QEMU guest agent not connected for VM %s/%s, auto-skipping quiesce", vm.Namespace, vm.Name)
		return true
	}

	return false
}

// isGuestAgentConnected checks if the QEMU guest agent is reported as connected
// in the VMI's status conditions.
func isGuestAgentConnected(ns, name string, log logrus.FieldLogger) bool {
	kvClient, err := util.GetKubeVirtclient()
	if err != nil {
		log.WithError(err).Warn("Failed to get KubeVirt client for agent detection")
		return false
	}

	vmi, err := (*kvClient).VirtualMachineInstance(ns).Get(
		context.TODO(), name, metav1.GetOptions{},
	)
	if err != nil {
		log.WithError(err).Warnf("Failed to get VMI %s/%s for agent detection", ns, name)
		return false
	}

	for _, c := range vmi.Status.Conditions {
		if c.Type == kvcore.VirtualMachineInstanceAgentConnected {
			return c.Status == corev1.ConditionTrue
		}
	}

	// No AgentConnected condition found at all
	return false
}
