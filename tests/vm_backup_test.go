package tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvv1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

const (
	dvName           = "test-dv"
	dvTemplateName   = "test-dv-template"
	instancetypeName = "test-vm-instancetype"
	preferenceName   = "test-vm-preference"
	acSecretName     = "test-access-credentials-secret"
	configMapName    = "test-configmap"
	secretName       = "test-secret"
)

var _ = Describe("[smoke] VM Backup", func() {
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string
	var vm *kvv1.VirtualMachine

	var f = framework.NewFramework()

	BeforeEach(func() {
		timeout, cancelFunc = context.WithTimeout(context.Background(), 10*time.Minute)
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

		err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		cancelFunc()
	})

	It("Stopped VM should be restored", func() {
		By(fmt.Sprintf("Creating DataVolume %s", dvName))
		err := f.CreateBlankDataVolume()
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForDataVolumePhase(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, dvName)
		Expect(err).ToNot(HaveOccurred())
		// creating a started VM, so it works correctly also on WFFC storage
		By("Starting a VM")
		err = f.CreateVMWithDVAndDVTemplate()
		Expect(err).ToNot(HaveOccurred())
		vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-dv-and-dvtemplate", dvTemplateName)
		Expect(err).ToNot(HaveOccurred())

		By("Stopping a VM")
		err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Creating backup")
		err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting DataVolume")
		err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
		Expect(err).ToNot(HaveOccurred())

		ok, err := framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())

		By("Creating restore")
		err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Checking DataVolume exists")
		err = framework.WaitForDataVolumePhase(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, dvName)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForDataVolumePhase(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, vm.Spec.DataVolumeTemplates[0].Name)
		Expect(err).ToNot(HaveOccurred())
	})

	It("started VM should be restored - with guest agent", func() {
		// creating a started VM, so it works correctly also on WFFC storage
		By("Starting a VM")
		vm, err := framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, framework.CreateVmWithGuestAgent("test-vm", f.StorageClass))
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
		ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

		By("Creating backup")
		err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Stopping VM")
		err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
	})

	It("started VM should be restored - without guest agent", func() {
		// creating a started VM, so it works correctly also on WFFC storage
		By("Starting a VM")
		vm, err := framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, framework.CreateVmWithGuestAgent("test-vm", f.StorageClass))
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		By("Creating backup")
		err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Stopping VM")
		err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("VM and VMI object graph backup", func() {
		It("with instancetype and preference", func() {
			By("Create instancetype and preference")
			err := f.CreateInstancetype()
			Expect(err).ToNot(HaveOccurred())
			err = f.CreatePreference()
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM")
			err = f.CreateVMWithInstancetypeAndPreference()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-instancetype-and-preference", dvName)
			Expect(err).ToNot(HaveOccurred())

			By("Wait instance type controller revision to be updated on VM spec")
			Eventually(func(g Gomega) {
				vm, err = f.KvClient.VirtualMachine(f.Namespace.Name).Get(vm.Name, &metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(vm.Spec.Instancetype.RevisionName).ToNot(BeEmpty())
				g.Expect(vm.Spec.Preference.RevisionName).ToNot(BeEmpty())
				_, err := f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Spec.Instancetype.RevisionName, metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())
				_, err = f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Spec.Preference.RevisionName, metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())
			}, 2*time.Minute, 2*time.Second).Should(Succeed())

			By("Creating backup")
			err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM, instancetype and preference")
			err = f.KvClient.VirtualMachineInstancetype(f.Namespace.Name).
				Delete(context.Background(), instancetypeName, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = f.KvClient.VirtualMachinePreference(f.Namespace.Name).
				Delete(context.Background(), preferenceName, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.DeleteVirtualMachineAndWait(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			// Wait until ControllerRevision is deleted
			Eventually(func(g Gomega) metav1.StatusReason {
				_, err := f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Spec.Instancetype.RevisionName, metav1.GetOptions{})
				if err != nil && errors.ReasonForError(err) != metav1.StatusReasonNotFound {
					return errors.ReasonForError(err)
				}
				_, err = f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Spec.Preference.RevisionName, metav1.GetOptions{})
				return errors.ReasonForError(err)
			}, 2*time.Minute, 2*time.Second).Should(Equal(metav1.StatusReasonNotFound))

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())
		})

		It("with configmap, secret and serviceaccount", func() {
			By("Creating configmap and secret")
			err := f.CreateConfigMap()
			Expect(err).ToNot(HaveOccurred())
			err = f.CreateSecret()
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM")
			err = f.CreateVMWithDifferentVolumes()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-different-volume-types", dvName)
			Expect(err).ToNot(HaveOccurred())

			By("Stopping a VM")
			err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM and volumes")
			err = deleteConfigMap(f.KvClient, configMapName, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			err = deleteSecret(f.KvClient, secretName, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying config map and secret exist")
			_, err = f.KvClient.CoreV1().ConfigMaps(f.Namespace.Name).Get(context.Background(), configMapName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = f.KvClient.CoreV1().Secrets(f.Namespace.Name).Get(context.Background(), secretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("with access credentials", func() {
			By("Creating access credentials")
			err := f.CreateAccessCredentialsSecret()
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM")
			err = f.CreateVMWithAccessCredentials()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-access-credentials", dvName)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM and access credentials secret")
			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = deleteSecret(f.KvClient, acSecretName, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Verifying secret exists")
			_, err = f.KvClient.CoreV1().Secrets(f.Namespace.Name).Get(context.Background(), acSecretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func deleteConfigMap(kvClient kubecli.KubevirtClient, name, namespace string) error {
	err := kvClient.CoreV1().ConfigMaps(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Wait until configmap is deleted
	Eventually(func(g Gomega) metav1.StatusReason {
		_, err = kvClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
		return errors.ReasonForError(err)
	}, 2*time.Minute, 2*time.Second).Should(Equal(metav1.StatusReasonNotFound))
	return nil
}

func deleteSecret(kvClient kubecli.KubevirtClient, name, namespace string) error {
	err := kvClient.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Wait until secret is deleted
	Eventually(func(g Gomega) metav1.StatusReason {
		_, err = kvClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		return errors.ReasonForError(err)
	}, 2*time.Minute, 2*time.Second).Should(Equal(metav1.StatusReasonNotFound))
	return nil
}
