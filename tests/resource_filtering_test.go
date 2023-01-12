package tests

import (
	"context"
	"fmt"
	"time"

	kubernetes "k8s.io/client-go/kubernetes"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

var _ = Describe("Resource includes", func() {
	var client, _ = util.GetK8sClient()
	var kvClient kubecli.KubevirtClient
	var namespace *v1.Namespace
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string
	var r = framework.NewKubernetesReporter()

	BeforeEach(func() {
		kvClientRef, err := util.GetKubeVirtclient()
		Expect(err).ToNot(HaveOccurred())
		kvClient = *kvClientRef

		timeout, cancelFunc = context.WithTimeout(context.Background(), 10*time.Minute)
		t := time.Now().UnixNano()
		backupName = fmt.Sprintf("test-backup-%d", t)
		restoreName = fmt.Sprintf("test-restore-%d", t)

		namespace, err = CreateNamespace(client)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			r.FailureCount++
			r.Dump(CurrentGinkgoTestDescription().Duration)
		}

		// Deleting the backup also deletes all restores, volume snapshots etc.
		err := framework.DeleteBackup(timeout, backupName, r.BackupNamespace)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		err = client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		cancelFunc()
	})

	Context("Include namespace", func() {
		var includedNamespace *v1.Namespace
		var otherNamespace *v1.Namespace

		BeforeEach(func() {
			var err error
			includedNamespace, err = CreateNamespace(client)
			Expect(err).ToNot(HaveOccurred())
			otherNamespace, err = CreateNamespace(client)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := client.CoreV1().Namespaces().Delete(context.TODO(), includedNamespace.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}

			err = client.CoreV1().Namespaces().Delete(context.TODO(), otherNamespace.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("Should only backup and restore DV from included namespace", func() {
			By("Creating DVs")
			dvSpec := framework.NewDataVolumeForBlankRawImage("included-test-dv", "100Mi", r.StorageClass)
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, includedNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForDataVolumePhase(kvClient, includedNamespace.Name, cdiv1.Succeeded, "included-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvSpec = framework.NewDataVolumeForBlankRawImage("other-test-dv", "100Mi", r.StorageClass)
			dvOther, err := framework.CreateDataVolumeFromDefinition(kvClient, otherNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForDataVolumePhase(kvClient, otherNamespace.Name, cdiv1.Succeeded, "other-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Crating backup test-backup")
			err = framework.CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting DVs")
			err = framework.DeleteDataVolume(kvClient, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitDataVolumeDeleted(kvClient, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteDataVolume(kvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitDataVolumeDeleted(kvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking included DataVolume exists")
			err = framework.WaitForDataVolumePhase(kvClient, includedNamespace.Name, cdiv1.Succeeded, "included-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Checking not included DataVolume does not exist")
			ok, err = framework.WaitDataVolumeDeleted(kvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = framework.DeleteDataVolume(kvClient, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should only backup and restore VM from included namespace", func() {
			By("Creating VirtualMachines")
			vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
			vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, includedNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForDataVolumePhase(kvClient, includedNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
			vmOther, err := framework.CreateVirtualMachineFromDefinition(kvClient, otherNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForDataVolumePhase(kvClient, otherNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = framework.CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VMs")
			err = framework.DeleteVirtualMachine(kvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(kvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteVirtualMachine(kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitVirtualMachineDeleted(kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying included VM exists")
			err = framework.WaitForVirtualMachineStatus(kvClient, includedNamespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying ignored VM does not exists")
			ok, err = framework.WaitVirtualMachineDeleted(kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = framework.DeleteVirtualMachine(kvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Include resources", func() {

		Context("Standalone DV", func() {
			It("Selecting DV+PVC: Both DVs and PVCs should be backed up and restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = framework.CreateBackupForResources(timeout, backupName, "datavolumes,persistentvolumeclaims", namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following 2 items:
				// - DataVolume
				// - PVC
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(2))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitPVCDeleted(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				_, err = framework.WaitForPVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume exists and succeeds")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting DV+PVC+PV+VolumeSnapshot+VSContent: Both DVs and PVCs should be backed up and restored, content of PVC not re-imported", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				resources := "datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following items:
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - a number of unbound PVs
				Expect(backup.Status.Progress.ItemsBackedUp).To(BeNumerically(">=", 5))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = framework.WaitForPVCPhase(client, namespace.Name, "test-dv", v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting only DVs: the restored DV should not recreate its PVC", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = framework.CreateBackupForResources(timeout, backupName, "datavolumes", namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following item:
				// - DataVolume
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(1))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitPVCDeleted(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting only PVCs: PVC should be restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				resources := "persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following items:
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - a number of unbound PVs
				Expect(backup.Status.Progress.ItemsBackedUp).To(BeNumerically(">=", 4))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = framework.WaitForPVCPhase(client, namespace.Name, "test-dv", v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())
				pvc, err := framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(len(pvc.OwnerReferences)).To(Equal(0))

				By("Checking DataVolume does not exist")
				Consistently(func() bool {
					_, err := framework.FindDataVolume(kvClient, namespace.Name, "test-dv")
					return apierrs.IsNotFound(err)
				}, "1000ms", "100ms").Should(BeTrue())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM with DVTemplates", func() {
			It("Selecting VM+DV+PVC: VM, DV and PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("included-test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, volumeName)
				Expect(err).ToNot(HaveOccurred())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, writerPod(volumeName))
				deletePod(kvClient, namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Wait for DataVolume")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "included-test-vm-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying DataVolume does not re-import content - file should exists")
				readerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, verifyFileExists(volumeName))
				deletePod(kvClient, namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+DV+PVC: Backing up VM should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances,persistentvolumeclaims"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod+PVC: Backup should succeed, DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.StopVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.DeleteDataVolume(kvClient, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, volumeName)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, writerPod(volumeName))
				deletePod(kvClient, namespace.Name, writerPod.Name)

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.StopVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				// Testing for ImportScheduled is not reliable, because sometimes it might happen so fast,
				// that DV switches to Succeeded before we even get here
				By("Checking DataVolume import succeeds")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, volumeName)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying DataVolume is re-imported - file should not exists")
				readerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, verifyNoFile(volumeName))
				deletePod(kvClient, namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not VMI or Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not VMI and Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				err = framework.PauseVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.StopVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-vm-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+DV+PVC+VMI+Pod: All objects should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+DV: VM, DV should be restored, PVC should not be recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+PVC: VM and PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, writerPod(volumeName))
				deletePod(kvClient, namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				// DV may not exist, so just check the PVC
				By("Verifying PVC is NOT re-imported - file exists")
				readerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, verifyFileExists(volumeName))
				deletePod(kvClient, namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not DV and PVC: VM should be restored, DV and PVC should be recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, writerPod(volumeName))
				deletePod(kvClient, namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "virtualmachines"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying DataVolume is re-imported - file should not exists")
				readerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, verifyNoFile(volumeName))
				deletePod(kvClient, namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VMI (with DV+PVC+Pod) but not VM: Backing up VMI should fail", func() {
				By("Creating VirtualMachine")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting VirtualMachine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup with DV+PVC+Pod")
				resources := "datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VMI (without DV+PVC+Pod) but not VM: Backing up VMI should fail", func() {
				By("Creating VirtualMachine")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting VirtualMachine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup without DV+PVC+Pod")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {

			It("Selecting VM+DV+PVC, VM stopped: VM, DV and PVC should be restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM + PVC, VM stopped: VM and PVC should be restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					PersistentVolumeClaim: &kvv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test-dv",
						}},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, "tet-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM + PVC, VM running: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not PVC: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("[smoke] Standalone VMI", func() {
			// This test tries to backup on all namespaces, on some clusters it always fails
			// need to be improved
			XIt("Selecting standalone VMI+DV+PVC+Pod: All objects should be restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI running")
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, "test-vmi", kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv-2")
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI+Pod without DV: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI+Pod without PVC: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI without Pod: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Selector includes", func() {

		Context("Standalone DV", func() {
			It("Should only backup and restore DV selected by label", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("included-test-dv", "100Mi", r.StorageClass)
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "included-test-dv")
				Expect(err).ToNot(HaveOccurred())

				dvSpec = framework.NewDataVolumeForBlankRawImage("other-test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvOther, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "other-test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Crating backup test-backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=include", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking included DataVolume exists")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking not included DataVolume does not exist")
				ok, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Backup of DVs selected by label should include PVCs", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("included-test-dv", "100Mi", r.StorageClass)
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "included-test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Crating backup test-backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=include", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contains the following 7 items:
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - VolumeSpapshotClass
				// - Datavolume resource definition
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(7))

				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvSpec.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM with DVTemplates and standalone DVs", func() {
			It("Backup of a stopped VM selected by label should include its DVs and PVCs", func() {
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				dvSpec.Annotations[forceBindAnnotation] = "true"

				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, dvSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				// creating a started VM, so it works correctly also on WFFC storage
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmSpec.Spec.Template.Spec.Domain.Devices.Disks = append(vmSpec.Spec.Template.Spec.Domain.Devices.Disks, kvv1.Disk{
					Name: "volume2",
					DiskDevice: kvv1.DiskDevice{
						Disk: &kvv1.DiskTarget{
							Bus: "virtio",
						},
					},
				})
				vmSpec.Spec.Template.Spec.Volumes = append(vmSpec.Spec.Template.Spec.Volumes, kvv1.Volume{
					Name: "volume2",
					VolumeSource: kvv1.VolumeSource{
						DataVolume: &kvv1.DataVolumeSource{
							Name: dvSpec.Name,
						},
					},
				})
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}

				vm, err := framework.CreateStartedVirtualMachine(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Stopping a VM")
				err = framework.StopVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contain the following 13 items:
				// - VirtualMachine
				// - 2 DataVolume
				// - 2 PVC
				// - 2 PV
				// - 2 VolumeSnapshot
				// - 2 VolumeSnapshotContent
				// - VolumeSpapshotClass
				// - Datavolume resource definition
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(13))
			})
		})
		Context("VM with DVTemplates", func() {
			It("Backup of a stopped VMs selected by label should include its DVs and PVCs", func() {
				By("Creating VirtualMachines")

				vmSpec := framework.CreateVmWithGuestAgent("included-test-vm", r.StorageClass)
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				_, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
				_, err = framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contains the following 8 items:
				// - VirtualMachine
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - VolumeSpapshotClass
				// - Datavolume resource definition
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(8))
			})

			It("Backup of a running VMs selected by label should include its DVs and PVCs, VMIs and Pods", func() {
				By("Creating VirtualMachines")

				vmSpec := framework.CreateVmWithGuestAgent("included-test-vm", r.StorageClass)
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				vm, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting VM")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contains the following 10 items:
				// - VirtualMachine
				// - VirtualMachineInstance
				// - Launcher pod
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - VolumeSpapshotClass
				// - Datavolume resource definition
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(10))
			})
		})

		Context("[smoke] Standalone VMI", func() {
			It("Backup of VMIs selected by label should include its DVs, PVCs, and Pods", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				dvSpec2 := framework.NewDataVolumeForBlankRawImage("test-dv-2", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec2.Name))
				_, err = framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec2)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv-2")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				pvcVolume := kvv1.VolumeSource{
					PersistentVolumeClaim: &kvv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test-dv-2",
						}},
				}
				addVolumeToVMI(vmiSpec, pvcVolume, "pvc-volume")
				vmiSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contains the following 13 items:
				// - VirtualMachineInstance
				// - Launcher pod
				// - DataVolume
				// - DV's PVC
				// - DV's PVC's PV
				// - standalone PVC
				// - standaolne PVC's PV
				// - VolumeSnapshot (DV)
				// - VolumeSnapshotContent (DV)
				// - VolumeSnapshot (PVC)
				// - VolumeSnapshotContent (PVC)
				// - VolumeSpapshotClass
				// - VMI resource definition
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(13))

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv-2")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

var _ = Describe("Resource excludes", func() {
	var client, _ = util.GetK8sClient()
	var kvClient kubecli.KubevirtClient
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var namespace *v1.Namespace
	var backupName string
	var restoreName string
	var r = framework.NewKubernetesReporter()

	BeforeEach(func() {
		kvClientRef, err := util.GetKubeVirtclient()
		Expect(err).ToNot(HaveOccurred())
		kvClient = *kvClientRef

		timeout, cancelFunc = context.WithTimeout(context.Background(), 10*time.Minute)
		namespace, err = CreateNamespace(client)
		Expect(err).ToNot(HaveOccurred())
		t := time.Now().UnixNano()
		backupName = fmt.Sprintf("test-backup-%d", t)
		restoreName = fmt.Sprintf("test-restore-%d", t)
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			r.FailureCount++
			r.Dump(CurrentGinkgoTestDescription().Duration)
		}

		// Deleting the backup also deletes all restores, volume snapshots etc.
		err := framework.DeleteBackup(timeout, backupName, r.BackupNamespace)
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

	Context("Exclude namespace", func() {
		var excludedNamespace *v1.Namespace
		var otherNamespace *v1.Namespace

		BeforeEach(func() {
			var err error
			excludedNamespace, err = CreateNamespace(client)
			Expect(err).ToNot(HaveOccurred())
			otherNamespace, err = CreateNamespace(client)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := client.CoreV1().Namespaces().Delete(context.TODO(), excludedNamespace.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}

			err = client.CoreV1().Namespaces().Delete(context.TODO(), otherNamespace.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("Should not backup and restore DV from excluded namespace", func() {
			By("Creating DVs")
			dvSpec := framework.NewDataVolumeForBlankRawImage("excluded-test-dv", "100Mi", r.StorageClass)
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvExcluded, err := framework.CreateDataVolumeFromDefinition(kvClient, excludedNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForDataVolumePhase(kvClient, excludedNamespace.Name, cdiv1.Succeeded, "excluded-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvSpec = framework.NewDataVolumeForBlankRawImage("other-test-dv", "100Mi", r.StorageClass)
			dvOther, err := framework.CreateDataVolumeFromDefinition(kvClient, otherNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForDataVolumePhase(kvClient, otherNamespace.Name, cdiv1.Succeeded, "other-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Crating backup test-backup")
			err = framework.CreateBackupForNamespaceExcludeNamespace(timeout, backupName, otherNamespace.Name, excludedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting DVs")
			err = framework.DeleteDataVolume(kvClient, excludedNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitDataVolumeDeleted(kvClient, excludedNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteDataVolume(kvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitDataVolumeDeleted(kvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking included DataVolume exists")
			err = framework.WaitForDataVolumePhase(kvClient, otherNamespace.Name, cdiv1.Succeeded, "other-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Checking not included DataVolume does not exist")
			ok, err = framework.WaitDataVolumeDeleted(kvClient, excludedNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = framework.DeleteDataVolume(kvClient, otherNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should not backup and restore VM from excluded namespace", func() {
			By("Creating VirtualMachines")
			//vmSpec := newVMSpecBlankDVTemplate("excluded-test-vm", "100Mi")
			vmSpec := framework.CreateVmWithGuestAgent("excluded-test-vm", r.StorageClass)
			vmExcluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, excludedNamespace.Name, vmSpec)

			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForDataVolumePhase(kvClient, excludedNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			//vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
			vmSpec = framework.CreateVmWithGuestAgent("other-test-vm", r.StorageClass)
			vmOther, err := framework.CreateVirtualMachineFromDefinition(kvClient, otherNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForDataVolumePhase(kvClient, otherNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = framework.CreateBackupForNamespaceExcludeNamespace(timeout, backupName, otherNamespace.Name, excludedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VMs")
			err = framework.DeleteVirtualMachine(kvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(kvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteVirtualMachine(kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitVirtualMachineDeleted(kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying included VM exists")
			err = framework.WaitForVirtualMachineStatus(kvClient, otherNamespace.Name, vmOther.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying ignored VM does not exists")
			ok, err = framework.WaitVirtualMachineDeleted(kvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = framework.DeleteVirtualMachine(kvClient, otherNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Exclude resources", func() {
		Context("Standalone DV", func() {
			It("[negative] PVC excluded: DV restored, PVC will not be re-imported", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "persistentvolumeclaims", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, r.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitPVCDeleted(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("DV excluded: PVC restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "datavolumes", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = framework.WaitForPVCPhase(client, namespace.Name, "test-dv", v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())
				pvc, err := framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pvc.OwnerReferences)).To(Equal(0))

				By("Checking DataVolume does not exist")
				Consistently(func() bool {
					_, err := framework.FindDataVolume(kvClient, namespace.Name, "test-dv")
					return apierrs.IsNotFound(err)
				}, "1000ms", "100ms").Should(BeTrue())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM with DVTemplates", func() {
			It("Pods excluded, VM running: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "pods", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Pods+DV excluded, VM running: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods,datavolumes"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[negative] Pods+PVC excluded, VM running: VM+DV restored, PVC not re-imported", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods,persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Pods excluded, VM stopped: VM+DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Pods excluded, VM paused: VM+DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI excluded, Pod not excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("PVC excluded: DV restored, PVC not re-imported", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not reimport")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("DV+PVC excluded: VM restored, DV+PVC recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolume,persistentvolumeclaim"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("DV excluded: VM+PVC restored, DV recreated and bound to the PVC", func() {
				By("Creating VirtualMachines")
				//vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, writerPod(volumeName))
				deletePod(kvClient, namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "datavolume"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying DataVolume does not re-import content - file should exists")
				readerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, verifyFileExists(volumeName))
				deletePod(kvClient, namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Running VM excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachine"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Stopped VM excluded: DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vm, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachine"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Delete VM")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				err = framework.DeleteDataVolume(kvClient, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM does not exists")
				_, err = framework.GetVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {
			It("VM with DV Volume, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolumes"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM with DV Volume, DV included, PVC excluded: VM+DV recreated, PVC not recreated and re-imported", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not reimport")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, source.DataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusProvisioning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM with PVC Volume, PVC excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					PersistentVolumeClaim: &kvv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test-dv",
						}},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Standalone VMI", func() {
			It("VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Pause VMI")
				err = framework.PauseVirtualMachine(kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pod"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI running")
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, "test-vmi", kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
			})

			It("[smoke] Pod included, VMI excluded: backup should succeed, only DV and PVC restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI not present")
				_, err = framework.GetVirtualMachineInstance(kvClient, namespace.Name, "test-vmi")
				Expect(err).To(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI+Pod included, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolumes"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Exclude label", func() {
		addExcludeLabel := func(labels map[string]string) map[string]string {
			if labels == nil {
				labels = make(map[string]string)
			}
			labels["velero.io/exclude-from-backup"] = "true"
			return labels
		}

		addExcludeLabelToDV := func(name string) {
			updateFunc := func(dataVolume *cdiv1.DataVolume) *cdiv1.DataVolume {
				dataVolume.SetLabels(addExcludeLabel(dataVolume.GetLabels()))
				return dataVolume
			}

			retryOnceOnErr(updateDataVolume(kvClient, namespace.Name, name, updateFunc)).Should(BeNil())
		}

		addExcludeLabelToPVC := func(name string) {
			update := func(pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
				pvc.SetLabels(addExcludeLabel(pvc.GetLabels()))
				return pvc
			}
			retryOnceOnErr(updatePvc(client, namespace.Name, name, update)).Should(BeNil())
		}

		addExcludeLabelToVMI := func(name string) {
			update := func(vmi *kvv1.VirtualMachineInstance) *kvv1.VirtualMachineInstance {
				vmi.SetLabels(addExcludeLabel(vmi.GetLabels()))
				return vmi
			}
			retryOnceOnErr(updateVmi(kvClient, namespace.Name, name, update)).Should(BeNil())
		}

		addExcludeLabelToVM := func(name string) {
			update := func(vm *kvv1.VirtualMachine) *kvv1.VirtualMachine {
				vm.SetLabels(addExcludeLabel(vm.GetLabels()))
				return vm
			}
			retryOnceOnErr(updateVm(kvClient, namespace.Name, name, update)).Should(BeNil())
		}

		addExcludeLabelToLauncherPodForVM := func(vmName string) {
			retryOnceOnErr(
				func() error {
					pod := FindLauncherPod(client, namespace.Name, vmName)
					pod.SetLabels(addExcludeLabel(pod.GetLabels()))
					_, err := client.CoreV1().Pods(namespace.Name).Update(context.TODO(), &pod, metav1.UpdateOptions{})
					return err
				}).Should(BeNil())
		}

		Context("Standalone DV", func() {

			It("DV included, PVC excluded: PVC should not be re-imported", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Add exlude label to PVC")
				addExcludeLabelToPVC("test-dv")

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "persistentvolumeclaims", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not reimport")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("PVC included, DV excluded: PVC should not be restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Add exclude label to DV")
				addExcludeLabelToDV("test-dv")

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "persistentvolumeclaims", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC does not exists")
				_, err = framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(err).To(HaveOccurred())

				By("Checking DataVolume does not exists")
				_, err = framework.FindDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).To(HaveOccurred())

			})
		})

		Context("VM with DVTemplates", func() {
			It("VM included, VMI excluded: should fail if VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to VMI")
				addExcludeLabelToVMI("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM+VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM+VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Negative: VM+VMI+Pod included should fail if VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM included, DV and PVC excluded: both DV and PVC recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", r.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude labels")
				addExcludeLabelToDV(vmSpec.Spec.DataVolumeTemplates[0].Name)
				addExcludeLabelToPVC(vmSpec.Spec.DataVolumeTemplates[0].Name)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM+PVC included, DV excluded: VM and PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, writerPod(volumeName))
				deletePod(kvClient, namespace.Name, writerPod.Name)

				By("Adding exclude label to DV")
				addExcludeLabelToDV(vmSpec.Spec.DataVolumeTemplates[0].Name)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying PVC is not re-imported - file exists")
				readerPod := runPodAndWaitSucceeded(kvClient, namespace.Name, verifyFileExists(volumeName))
				deletePod(kvClient, namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI included, VM excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to VM")
				addExcludeLabelToVM("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {
			It("VM with DV Volume, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label")
				addExcludeLabelToDV("test-dv")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[negative] VM with DV Volume, DV included, PVC excluded: VM+DV recreated, PVC not recreated", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude labels")
				addExcludeLabelToPVC("test-dv")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending and no PVC")
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Pending, source.DataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.FindPVC(client, namespace.Name, "test-dv")
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusProvisioning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			//TODO: verify if that is what we actualy want
			It("VM with PVC Volume, PVC excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					PersistentVolumeClaim: &kvv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test-dv",
						}},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude labels")
				addExcludeLabelToPVC("test-dv")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("[smoke] Standalone VMI", func() {
			It("VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vmi")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vmiSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pause VMI")
				err = framework.PauseVirtualMachine(kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vmi")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI running")
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, "test-vmi", kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
			})

			It("Pod included, VMI excluded: backup should succeed, only DV and PVC restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vmiSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Adding exclude label to VMI")
				addExcludeLabelToVMI("test-vmi")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = framework.WaitForDataVolumePhaseButNot(kvClient, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI not present")
				_, err = framework.GetVirtualMachineInstance(kvClient, namespace.Name, "test-vmi")
				Expect(err).To(HaveOccurred())

				By("Cleanup")
				err = framework.DeleteDataVolume(kvClient, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI+Pod included, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to DV")
				addExcludeLabelToDV("test-dv")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

func FindLauncherPod(client *kubernetes.Clientset, namespace string, vmName string) v1.Pod {
	var pod v1.Pod
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "kubevirt.io=virt-launcher",
	})
	Expect(err).WithOffset(1).ToNot(HaveOccurred())
	for _, item := range pods.Items {
		if ann, ok := item.GetAnnotations()["kubevirt.io/domain"]; ok && ann == vmName {
			pod = item
		}
	}
	Expect(pod).WithOffset(1).ToNot(BeNil())
	return pod
}

func updateVm(kvClient kubecli.KubevirtClient, namespace string, name string,
	update func(*kvv1.VirtualMachine) *kvv1.VirtualMachine) func() error {
	return func() error {
		vm, err := kvClient.VirtualMachine(namespace).Get(name, &metav1.GetOptions{})
		if err != nil {
			return err
		}
		vm = update(vm)

		_, err = kvClient.VirtualMachine(namespace).Update(vm)
		return err
	}
}

func updateVmi(kvClient kubecli.KubevirtClient, namespace string, name string,
	update func(*kvv1.VirtualMachineInstance) *kvv1.VirtualMachineInstance) func() error {
	return func() error {
		vmi, err := kvClient.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
		if err != nil {
			return err
		}
		vmi = update(vmi)

		_, err = kvClient.VirtualMachineInstance(namespace).Update(vmi)
		return err
	}
}

func updatePvc(client *kubernetes.Clientset, namespace string, name string,
	update func(*v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim) func() error {
	return func() error {
		pvc, err := framework.FindPVC(client, namespace, name)
		if err != nil {
			return err
		}
		pvc = update(pvc)

		_, err = client.CoreV1().PersistentVolumeClaims(namespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
		return err
	}
}
func updateDataVolume(kvClient kubecli.KubevirtClient, namespace string, name string,
	update func(dataVolume *cdiv1.DataVolume) *cdiv1.DataVolume) func() error {
	return func() error {
		dv, err := framework.FindDataVolume(kvClient, namespace, name)
		if err != nil {
			return err
		}
		dv = update(dv)

		_, err = kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Update(context.TODO(), dv, metav1.UpdateOptions{})
		return err
	}
}

func retryOnceOnErr(f func() error) Assertion {
	err := f()
	if err != nil {
		err = f()
	}

	return Expect(err)
}

func runPodAndWaitSucceeded(kvClient kubecli.KubevirtClient, namespace string, podSpec *v1.Pod) *v1.Pod {
	By("creating a pod that writes to pvc")
	pod, err := kvClient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	Expect(err).WithOffset(1).ToNot(HaveOccurred())

	By("Wait for pod to reach a completed phase")
	Eventually(func() error {
		updatedPod, err := kvClient.CoreV1().Pods(namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if updatedPod.Status.Phase != v1.PodSucceeded {
			return fmt.Errorf("Pod in phase %s, expected Succeeded", updatedPod.Status.Phase)
		}
		return nil
	}, 3*time.Minute, 5*time.Second).WithOffset(1).Should(Succeed(), "pod should reach Succeeded state")

	return pod
}

func deletePod(kvClient kubecli.KubevirtClient, namespace, podName string) {
	By("Delete pod")
	zero := int64(0)
	err := kvClient.CoreV1().Pods(namespace).Delete(context.Background(), podName,
		metav1.DeleteOptions{
			GracePeriodSeconds: &zero,
		})
	Expect(err).WithOffset(1).ToNot(HaveOccurred())

	By("verify deleted")
	Eventually(func() error {
		_, err := kvClient.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		return err
	}, 3*time.Minute, 5*time.Second).
		WithOffset(1).
		Should(Satisfy(apierrs.IsNotFound), "pod should disappear")
}

func verifyFileExists(volumeName string) *v1.Pod {
	return PodWithPvcSpec("reader-pod",
		volumeName,
		[]string{"sh"},
		[]string{"-c", "test -f /pvc/test.txt"})
}

func verifyNoFile(volumeName string) *v1.Pod {
	return PodWithPvcSpec("reader-pod",
		volumeName,
		[]string{"sh"},
		[]string{"-c", "! test -e /pvc/test.txt"})
}

func writerPod(volumeName string) *v1.Pod {
	return PodWithPvcSpec(
		"writer-pod",
		volumeName,
		[]string{"sh"},
		[]string{"-c", "echo testing > /pvc/test.txt && sleep 1s"})
}
