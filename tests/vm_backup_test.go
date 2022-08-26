package tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvv1 "kubevirt.io/client-go/api/v1"
	kubecli "kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

var _ = Describe("[smoke] VM Backup", func() {
	var client, _ = util.GetK8sClient()
	var kvClient *kubecli.KubevirtClient
	var namespace *v1.Namespace
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string
	var vm *kvv1.VirtualMachine

	var r = framework.NewKubernetesReporter()

	const snapshotLocation = "test-location"

	BeforeEach(func() {
		var err error
		kvClient, err = util.GetKubeVirtclient()
		Expect(err).ToNot(HaveOccurred())

		timeout, cancelFunc = context.WithTimeout(context.Background(), 10*time.Minute)
		t := time.Now().UnixNano()
		backupName = fmt.Sprintf("test-backup-%d", t)
		restoreName = fmt.Sprintf("test-restore-%d", t)

		namespace, err = CreateNamespace(client)
		Expect(err).ToNot(HaveOccurred())

		vmSpec := CreateVmWithGuestAgent("test-vm", r.StorageClass)
		vm, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
		Expect(err).ToNot(HaveOccurred())

		By("Starting VM")
		err = StartVirtualMachine(*kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		err = WaitForDataVolumePhase(*kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
		Expect(err).ToNot(HaveOccurred())
		err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vm.Name, kvv1.Running)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			r.FailureCount++
			r.Dump(CurrentGinkgoTestDescription().Duration)
		}

		// Deleting the backup also deletes all restores, volume snapshots etc.
		err := DeleteBackup(timeout, backupName, r.BackupNamespace)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		By(fmt.Sprintf("Destroying namespace %q for this suite.", namespace.Name))
		err = client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		cancelFunc()
	})

	It("Stopped VM should be restored", func() {
		By("Stopping a VM")
		err := StopVirtualMachine(*kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating backup")
		err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := GetBackupPhase(timeout, backupName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Deleting VM")
		err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := GetRestorePhase(timeout, restoreName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())
	})

	It("started VM should be restored", func() {
		err := WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
		ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

		By("Creating backup")
		err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := GetBackupPhase(timeout, backupName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Stopping VM")
		err = StopVirtualMachine(*kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting VM")
		err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := GetRestorePhase(timeout, restoreName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
	})
})
