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
	. "kubevirt.io/kubevirt-velero-plugin/tests/framework/matcher"
)

const (
	includedDVName = "included-test-dv"
	otherDVName    = "other-test-dv"
	includedVMName = "included-test-vm"
)

var _ = Describe("Resource includes", func() {
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string

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

		cancelFunc()
	})

	Context("Include namespace", func() {
		var includedNamespace *v1.Namespace
		var otherNamespace *v1.Namespace

		BeforeEach(func() {
			var err error
			includedNamespace, err = f.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(includedNamespace)
			otherNamespace, err = f.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(otherNamespace)
		})

		It("[test_id:9798]Should only backup and restore DV from included namespace", func() {
			By("Creating DVs")
			dvSpec := framework.NewDataVolumeForBlankRawImage(includedDVName, "100Mi", f.StorageClass)
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, includedNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			framework.EventuallyDVWith(f.KvClient, includedNamespace.Name, includedDVName, 180, HaveSucceeded())

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvSpec = framework.NewDataVolumeForBlankRawImage(otherDVName, "100Mi", f.StorageClass)
			dvOther, err := framework.CreateDataVolumeFromDefinition(f.KvClient, otherNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())
			framework.EventuallyDVWith(f.KvClient, otherNamespace.Name, otherDVName, 180, HaveSucceeded())

			By("Crating backup test-backup")
			err = framework.CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting DVs")
			err = framework.DeleteDataVolume(f.KvClient, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitDataVolumeDeleted(f.KvClient, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteDataVolume(f.KvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitDataVolumeDeleted(f.KvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking included DataVolume exists")
			framework.EventuallyDVWith(f.KvClient, includedNamespace.Name, includedDVName, 180, HaveSucceeded())

			By("Checking not included DataVolume does not exist")
			ok, err = framework.WaitDataVolumeDeleted(f.KvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("[test_id:9799]Should only backup and restore VM from included namespace", func() {
			By("Creating VirtualMachines")
			vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
			vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, includedNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			framework.EventuallyDVWith(f.KvClient, includedNamespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

			vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
			vmOther, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, otherNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			framework.EventuallyDVWith(f.KvClient, otherNamespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

			By("Creating backup")
			err = framework.CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VMs")
			err = framework.DeleteVirtualMachine(f.KvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteVirtualMachine(f.KvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying included VM exists")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, includedNamespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying ignored VM does not exists")
			ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	Context("Include resources", func() {

		Context("Standalone DV", func() {
			It("[test_id:9800]Selecting DV+PVC: Both DVs and PVCs should be backed up and restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup test-backup")
				err = framework.CreateBackupForResources(timeout, backupName, "datavolumes,persistentvolumeclaims", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following 5 items:
				// - DataVolume
				// - PVC
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - VolumeSnapshotClass
				expectedItems := 5
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedItems -= 1
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(expectedItems))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				_, err = framework.WaitForPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume exists and succeeds")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())
			})

			It("[test_id:9801]Selecting DV+PVC+PV+VolumeSnapshot+VSContent: Both DVs and PVCs should be backed up and restored, content of PVC not re-imported", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup test-backup")
				resources := "datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following items:
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - a number of unbound PVs
				expectedItems := 5
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedItems = 4
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(BeNumerically(">=", expectedItems))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = framework.WaitForPVCPhase(f.K8sClient, f.Namespace.Name, dvName, v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("[test_id:9802]Selecting only DVs: the restored DV should not recreate its PVC", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup test-backup")
				err = framework.CreateBackupForResources(timeout, backupName, "datavolumes", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following item:
				// - DataVolume
				expectedItems := 1
				isDVGC := framework.IsDataVolumeGC(f.KvClient)
				if isDVGC {
					expectedItems = 0
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(expectedItems))

				if !isDVGC {
					By("Deleting DVs")
					err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
					Expect(err).ToNot(HaveOccurred())
					_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
					Expect(err).ToNot(HaveOccurred())
					_, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
					Expect(err).ToNot(HaveOccurred())

					By("Creating restore test-restore")
					err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
					Expect(err).ToNot(HaveOccurred())
					err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
					Expect(err).ToNot(HaveOccurred())

					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Pending, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("[test_id:9803]Selecting only PVCs: PVC should be restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup test-backup")
				resources := "persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
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
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = framework.WaitForPVCPhase(f.K8sClient, f.Namespace.Name, dvName, v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())
				pvc, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pvc.OwnerReferences)).To(Equal(0))

				By("Checking DataVolume does not exist")
				Consistently(func() bool {
					_, err := framework.FindDataVolume(f.KvClient, f.Namespace.Name, dvName)
					return apierrs.IsNotFound(err)
				}, "1000ms", "100ms").Should(BeTrue())
			})
		})

		Context("VM with DVTemplates", func() {
			It("[test_id:9804]Selecting VM+DV+PVC: VM, DV and PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent(includedVMName, f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, volumeName, 180, HaveSucceeded())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, writerPod(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, "included-test-vm-dv")
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "included-test-vm-dv")
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying DataVolume does not re-import content - file should exists")
				readerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, verifyFileExists(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10188]Selecting VM+DV+PVC: Backing up VM should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10189]Selecting VM+VMI but not Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances,persistentvolumeclaims"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10190]Selecting VM+VMI but not Pod+PVC: Backup should succeed, DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, BeInPhase(cdiv1.ImportScheduled))

				By("Checking DataVolume import succeeds")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10191]Selecting VM+VMI but not Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, volumeName, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, writerPod(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, writerPod.Name)

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Stopping the VM")
				err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				// Testing for ImportScheduled is not reliable, because sometimes it might happen so fast,
				// that DV switches to Succeeded before we even get here
				By("Checking DataVolume import succeeds")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, volumeName, 180, HaveSucceeded())

				By("Verifying DataVolume is re-imported - file should not exists")
				readerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, verifyNoFile(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, readerPod.Name)
			})

			It("[test_id:10192]Selecting VM but not VMI or Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10193]Selecting VM but not VMI and Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				err = framework.PauseVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, "test-vm-dv")
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-vm-dv")
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10194]Selecting VM+DV+PVC+VMI+Pod: All objects should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10195]Selecting VM+DV: VM, DV should be restored, PVC should not be recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Creating backup")
				resources := "virtualmachines,datavolumes"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, BeInPhase(cdiv1.Pending))

				expectedStatus := kvv1.VirtualMachineStatusProvisioning
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedStatus = kvv1.VirtualMachineStatusStopped
				}
				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, expectedStatus)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10196]Selecting VM+PVC: VM and PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, writerPod(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				// DV may not exist, so just check the PVC
				By("Verifying PVC is NOT re-imported - file exists")
				readerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, verifyFileExists(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10197]Selecting VM but not DV and PVC: VM should be restored, DV and PVC should be recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, writerPod(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "virtualmachines"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Verifying DataVolume is re-imported - file should not exists")
				readerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, verifyNoFile(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10198]Selecting VMI (with DV+PVC+Pod) but not VM: Backing up VMI should fail", func() {
				By("Creating VirtualMachine")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting VirtualMachine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup with DV+PVC+Pod")
				resources := "datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10199]Selecting VMI (without DV+PVC+Pod) but not VM: Backing up VMI should fail", func() {
				By("Creating VirtualMachine")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting VirtualMachine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup without DV+PVC+Pod")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {

			It("[test_id:10200]Selecting VM+DV+PVC, VM stopped: VM, DV and PVC should be restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10201]Selecting VM + PVC, VM stopped: VM and PVC should be restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					PersistentVolumeClaim: &kvv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
							ClaimName: dvName,
						}},
				}
				vmSpec := newVMSpec(includedVMName, "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, "tet-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10202]Selecting VM + PVC, VM running: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec(includedVMName, "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10203]Selecting VM but not PVC: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec(includedVMName, "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("[smoke] Standalone VMI", func() {
			// This test tries to backup on all namespaces, on some clusters it always fails
			// need to be improved
			It("[test_id:10204]Selecting standalone VMI+DV+PVC+Pod: All objects should be restored", func() {
				By(fmt.Sprintf("Creating DataVolume %s", dvName))
				err := f.CreateDataVolumeWithGuestAgentImage()
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiName := "test-vmi-with-dv"
				err = f.CreateVMIWithDataVolume()
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiName, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vmiName, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(f.KvClient, f.Namespace.Name, vmiName)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying VMI running")
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiName, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10205]Selecting standalone VMI+Pod without DV: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10206]Selecting standalone VMI+Pod without PVC: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10207]Selecting standalone VMI without Pod: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForResources(timeout, backupName, resources, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Selector includes", func() {

		Context("Standalone DV", func() {
			It("[test_id:10208][no-gc] Should only backup and restore DV selected by label", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(includedDVName, "100Mi", f.StorageClass)
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, includedDVName, 180, HaveSucceeded())

				dvSpec = framework.NewDataVolumeForBlankRawImage(otherDVName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvOther, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, otherDVName, 180, HaveSucceeded())

				By("Crating backup test-backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=include", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking included DataVolume exists")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvIncluded.Name, 180, HaveSucceeded())

				By("Checking not included DataVolume does not exist")
				ok, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			It("[test_id:10209]Backup of DVs selected by label should include PVCs", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(includedDVName, "100Mi", f.StorageClass)
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, includedDVName, 180, HaveSucceeded())

				By("Crating backup test-backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=include", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contains the following 8 items:
				// - DataVolume CRD
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - VolumeSpapshotClass
				expectedItems := 7
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedItems -= 1
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(expectedItems))

				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvSpec.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM with DVTemplates and standalone DVs", func() {
			It("[test_id:9679]Backup of a stopped VM selected by label should include its DVs and PVCs", func() {
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				dvSpec.Annotations[forceBindAnnotation] = "true"

				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvSpec.Name, 180, HaveSucceeded())
				// creating a started VM, so it works correctly also on WFFC storage
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
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

				vm, err := framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Stopping a VM")
				err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contain the following 12 items:
				// - VirtualMachine
				// - 2 DataVolume
				// - 2 PVC
				// - 2 PV
				// - 2 VolumeSnapshot
				// - 2 VolumeSnapshotContent
				// - VolumeSpapshotClass
				// - DataVolume CRD
				expectedItems := 13
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedItems -= 2
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(expectedItems))
			})
		})
		Context("VM with DVTemplates", func() {
			It("[test_id:10210]Backup of a stopped VMs selected by label should include its DVs and PVCs", func() {
				By("Creating VirtualMachines")

				vmSpec := framework.CreateVmWithGuestAgent(includedVMName, f.StorageClass)
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contains the following 7 items:
				// - VirtualMachine
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - VolumeSpapshotClass
				// - DataVolume CRD
				expectedItems := 8
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedItems -= 1
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(expectedItems))
			})

			It("[test_id:10211]Backup of a running VMs selected by label should include its DVs and PVCs, VMIs and Pods", func() {
				By("Creating VirtualMachines")

				vmSpec := framework.CreateVmWithGuestAgent(includedVMName, f.StorageClass)
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting VM")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				// The backup should contains the following 9 items:
				// - VirtualMachine
				// - VirtualMachineInstance
				// - Launcher pod
				// - DataVolume
				// - PVC
				// - PV
				// - VolumeSnapshot
				// - VolumeSnapshotContent
				// - VolumeSpapshotClass
				// - DataVolume CRD
				expectedItems := 10
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedItems -= 1
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(expectedItems))
			})
		})

		Context("[smoke] Standalone VMI", func() {
			It("[test_id:10212]Backup of VMIs selected by label should include its DVs, PVCs, and Pods", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				dvSpec2 := framework.NewDataVolumeForBlankRawImage("test-dv-2", "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec2.Name))
				_, err = framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec2)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, "test-dv-2", 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
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
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
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
				// - DataVolume CRD
				expectedItems := 13
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedItems -= 1
				}
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(expectedItems))
			})
		})
	})
})

var _ = Describe("Resource excludes", func() {
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string

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

		cancelFunc()
	})

	Context("Exclude namespace", func() {
		var excludedNamespace *v1.Namespace
		var otherNamespace *v1.Namespace

		BeforeEach(func() {
			var err error
			excludedNamespace, err = f.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(excludedNamespace)
			otherNamespace, err = f.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(otherNamespace)
		})

		It("[test_id:10213]Should not backup and restore DV from excluded namespace", func() {
			By("Creating DVs")
			dvSpec := framework.NewDataVolumeForBlankRawImage("excluded-test-dv", "100Mi", f.StorageClass)
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvExcluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, excludedNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			framework.EventuallyDVWith(f.KvClient, excludedNamespace.Name, "excluded-test-dv", 180, HaveSucceeded())

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvSpec = framework.NewDataVolumeForBlankRawImage(otherDVName, "100Mi", f.StorageClass)
			dvOther, err := framework.CreateDataVolumeFromDefinition(f.KvClient, otherNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())
			framework.EventuallyDVWith(f.KvClient, otherNamespace.Name, otherDVName, 180, HaveSucceeded())

			By("Crating backup test-backup")
			err = framework.CreateBackupForNamespaceExcludeNamespace(timeout, backupName, otherNamespace.Name, excludedNamespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting DVs")
			err = framework.DeleteDataVolume(f.KvClient, excludedNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitDataVolumeDeleted(f.KvClient, excludedNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteDataVolume(f.KvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitDataVolumeDeleted(f.KvClient, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking included DataVolume exists")
			framework.EventuallyDVWith(f.KvClient, otherNamespace.Name, otherDVName, 180, HaveSucceeded())

			By("Checking not included DataVolume does not exist")
			ok, err = framework.WaitDataVolumeDeleted(f.KvClient, excludedNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("[test_id:10214]Should not backup and restore VM from excluded namespace", func() {
			By("Creating VirtualMachines")
			//vmSpec := newVMSpecBlankDVTemplate("excluded-test-vm", "100Mi")
			vmSpec := framework.CreateVmWithGuestAgent("excluded-test-vm", f.StorageClass)
			vmExcluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, excludedNamespace.Name, vmSpec)

			Expect(err).ToNot(HaveOccurred())
			framework.EventuallyDVWith(f.KvClient, excludedNamespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

			//vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
			vmSpec = framework.CreateVmWithGuestAgent("other-test-vm", f.StorageClass)
			vmOther, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, otherNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			framework.EventuallyDVWith(f.KvClient, otherNamespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

			By("Creating backup")
			err = framework.CreateBackupForNamespaceExcludeNamespace(timeout, backupName, otherNamespace.Name, excludedNamespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VMs")
			err = framework.DeleteVirtualMachine(f.KvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteVirtualMachine(f.KvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying included VM exists")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, otherNamespace.Name, vmOther.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying ignored VM does not exists")
			ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	Context("Exclude resources", func() {
		Context("Standalone DV", func() {
			It("[test_id:10215][negative][no-gc] PVC excluded: DV restored, PVC will not be re-imported", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, "persistentvolumeclaims", snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))

				By("Deleting DVs")
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, BeInPhase(cdiv1.Pending))
				_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(apierrs.IsNotFound(err)).To(BeTrue())
			})

			It("[test_id:10216]DV excluded: PVC restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup test-backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, "datavolumes", snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = framework.WaitForPVCPhase(f.K8sClient, f.Namespace.Name, dvName, v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())
				pvc, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pvc.OwnerReferences)).To(Equal(0))

				By("Checking DataVolume does not exist")
				Consistently(func() bool {
					_, err := framework.FindDataVolume(f.KvClient, f.Namespace.Name, dvName)
					return apierrs.IsNotFound(err)
				}, "1000ms", "100ms").Should(BeTrue())
			})
		})

		Context("VM with DVTemplates", func() {
			It("[test_id:10217]Pods excluded, VM running: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, "pods", snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10218]Pods+DV excluded, VM running: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods,datavolumes"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10219][negative][no-gc] Pods+PVC excluded, VM running: VM+DV restored, PVC not re-imported", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					// gc case in test:
					// [gc-only] Pods+PVC excluded, VM running, DV GC: VM restored, DV and PVC recreated
					Skip("Test worth testing only without GC")
				}
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods,persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, BeInPhase(cdiv1.Pending))
				_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusProvisioning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10220][gc-only] Pods+PVC excluded, VM running, DV GC: VM restored, DV and PVC recreated", func() {
				if !framework.IsDataVolumeGC(f.KvClient) {
					// no gc case in test:
					// [negative][no-gc] Pods+PVC excluded, VM running: VM+DV restored, PVC not re-imported
					Skip("Test worth testing only without GC")
				}
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods,persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VM")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, BeInPhase(cdiv1.ImportScheduled))

				By("Checking DataVolume import succeeds")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10221]Pods excluded, VM stopped: VM+DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Creating backup")
				resources := "pods"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10222]Pods excluded, VM paused: VM+DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10223]VMI excluded, Pod not excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10224]PVC excluded: DV restored, PVC not re-imported", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not reimport")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, BeInPhase(cdiv1.Pending))
				_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				expectedStatus := kvv1.VirtualMachineStatusProvisioning
				if framework.IsDataVolumeGC(f.KvClient) {
					expectedStatus = kvv1.VirtualMachineStatusStopped
				}
				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, expectedStatus)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10225]DV+PVC excluded: VM restored, DV+PVC recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Creating backup")
				resources := "datavolume,persistentvolumeclaim"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, BeInPhase(cdiv1.ImportScheduled))

				By("Checking DataVolume import succeeds")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10226]DV excluded: VM+PVC restored, DV recreated and bound to the PVC", func() {
				By("Creating VirtualMachines")
				//vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, writerPod(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, writerPod.Name)

				By("Creating backup")
				resources := "datavolume"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying DataVolume does not re-import content - file should exists")
				readerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, verifyFileExists(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10227]Running VM excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachine"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10228]Stopped VM excluded: DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate(includedVMName, "100Mi")
				vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Creating backup")
				resources := "virtualmachine"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Delete VM")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM does not exists")
				_, err = framework.GetVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {
			It("[test_id:10229][no-gc] VM with DV Volume, DV excluded: backup should fail", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec(includedVMName, "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup")
				resources := "datavolumes"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10230][no-gc] VM with DV Volume, DV included, PVC excluded: VM+DV recreated, PVC not recreated and re-imported", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not reimport")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, source.DataVolume.Name, 180, BeInPhase(cdiv1.Pending))
				_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusProvisioning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10231][gc-only] VM with DV Volume, DV GC, PVC excluded: backup should fail", func() {
				if !framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only with GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10233]VM with PVC Volume, PVC excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					PersistentVolumeClaim: &kvv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
							ClaimName: dvName,
						}},
				}
				vmSpec := newVMSpec(includedVMName, "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Standalone VMI", func() {
			It("[test_id:10234]VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10235]VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Pause VMI")
				err = framework.PauseVirtualMachine(f.KvClient, f.Namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pod"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(f.KvClient, f.Namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying VMI running")
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, "test-vmi", kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10236][smoke] Pod included, VMI excluded: backup should succeed, only DV and PVC restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				vm, err := framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(f.KvClient, f.Namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying VMI not present")
				_, err = framework.GetVirtualMachineInstance(f.KvClient, f.Namespace.Name, "test-vmi")
				Expect(err).To(HaveOccurred())
			})

			It("[test_id:10237][no-gc] VMI+Pod included, DV excluded: backup should fail", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolumes"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10238][gc-only] VMI+Pod included, DV GC, PVC excluded: backup should fail", func() {
				if !framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only with GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, resources, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
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

			retryOnceOnErr(updateDataVolume(f.KvClient, f.Namespace.Name, name, updateFunc)).Should(BeNil())
		}

		addExcludeLabelToPVC := func(name string) {
			update := func(pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
				pvc.SetLabels(addExcludeLabel(pvc.GetLabels()))
				return pvc
			}
			retryOnceOnErr(updatePvc(f.K8sClient, f.Namespace.Name, name, update)).Should(BeNil())
		}

		addExcludeLabelToVMI := func(name string) {
			update := func(vmi *kvv1.VirtualMachineInstance) *kvv1.VirtualMachineInstance {
				vmi.SetLabels(addExcludeLabel(vmi.GetLabels()))
				return vmi
			}
			retryOnceOnErr(updateVmi(f.KvClient, f.Namespace.Name, name, update)).Should(BeNil())
		}

		addExcludeLabelToVM := func(name string) {
			update := func(vm *kvv1.VirtualMachine) *kvv1.VirtualMachine {
				vm.SetLabels(addExcludeLabel(vm.GetLabels()))
				return vm
			}
			retryOnceOnErr(updateVm(f.KvClient, f.Namespace.Name, name, update)).Should(BeNil())
		}

		addExcludeLabelToLauncherPodForVM := func(vmName string) {
			retryOnceOnErr(
				func() error {
					pod := FindLauncherPod(f.K8sClient, f.Namespace.Name, vmName)
					pod.SetLabels(addExcludeLabel(pod.GetLabels()))
					_, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Update(context.TODO(), &pod, metav1.UpdateOptions{})
					return err
				}).Should(BeNil())
		}

		Context("Standalone DV", func() {

			It("[test_id:10239][no-gc] DV included, PVC excluded: PVC should not be re-imported", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Add exlude label to PVC")
				addExcludeLabelToPVC(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, "persistentvolumeclaims", snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not reimport")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, BeInPhase(cdiv1.Pending))
				_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(apierrs.IsNotFound(err)).To(BeTrue())
			})

			It("[test_id:10247][no-gc] PVC included, DV excluded: PVC should not be restored, ownership relation empty", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Add exclude label to DV")
				addExcludeLabelToDV(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespaceExcludeResources(timeout, backupName, f.Namespace.Name, "persistentvolumeclaims", snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC does not exists")
				_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(err).To(HaveOccurred())

				By("Checking DataVolume does not exists")
				_, err = framework.FindDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).To(HaveOccurred())

			})
		})

		Context("VM with DVTemplates", func() {
			It("[test_id:10248]VM included, VMI excluded: should fail if VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to VMI")
				addExcludeLabelToVMI("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10249]VM+VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10250]VM+VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10251]Negative: VM+VMI+Pod included should fail if VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = framework.PauseVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10252]VM included, DV and PVC excluded: both DV and PVC recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Adding exclude labels")
				if !framework.IsDataVolumeGC(f.KvClient) {
					addExcludeLabelToDV(vmSpec.Spec.DataVolumeTemplates[0].Name)
				}
				addExcludeLabelToPVC(vmSpec.Spec.DataVolumeTemplates[0].Name)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, BeInPhase(cdiv1.ImportScheduled))

				By("Checking DataVolume import succeeds")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10253]VM+PVC included, DV excluded(or GC): VM and PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())
				volumeName := vmSpec.Spec.DataVolumeTemplates[0].Name

				By("Writing to PVC filesystem")
				writerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, writerPod(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, writerPod.Name)

				By("Adding exclude label to DV")
				if !framework.IsDataVolumeGC(f.KvClient) {
					addExcludeLabelToDV(vmSpec.Spec.DataVolumeTemplates[0].Name)
				}

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying PVC is not re-imported - file exists")
				readerPod := runPodAndWaitSucceeded(f.KvClient, f.Namespace.Name, verifyFileExists(volumeName))
				deletePod(f.KvClient, f.Namespace.Name, readerPod.Name)

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10254]VMI included, VM excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				_, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())

				By("Starting the virtual machine")
				err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to VM")
				addExcludeLabelToVM("test-vm")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {
			It("[test_id:10255][no-gc] VM with DV Volume, DV excluded: backup should fail", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Adding exclude label")
				addExcludeLabelToDV(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10256][gc-only] VM with DV Volume, DV GC, PVC excluded: backup should fail", func() {
				if !framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only with GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())
				_, err = framework.WaitOnlyDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label")
				addExcludeLabelToPVC(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10260][negative][no-gc] VM with DV Volume, DV included, PVC excluded: VM+DV recreated, PVC not recreated", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dvName,
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude labels")
				addExcludeLabelToPVC(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				_, err = framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeletePVC(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume Pending and no PVC")
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, source.DataVolume.Name, 180, BeInPhase(cdiv1.Pending))
				_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, dvName)
				Expect(apierrs.IsNotFound(err)).To(BeTrue())

				By("Verifying included VM exists")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusProvisioning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10261]VM with PVC Volume, PVC excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					PersistentVolumeClaim: &kvv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
							ClaimName: dvName,
						}},
				}
				vmSpec := newVMSpec(includedVMName, "100Mi", source)
				_, err = framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Adding exclude labels")
				addExcludeLabelToPVC(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("[smoke] Standalone VMI", func() {
			It("[test_id:10262]VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vmi")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10263]VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pause VMI")
				err = framework.PauseVirtualMachine(f.KvClient, f.Namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vmi")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(f.KvClient, f.Namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying VMI running")
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, "test-vmi", kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10264]Pod included, VMI excluded: backup should succeed, only DV and PVC restored", func() {
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForVmWithGuestAgentImage(dvName, f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := framework.WaitForVirtualMachineInstanceCondition(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Adding exclude label to VMI")
				addExcludeLabelToVMI("test-vmi")

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = framework.DeleteVirtualMachineInstance(f.KvClient, f.Namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				ok, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvName)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				if framework.IsDataVolumeGC(f.KvClient) {
					By("Checking DataVolume does not exist")
					deleted, err := framework.DataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
					Expect(err).ToNot(HaveOccurred())
					Expect(deleted).To(BeTrue())
				} else {
					By("Checking DataVolume does not re-import content")
					err = framework.WaitForDataVolumePhaseButNot(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, dvName)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Verifying VMI not present")
				_, err = framework.GetVirtualMachineInstance(f.KvClient, f.Namespace.Name, "test-vmi")
				Expect(err).To(HaveOccurred())
			})

			It("[test_id:10265][no-gc] VMI+Pod included, DV excluded: backup should fail", func() {
				if framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only without GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to DV")
				addExcludeLabelToDV(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:10266][gc-only] VMI+Pod included, DV GC, PVC excluded: backup should fail", func() {
				if !framework.IsDataVolumeGC(f.KvClient) {
					Skip("Test worth testing only with GC")
				}
				By("Creating DVs")
				dvSpec := framework.NewDataVolumeForBlankRawImage(dvName, "100Mi", f.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", dvName)
				_, err = framework.CreateVirtualMachineInstanceFromDefinition(f.KvClient, f.Namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForVirtualMachineInstancePhase(f.KvClient, f.Namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to DV")
				addExcludeLabelToPVC(dvName)

				By("Creating backup")
				err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = framework.WaitForBackupPhase(timeout, backupName, f.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
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
		vm, err := kvClient.VirtualMachine(namespace).Get(context.Background(), name, &metav1.GetOptions{})
		if err != nil {
			return err
		}
		vm = update(vm)

		_, err = kvClient.VirtualMachine(namespace).Update(context.Background(), vm)
		return err
	}
}

func updateVmi(kvClient kubecli.KubevirtClient, namespace string, name string,
	update func(*kvv1.VirtualMachineInstance) *kvv1.VirtualMachineInstance) func() error {
	return func() error {
		vmi, err := kvClient.VirtualMachineInstance(namespace).Get(context.Background(), name, &metav1.GetOptions{})
		if err != nil {
			return err
		}
		vmi = update(vmi)

		_, err = kvClient.VirtualMachineInstance(namespace).Update(context.Background(), vmi)
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
	By("creating a pod")
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
