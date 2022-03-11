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
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

var _ = Describe("[smoke] VM Backup", func() {
	var client, _ = util.GetK8sClient()
	var cdiClient *cdiclientset.Clientset
	var kvClient *kubecli.KubevirtClient
	var namespace *v1.Namespace
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var r = framework.NewKubernetesReporter()

	const snapshotLocation = "test-location"

	// BeforeSuite(func() {
	BeforeEach(func() {
		var err error
		cdiClient, err = util.GetCDIclientset()
		Expect(err).ToNot(HaveOccurred())
		kvClient, err = util.GetKubeVirtclient()
		Expect(err).ToNot(HaveOccurred())

		// err = createSnapshotLocation(context.TODO(), snapshotLocation, "aws", "minio")
		// Expect(err).ToNot(HaveOccurred())
		// })

		// BeforeEach(func() {
		// var err error
		namespace, err = CreateNamespace(client)
		Expect(err).ToNot(HaveOccurred())

		timeout, cancelFunc = context.WithTimeout(context.Background(), 5*time.Minute)
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			r.FailureCount++
			r.Dump(CurrentGinkgoTestDescription().Duration)
		}

		By(fmt.Sprintf("Destroying namespace %q for this suite.", namespace.Name))
		err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}

		cancelFunc()
	})

	Context("VM", func() {
		var vm *kvv1.VirtualMachine

		BeforeEach(func() {
			var err error
			vmSpec := CreateVmWithGuestAgent("test-vm", r.StorageClass)
			vm, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())

			By("Starting VM")
			err = StartVirtualMachine(*kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForDataVolumePhase(cdiClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vm.Name, kvv1.Running)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())

			err = DeleteBackup(timeout, "test-backup", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Backing up stopped VM should succeed", func() {
			By("Stoppin a VM")
			err := StopVirtualMachine(*kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = CreateBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, "test-backup", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))
		})

		It("Backing up started VM should succeed", func() {
			err := WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vm.Name, kvv1.Running)
			Expect(err).ToNot(HaveOccurred())
			ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

			By("Creating backup")
			err = CreateBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, "test-backup", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))
		})

		It("Stoped VM should be restored", func() {
			By("Stoppin a VM")
			err := StopVirtualMachine(*kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = CreateBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, "test-backup", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting VM")
			err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating restore")
			err = CreateRestoreForBackup(timeout, "test-backup", "test-restore", r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := GetRestorePhase(timeout, "test-restore", r.BackupNamespace)
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
			err = CreateBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, "test-backup", r.BackupNamespace)
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
			err = CreateRestoreForBackup(timeout, "test-backup", "test-restore", r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := GetRestorePhase(timeout, "test-restore", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Verifying VM")
			err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
