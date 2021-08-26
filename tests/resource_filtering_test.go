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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvv1 "kubevirt.io/client-go/api/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

const (
	backupName  = "test-backup"
	restoreName = "test-restore"
)

var newVMSpecDVTemplate = func(vmName, size string) *kvv1.VirtualMachine {
	no := false
	var zero int64 = 0
	return &kvv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmName,
		},
		Spec: kvv1.VirtualMachineSpec{
			Running: &no,
			Template: &kvv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: vmName,
				},
				Spec: kvv1.VirtualMachineInstanceSpec{
					Domain: kvv1.DomainSpec{
						Resources: kvv1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceMemory): resource.MustParse(size),
							},
						},
						Machine: &kvv1.Machine{
							Type: "",
						},
						Devices: kvv1.Devices{
							Disks: []kvv1.Disk{
								{
									Name: "volume0",
									DiskDevice: kvv1.DiskDevice{
										Disk: &kvv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
						},
					},
					Volumes: []kvv1.Volume{
						{
							Name: "volume0",
							VolumeSource: kvv1.VolumeSource{
								DataVolume: &kvv1.DataVolumeSource{
									Name: vmName + "-dv",
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &zero,
				},
			},
			DataVolumeTemplates: []kvv1.DataVolumeTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: vmName + "-dv",
					},
					Spec: cdiv1.DataVolumeSpec{
						Source: cdiv1.DataVolumeSource{
							Blank: &cdiv1.DataVolumeBlankImage{},
						},
						PVC: &v1.PersistentVolumeClaimSpec{
							AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceName(v1.ResourceStorage): resource.MustParse(size),
								},
							},
						},
					},
				},
			},
		},
	}
}

var newVMSpec = func(vmName, size string, volumeSource kvv1.VolumeSource) *kvv1.VirtualMachine {
	no := false
	var zero int64 = 0
	return &kvv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmName,
		},
		Spec: kvv1.VirtualMachineSpec{
			Running: &no,
			Template: &kvv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: vmName,
				},
				Spec: kvv1.VirtualMachineInstanceSpec{
					Domain: kvv1.DomainSpec{
						Resources: kvv1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceMemory): resource.MustParse(size),
							},
						},
						Machine: &kvv1.Machine{
							Type: "",
						},
						Devices: kvv1.Devices{
							Disks: []kvv1.Disk{
								{
									Name: "volume0",
									DiskDevice: kvv1.DiskDevice{
										Disk: &kvv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
						},
					},
					Volumes: []kvv1.Volume{
						{
							Name:         "volume0",
							VolumeSource: volumeSource,
						},
					},
					TerminationGracePeriodSeconds: &zero,
				},
			},
		},
	}
}

func addVolumeToVMI(vmi *kvv1.VirtualMachineInstance, source kvv1.VolumeSource, volumeName string) *kvv1.VirtualMachineInstance {
	vmi.Spec.Domain.Devices.Disks = append(vmi.Spec.Domain.Devices.Disks, kvv1.Disk{
		Name: volumeName,
		DiskDevice: kvv1.DiskDevice{
			Disk: &kvv1.DiskTarget{
				Bus: "virtio",
			},
		},
	})
	vmi.Spec.Volumes = append(vmi.Spec.Volumes, kvv1.Volume{
		Name:         volumeName,
		VolumeSource: source,
	})
	return vmi
}

func newVMISpec(vmiName, size string) *kvv1.VirtualMachineInstance {
	var zero int64 = 0

	vmi := &kvv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmiName,
		},
		Spec: kvv1.VirtualMachineInstanceSpec{
			Domain: kvv1.DomainSpec{
				Resources: kvv1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceMemory): resource.MustParse(size),
					},
				},
				Machine: &kvv1.Machine{
					Type: "",
				},
				Devices: kvv1.Devices{
					Disks: []kvv1.Disk{},
				},
			},
			Volumes:                       []kvv1.Volume{},
			TerminationGracePeriodSeconds: &zero,
		},
	}
	return vmi
}

func newVMISpecWithDV(vmiName, size, dvName string) *kvv1.VirtualMachineInstance {
	vmi := newVMISpec(vmiName, size)

	source := kvv1.VolumeSource{
		DataVolume: &kvv1.DataVolumeSource{
			Name: dvName,
		},
	}
	vmi = addVolumeToVMI(vmi, source, "volume0")
	return vmi
}

func newVMISpecWithPVC(vmiName, size, pvcName string) *kvv1.VirtualMachineInstance {
	vmi := newVMISpec(vmiName, size)

	source := kvv1.VolumeSource{
		PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcName,
		},
	}
	vmi = addVolumeToVMI(vmi, source, "volume0")
	return vmi
}

var _ = Describe("Resource includes", func() {
	var client, _ = util.GetK8sClient()
	var timeout context.Context
	var cancelFunc context.CancelFunc

	BeforeEach(func() {
		timeout, cancelFunc = context.WithTimeout(context.Background(), 5*time.Minute)
	})

	AfterEach(func() {
		// Deleting the backup also deletes all restores, volume snapshots etc.
		err := DeleteBackup(timeout, backupName)
		Expect(err).ToNot(HaveOccurred())

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
			dvSpec := NewDataVolumeForBlankRawImage("included-test-dv", "100Mi")
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, includedNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForDataVolumePhase(clientSet, includedNamespace.Name, cdiv1.Succeeded, "included-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvSpec = NewDataVolumeForBlankRawImage("other-test-dv", "100Mi")
			dvOther, err := CreateDataVolumeFromDefinition(clientSet, otherNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, otherNamespace.Name, cdiv1.Succeeded, "other-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Crating backup test-backup")
			err = CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, true)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting DVs")
			err = DeleteDataVolume(clientSet, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := WaitDataVolumeDeleted(clientSet, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = DeleteDataVolume(clientSet, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = WaitDataVolumeDeleted(clientSet, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking included DataVolume exists")
			err = WaitForDataVolumePhase(clientSet, includedNamespace.Name, cdiv1.Succeeded, "included-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Checking not included DataVolume does not exist")
			ok, err = WaitDataVolumeDeleted(clientSet, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = DeleteDataVolume(clientSet, includedNamespace.Name, dvIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should only backup and restore VM from included namespace", func() {
			By("Creating VirtualMachines")
			vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
			vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, includedNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, includedNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			vmSpec = newVMSpecDVTemplate("other-test-vm", "100Mi")
			vmOther, err := CreateVirtualMachineFromDefinition(*kvClient, otherNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, otherNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VMs")
			err = DeleteVirtualMachine(*kvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := WaitVirtualMachineDeleted(*kvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = DeleteVirtualMachine(*kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = WaitVirtualMachineDeleted(*kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying included VM exists")
			err = WaitForVirtualMachineStatus(*kvClient, includedNamespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying ignored VM does not exists")
			ok, err = WaitVirtualMachineDeleted(*kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = DeleteVirtualMachine(*kvClient, includedNamespace.Name, vmIncluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Include resources", func() {
		var namespace *v1.Namespace

		BeforeEach(func() {
			var err error
			namespace, err = CreateNamespace(client)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		Context("Standalone DV", func() {
			It("Selecting DV+PVC: Both DVs and PVCs should be backed up and restored, content of PVC re-imported", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = CreateBackupForResources(timeout, backupName, "datavolumes,persistentvolumeclaims", snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following 2 items:
				// - DataVolume
				// - PVC
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(2))

				By("Deleting DVs")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				_, err = WaitForPVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting DV+PVC+PV+VolumeSnapshot+VSContent: Both DVs and PVCs should be backed up and restored, content of PVC not re-imported", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				resources := "datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
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
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeletePVC(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = WaitForPVCPhase(client, namespace.Name, "test-dv", v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting only DVs: the restored DV should recreate its PVC", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = CreateBackupForResources(timeout, backupName, "datavolumes", snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
				Expect(err).ToNot(HaveOccurred())
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(backup.Status.Progress.TotalItems))
				// The backup should contains the following item:
				// - DataVolume
				Expect(backup.Status.Progress.ItemsBackedUp).To(Equal(1))

				By("Deleting DVs")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting only PVCs: PVC should be restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				resources := "persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
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
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeletePVC(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = WaitForPVCPhase(client, namespace.Name, "test-dv", v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())
				pvc, err := FindPVC(client, namespace.Name, "test-dv")
				Expect(len(pvc.OwnerReferences)).To(Equal(0))

				By("Checking DataVolume does not exist")
				Consistently(func() bool {
					_, err := FindDataVolume(clientSet, namespace.Name, "test-dv")
					return apierrs.IsNotFound(err)
				}, "1000ms", "100ms").Should(BeTrue())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM with DVTemplates", func() {
			It("Selecting VM+DV+PVC: VM, DV and PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "included-test-vm-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+DV+PVC: Backing up VM should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Pausing the virtual machine")
				err = PauseVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = StopVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				err = DeleteDataVolume(clientSet, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeletePVC(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatuses(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not VMI or Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not VMI and Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				err = PauseVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = StopVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-vm-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatuses(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+DV+PVC+VMI+Pod: All objects should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+DV: VM, DV should be restored, PVC should be recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+PVC: VM, PVC should be restored, DV should be recreated and bound to the PVC", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				ok, err = WaitPVCDeleted(client, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not DV and PVC: VM should be restored, DV and PVC should be recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VMI (with DV+PVC+Pod) but not VM: Backing up VMI should fail", func() {
				By("Creating VirtualMachine")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting VirtualMachine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup with DV+PVC+Pod")
				resources := "datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VMI (without DV+PVC+Pod) but not VM: Backing up VMI should fail", func() {
				By("Creating VirtualMachine")
				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting VirtualMachine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup without DV+PVC+Pod")
				resources := "virtualmachineinstances"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {
			It("Selecting VM+DV+PVC, VM stopped: VM, DV and PVC should be restored", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("test-vm", "100Mi", source)
				vm, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = DeletePVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM + PVC, VM stopped: VM and PVC should be restored", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test-dv",
					},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				vm, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitPVCDeleted(client, namespace.Name, "tet-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM + PVC, VM running: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				_, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not PVC: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachines")
				source := kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: "test-dv",
					},
				}
				vmSpec := newVMSpec("included-test-vm", "100Mi", source)
				_, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Standalone VMI", func() {
			It("Selecting standalone VMI+DV+PVC+Pod: All objects should be restored", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "100Mi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = DeleteVirtualMachineInstance(*kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI running")
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, "test-vmi", kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv-2")
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI+Pod without DV: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "100Mi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI+Pod without PVC: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithPVC("test-vmi", "100Mi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI without Pod: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "100Mi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Selector includes", func() {
		var namespace *v1.Namespace

		BeforeEach(func() {
			var err error
			namespace, err = CreateNamespace(client)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		Context("Standalone DV", func() {
			It("Should only backup and restore DV selected by label", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("included-test-dv", "100Mi")
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "included-test-dv")
				Expect(err).ToNot(HaveOccurred())

				dvSpec = NewDataVolumeForBlankRawImage("other-test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvOther, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "other-test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Crating backup test-backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=include", snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				err = DeleteDataVolume(clientSet, namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking included DataVolume exists")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking not included DataVolume does not exist")
				ok, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvOther.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Backup of DVs selected by label should include PVCs", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("included-test-dv", "100Mi")
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "included-test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Crating backup test-backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=include", snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
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

				err = DeleteDataVolume(clientSet, namespace.Name, dvSpec.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM with DVTemplates", func() {
			It("Backup of a stopped VMs selected by label should include its DVs and PVCs", func() {
				By("Creating VirtualMachines")

				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				_, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				vmSpec = newVMSpecDVTemplate("other-test-vm", "100Mi")
				_, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
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

				vmSpec := newVMSpecDVTemplate("included-test-vm", "100Mi")
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				_, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting VM")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
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

		Context("Standalone VMI", func() {
			It("Backup of VMIs selected by label should include its DVs, PVCs, and Pods", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				dvSpec2 := NewDataVolumeForBlankRawImage("test-dv-2", "100Mi")
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec2.Name))
				_, err = CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec2)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv-2")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "100Mi", "test-dv")
				pvcVolume := kvv1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test-dv-2",
					},
				}
				addVolumeToVMI(vmiSpec, pvcVolume, "pvc-volume")
				vmiSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName)
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
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv-2")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
