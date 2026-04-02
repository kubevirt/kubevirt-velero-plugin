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
	"fmt"
	"strings"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var vmBackupGVR = schema.GroupVersionResource{
	Group:    "backup.kubevirt.io",
	Version:  "v1alpha1",
	Resource: "virtualmachinebackups",
}

// SourceRef identifies the source for a VirtualMachineBackup
type SourceRef struct {
	APIGroup string // "kubevirt.io" or "backup.kubevirt.io"
	Kind     string // "VirtualMachine" or "VirtualMachineBackupTracker"
	Name     string
}

// CreateParams contains parameters for creating a VirtualMachineBackup CR
type CreateParams struct {
	Name            string
	Namespace       string
	Source          SourceRef
	PVCName         string
	Mode            string // "Push" or "Pull"
	SkipQuiesce     bool
	ForceFullBackup bool
}

// BackupCRName returns a deterministic name for the VirtualMachineBackup CR
func BackupCRName(veleroBackupName, vmName string) string {
	name := fmt.Sprintf("velero-%s-%s", veleroBackupName, vmName)
	if len(name) > 253 {
		name = name[:253]
	}
	return name
}

// apiContext returns a context with a 30-second timeout for Kubernetes API calls.
func apiContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// ParseOperationID splits a "namespace/name" operation ID.
// Returns an error if the format is invalid.
func ParseOperationID(operationID string) (string, string, error) {
	parts := strings.SplitN(operationID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid operation ID %q: expected namespace/name format", operationID)
	}
	return parts[0], parts[1], nil
}

// CreateVMBackupCR creates a VirtualMachineBackup CR using the dynamic client
var CreateVMBackupCR = func(params CreateParams) error {
	client, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to get dynamic client: %w", err)
	}

	spec := map[string]interface{}{
		"source": map[string]interface{}{
			"apiGroup": params.Source.APIGroup,
			"kind":     params.Source.Kind,
			"name":     params.Source.Name,
		},
		"mode":    params.Mode,
		"pvcName": params.PVCName,
	}

	if params.SkipQuiesce {
		spec["skipQuiesce"] = true
	}
	if params.ForceFullBackup {
		spec["forceFullBackup"] = true
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "backup.kubevirt.io/v1alpha1",
			"kind":       "VirtualMachineBackup",
			"metadata": map[string]interface{}{
				"name":      params.Name,
				"namespace": params.Namespace,
				"labels": map[string]interface{}{
					ScratchPVCBackupLabel: params.Name,
				},
			},
			"spec": spec,
		},
	}

	ctx, cancel := apiContext()
	defer cancel()
	_, err = client.Resource(vmBackupGVR).Namespace(params.Namespace).Create(
		ctx, obj, metav1.CreateOptions{},
	)
	return err
}

// GetVMBackup retrieves a VirtualMachineBackup CR by namespace and name
var GetVMBackup = func(ns, name string) (*unstructured.Unstructured, error) {
	client, err := getDynamicClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := apiContext()
	defer cancel()
	return client.Resource(vmBackupGVR).Namespace(ns).Get(
		ctx, name, metav1.GetOptions{},
	)
}

// DeleteVMBackupCR deletes a VirtualMachineBackup CR
var DeleteVMBackupCR = func(ns, name string) error {
	client, err := getDynamicClient()
	if err != nil {
		return err
	}

	ctx, cancel := apiContext()
	defer cancel()
	return client.Resource(vmBackupGVR).Namespace(ns).Delete(
		ctx, name, metav1.DeleteOptions{},
	)
}

// VMBackupExists checks if a VirtualMachineBackup CR already exists (for idempotency)
func VMBackupExists(ns, name string) (bool, error) {
	_, err := GetVMBackup(ns, name)
	if err == nil {
		return true, nil
	}
	if k8serrors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

func getDynamicClient() (dynamic.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(clientConfig)
}
