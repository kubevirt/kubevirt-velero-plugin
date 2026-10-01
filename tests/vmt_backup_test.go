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

// NOTE: These specs require the virt-template (https://github.com/kubevirt/virt-template) CRDs
// and controller to be available in the target cluster - KubeVirt v1.9+ deploys these
// automatically once its Template feature gate is enabled (on by default as of v1.9). They are
// skipped automatically (see the BeforeEach discovery check) when the template.kubevirt.io/v1beta1
// API is not available, so they are safe to run against clusters that don't have virt-template
// deployed.
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
	. "kubevirt.io/kubevirt-velero-plugin/tests/framework/matcher"
)

const (
	vmForTemplateDVName = "test-vm-for-template-dv"
	vmtrName            = "test-vmtr"
	vmtPollInterval     = 2 * time.Second
	vmtrReadyTimeout    = 300 * time.Second
	vmtSpecTimeout      = 15 * time.Minute
)

var (
	vmtGVR  = schema.GroupVersionResource{Group: "template.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinetemplates"}
	vmtrGVR = schema.GroupVersionResource{Group: "template.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinetemplaterequests"}
)

var _ = Describe("[smoke] VirtualMachineTemplate Backup", func() {
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string
	var dynClient dynamic.Interface

	var f = framework.NewFramework()

	BeforeEach(func() {
		_, err := f.K8sClient.Discovery().ServerResourcesForGroupVersion("template.kubevirt.io/v1beta1")
		if err != nil {
			Skip("virt-template CRDs not available on this cluster")
		}

		cfg, err := f.LoadConfig()
		Expect(err).ToNot(HaveOccurred())
		dynClient, err = dynamic.NewForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())

		timeout, cancelFunc = context.WithTimeout(context.Background(), vmtSpecTimeout)
		t := time.Now().UnixNano()
		backupName = fmt.Sprintf("test-backup-%d", t)
		restoreName = fmt.Sprintf("test-restore-%d", t)
	})

	AfterEach(func() {
		// Deleting the backup also deletes all restores, volume snapshots etc.
		err := framework.DeleteBackup(timeout, backupName, f.BackupNamespace)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		cancelFunc()
	})

	It("VirtualMachineTemplate created from a VM should be backed up and restored with its golden image", func() {
		By("Creating source VM")
		err := f.CreateVMForTemplate()
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the source VM's DataVolume to succeed")
		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmForTemplateDVName, 180, HaveSucceeded())

		By("Creating VirtualMachineTemplateRequest")
		err = f.CreateVirtualMachineTemplateRequest()
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the VirtualMachineTemplateRequest to become Ready")
		templateName, err := waitVMTRTemplateName(f, dynClient, f.Namespace.Name, vmtrName)
		Expect(err).ToNot(HaveOccurred())
		Expect(templateName).ToNot(BeEmpty())

		By("Determining the golden image DataVolume created for the template")
		goldenImageName, err := getTemplateGoldenImageDVName(dynClient, f.Namespace.Name, templateName)
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the golden image DataVolume to succeed")
		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, goldenImageName, 180, HaveSucceeded())

		By("Dumping VirtualMachineTemplate YAML before backup")
		dumpVMTYaml(dynClient, f.Namespace.Name, templateName)

		By("Creating backup with label selector (only the VirtualMachineTemplate is labeled, " +
			"the golden image must be discovered by the plugin)")
		err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Deleting the VirtualMachineTemplate, its golden image and the VirtualMachineTemplateRequest")
		err = deleteVirtualMachineTemplate(dynClient, f.Namespace.Name, templateName)
		Expect(err).ToNot(HaveOccurred())
		err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, goldenImageName)
		Expect(err).ToNot(HaveOccurred())
		ok, err := framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, goldenImageName)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		// The request is included in the backup (it carries the same label as the template,
		// see manifests/vmtr_from_vm.yaml), so without deleting it here it would still exist
		// untouched after the restore regardless of what VMTRRestoreItemAction does. Deleting
		// it - and waiting for virt-template's finalizer to actually let it go - makes the
		// "VMTR was not restored" assertion below meaningful.
		err = deleteVirtualMachineTemplateRequestAndWait(dynClient, f.Namespace.Name, vmtrName)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
		Expect(err).ToNot(HaveOccurred())

		By("Dumping VirtualMachineTemplate YAML after restore")
		dumpVMTYaml(dynClient, f.Namespace.Name, templateName)

		By("Verifying the VirtualMachineTemplate was restored")
		restoredTemplate, err := getVirtualMachineTemplate(dynClient, f.Namespace.Name, templateName)
		Expect(err).ToNot(HaveOccurred())
		Expect(restoredTemplate).ToNot(BeNil())

		By("Verifying the restored template still references the golden image by name")
		restoredGoldenImageName, err := getTemplateGoldenImageDVName(dynClient, f.Namespace.Name, templateName)
		Expect(err).ToNot(HaveOccurred())
		Expect(restoredGoldenImageName).To(Equal(goldenImageName))

		By("Verifying the golden image DataVolume was restored")
		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, goldenImageName, 180, HaveSucceeded())

		By("Verifying the VirtualMachineTemplateRequest was not restored")
		_, err = getVirtualMachineTemplateRequest(dynClient, f.Namespace.Name, vmtrName)
		Expect(apierrs.IsNotFound(err)).To(BeTrue(), "VirtualMachineTemplateRequest should not be restored")
	})

	It("VirtualMachineTemplate should be backed up and restored to a different namespace with namespace mapping", func() {
		By("Creating target namespace for namespace-mapped restore")
		targetNs, err := f.CreateNamespace()
		Expect(err).ToNot(HaveOccurred())
		f.AddNamespaceToDelete(targetNs)

		By("Creating source VM")
		err = f.CreateVMForTemplate()
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the source VM's DataVolume to succeed")
		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmForTemplateDVName, 180, HaveSucceeded())

		By("Creating VirtualMachineTemplateRequest")
		err = f.CreateVirtualMachineTemplateRequest()
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the VirtualMachineTemplateRequest to become Ready")
		templateName, err := waitVMTRTemplateName(f, dynClient, f.Namespace.Name, vmtrName)
		Expect(err).ToNot(HaveOccurred())
		Expect(templateName).ToNot(BeEmpty())

		By("Determining the golden image DataVolume created for the template")
		goldenImageName, err := getTemplateGoldenImageDVName(dynClient, f.Namespace.Name, templateName)
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for the golden image DataVolume to succeed")
		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, goldenImageName, 180, HaveSucceeded())

		By("Creating backup with label selector")
		err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Creating restore with namespace mapping")
		err = framework.CreateRestoreWithNamespaceMapping(timeout, backupName, restoreName, f.BackupNamespace,
			map[string]string{f.Namespace.Name: targetNs.Name},
			nil,
			true)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying the VirtualMachineTemplate was restored to the target namespace")
		restoredTemplate, err := getVirtualMachineTemplate(dynClient, targetNs.Name, templateName)
		Expect(err).ToNot(HaveOccurred())
		Expect(restoredTemplate).ToNot(BeNil())

		By("Verifying the restored template's golden image reference was remapped to the target namespace")
		vm, err := util.GetTemplateVM(restoredTemplate)
		Expect(err).ToNot(HaveOccurred())
		Expect(vm).ToNot(BeNil())
		Expect(vm.Spec.DataVolumeTemplates).ToNot(BeEmpty())
		pvcSource := vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC
		Expect(pvcSource).ToNot(BeNil())
		Expect(pvcSource.Name).To(Equal(goldenImageName))
		// virt-template always sets this namespace explicitly (it's never left empty), so
		// this pins down that VMTRestoreItemAction actually remapped it.
		Expect(pvcSource.Namespace).To(Equal(targetNs.Name))

		By("Verifying the golden image DataVolume was restored in the target namespace")
		framework.EventuallyDVWith(f.KvClient, targetNs.Name, goldenImageName, 180, HaveSucceeded())

		By("Verifying the original VirtualMachineTemplate still exists in the source namespace")
		_, err = getVirtualMachineTemplate(dynClient, f.Namespace.Name, templateName)
		Expect(err).ToNot(HaveOccurred())
	})
})

func waitVMTRTemplateName(f *framework.Framework, dynClient dynamic.Interface, namespace, name string) (string, error) {
	var templateName string
	err := wait.PollImmediate(vmtPollInterval, vmtrReadyTimeout, func() (bool, error) {
		obj, err := dynClient.Resource(vmtrGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
		if err != nil || !found {
			return false, nil
		}
		ready := false
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cond["type"] == "Ready" && cond["status"] == "True" {
				ready = true
				break
			}
		}
		if !ready {
			return false, nil
		}

		refName, found, err := unstructured.NestedString(obj.Object, "status", "templateRef", "name")
		if err != nil || !found || refName == "" {
			return false, nil
		}
		templateName = refName
		return true, nil
	})
	if err != nil {
		dumpVMTRDiagnostics(f, dynClient, namespace, name)
		return "", fmt.Errorf("VirtualMachineTemplateRequest %s/%s did not become Ready within %v: %w", namespace, name, vmtrReadyTimeout, err)
	}
	return templateName, nil
}

func dumpVMTRDiagnostics(f *framework.Framework, dynClient dynamic.Interface, namespace, name string) {
	ctx := context.Background()

	if obj, err := getVirtualMachineTemplateRequest(dynClient, namespace, name); err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to get VirtualMachineTemplateRequest %s/%s for dump: %v\n", namespace, name, err)
	} else if data, err := json.MarshalIndent(obj.Object, "", "  "); err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to marshal VirtualMachineTemplateRequest %s/%s: %v\n", namespace, name, err)
	} else {
		fmt.Fprintf(GinkgoWriter, "=== VirtualMachineTemplateRequest %s/%s ===\n%s\n", namespace, name, string(data))
	}

	if dvs, err := f.KvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to list DataVolumes in %s: %v\n", namespace, err)
	} else {
		fmt.Fprintf(GinkgoWriter, "=== DataVolumes in %s ===\n", namespace)
		for i := range dvs.Items {
			dv := &dvs.Items[i]
			fmt.Fprintf(GinkgoWriter, "  %s phase=%s progress=%s\n", dv.Name, dv.Status.Phase, dv.Status.Progress)
			for _, c := range dv.Status.Conditions {
				fmt.Fprintf(GinkgoWriter, "    condition %s=%s reason=%q message=%q\n", c.Type, c.Status, c.Reason, c.Message)
			}
		}
	}

	if pvcs, err := f.K8sClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to list PVCs in %s: %v\n", namespace, err)
	} else {
		fmt.Fprintf(GinkgoWriter, "=== PersistentVolumeClaims in %s ===\n", namespace)
		for i := range pvcs.Items {
			pvc := &pvcs.Items[i]
			fmt.Fprintf(GinkgoWriter, "  %s phase=%s class=%v volumeMode=%v accessModes=%v request=%v dataSource=%v\n",
				pvc.Name, pvc.Status.Phase, ptrStr(pvc.Spec.StorageClassName), ptrVolumeMode(pvc.Spec.VolumeMode),
				pvc.Spec.AccessModes, pvc.Spec.Resources.Requests.Storage(), pvc.Spec.DataSource)
		}
	}

	if events, err := f.K8sClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to list events in %s: %v\n", namespace, err)
	} else {
		fmt.Fprintf(GinkgoWriter, "=== Events in %s ===\n", namespace)
		for i := range events.Items {
			e := &events.Items[i]
			fmt.Fprintf(GinkgoWriter, "  %s %s/%s %s: %s\n", e.Type, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Reason, e.Message)
		}
	}
	fmt.Fprintf(GinkgoWriter, "=== End VirtualMachineTemplateRequest diagnostics ===\n")
}

func ptrStr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func ptrVolumeMode(m *corev1.PersistentVolumeMode) string {
	if m == nil {
		return "<nil>"
	}
	return string(*m)
}

func getTemplateGoldenImageDVName(dynClient dynamic.Interface, namespace, name string) (string, error) {
	obj, err := getVirtualMachineTemplate(dynClient, namespace, name)
	if err != nil {
		return "", err
	}

	vm, err := util.GetTemplateVM(obj)
	if err != nil {
		return "", err
	}
	if vm == nil || len(vm.Spec.DataVolumeTemplates) == 0 {
		return "", fmt.Errorf("VirtualMachineTemplate %s/%s has no dataVolumeTemplates", namespace, name)
	}

	source := vm.Spec.DataVolumeTemplates[0].Spec.Source
	if source == nil || source.PVC == nil {
		return "", fmt.Errorf("VirtualMachineTemplate %s/%s's first dataVolumeTemplate has no PVC source", namespace, name)
	}
	return source.PVC.Name, nil
}

func getVirtualMachineTemplate(dynClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	return dynClient.Resource(vmtGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func getVirtualMachineTemplateRequest(dynClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	return dynClient.Resource(vmtrGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func deleteVirtualMachineTemplate(dynClient dynamic.Interface, namespace, name string) error {
	err := dynClient.Resource(vmtGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

func deleteVirtualMachineTemplateRequestAndWait(dynClient dynamic.Interface, namespace, name string) error {
	err := dynClient.Resource(vmtrGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		return err
	}

	return wait.PollImmediate(vmtPollInterval, vmtrReadyTimeout, func() (bool, error) {
		_, err := getVirtualMachineTemplateRequest(dynClient, namespace, name)
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}

func dumpVMTYaml(dynClient dynamic.Interface, namespace, name string) {
	obj, err := getVirtualMachineTemplate(dynClient, namespace, name)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to get VirtualMachineTemplate %s/%s for dump: %v\n", namespace, name, err)
		return
	}
	data, err := json.MarshalIndent(obj.Object, "", "  ")
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to marshal VirtualMachineTemplate %s/%s: %v\n", namespace, name, err)
		return
	}
	fmt.Fprintf(GinkgoWriter, "=== VirtualMachineTemplate %s/%s YAML ===\n%s\n=== End VirtualMachineTemplate ===\n", namespace, name, string(data))
}
