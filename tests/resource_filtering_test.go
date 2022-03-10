package tests

import (
	"context"
	"fmt"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
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

var newVMSpecBlankDVTemplate = func(vmName, size string) *kvv1.VirtualMachine {
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
								v1.ResourceName(v1.ResourceMemory): resource.MustParse("256M"),
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
						Source: &cdiv1.DataVolumeSource{
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
								v1.ResourceName(v1.ResourceMemory): resource.MustParse("256M"),
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

func newVMISpec(vmiName string) *kvv1.VirtualMachineInstance {
	var zero int64 = 0

	vmi := &kvv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmiName,
		},
		Spec: kvv1.VirtualMachineInstanceSpec{
			Domain: kvv1.DomainSpec{
				Resources: kvv1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceMemory): resource.MustParse("512M"),
					},
				},
				Machine: &kvv1.Machine{
					Type: "q35",
				},
				Devices: kvv1.Devices{
					Rng:   &kvv1.Rng{},
					Disks: []kvv1.Disk{},
					Interfaces: []kvv1.Interface{{
						Name: "default",
						InterfaceBindingMethod: kvv1.InterfaceBindingMethod{
							Masquerade: &kvv1.InterfaceMasquerade{},
						},
					}},
				},
			},
			Networks: []kvv1.Network{{
				Name: "default",
				NetworkSource: kvv1.NetworkSource{
					Pod: &kvv1.PodNetwork{},
				},
			}},
			Volumes:                       []kvv1.Volume{},
			TerminationGracePeriodSeconds: &zero,
		},
	}

	return vmi
}

func newBigVMISpecWithDV(vmiName, dvName string) *kvv1.VirtualMachineInstance {
	networkData := `ethernets:
  eth0:
    addresses:
    - fd10:0:2::2/120
    dhcp4: true
    gateway6: fd10:0:2::1
    match: {}
    nameservers:
      addresses:
      - 10.96.0.10
      search:
      - default.svc.cluster.local
      - svc.cluster.local
      - cluster.local
version: 2`
	vmi := newVMISpec(vmiName)

	dvSource := kvv1.VolumeSource{
		DataVolume: &kvv1.DataVolumeSource{
			Name: dvName,
		},
	}
	networkDataSource := kvv1.VolumeSource{
		CloudInitNoCloud: &kvv1.CloudInitNoCloudSource{
			NetworkData: networkData,
		},
	}
	vmi = addVolumeToVMI(vmi, dvSource, "volume0")
	vmi = addVolumeToVMI(vmi, networkDataSource, "volume1")
	return vmi
}

func newVMISpecWithDV(vmiName, dvName string) *kvv1.VirtualMachineInstance {
	vmi := newVMISpec(vmiName)

	source := kvv1.VolumeSource{
		DataVolume: &kvv1.DataVolumeSource{
			Name: dvName,
		},
	}
	vmi = addVolumeToVMI(vmi, source, "volume0")
	return vmi
}

func newVMISpecWithPVC(vmiName, pvcName string) *kvv1.VirtualMachineInstance {
	vmi := newVMISpec(vmiName)

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
	var backupName string
	var restoreName string
	var r = framework.NewKubernetesReporter()

	BeforeEach(func() {
		timeout, cancelFunc = context.WithTimeout(context.Background(), 5*time.Minute)
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
		err := DeleteBackup(timeout, backupName, r.BackupNamespace)
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
			dvSpec := NewDataVolumeForBlankRawImage("included-test-dv", "100Mi", r.StorageClass)
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, includedNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForDataVolumePhase(clientSet, includedNamespace.Name, cdiv1.Succeeded, "included-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvSpec = NewDataVolumeForBlankRawImage("other-test-dv", "100Mi", r.StorageClass)
			dvOther, err := CreateDataVolumeFromDefinition(clientSet, otherNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, otherNamespace.Name, cdiv1.Succeeded, "other-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Crating backup test-backup")
			err = CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
			err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
			vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
			vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, includedNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, includedNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
			vmOther, err := CreateVirtualMachineFromDefinition(*kvClient, otherNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, otherNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = CreateBackupForNamespace(timeout, backupName, includedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
			err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = CreateBackupForResources(timeout, backupName, "datavolumes,persistentvolumeclaims", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				resources := "datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = CreateBackupForResources(timeout, backupName, "datavolumes", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				resources := "persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances,persistentvolumeclaims"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod+PVC: Backup should succeed, DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,virtualmachineinstances"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+VMI but not Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not VMI or Pod: Backing up should fail if the VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not VMI and Pod: Backing up should succeed if the VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-vm-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM+DV+PVC+VMI+Pod: All objects should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,datavolumes"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachines"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VMI (without DV+PVC+Pod) but not VM: Backing up VMI should fail", func() {
				By("Creating VirtualMachine")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {

			It("Selecting VM+DV+PVC, VM stopped: VM, DV and PVC should be restored", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting VM but not PVC: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("[smoke] Standalone VMI", func() {
			It("Selecting standalone VMI+DV+PVC+Pod: All objects should be restored", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "datavolumes,virtualmachineinstances,pods,persistentvolumeclaims,persistentvolumes,volumesnapshots,volumesnapshotcontents"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = DeleteVirtualMachineInstance(*kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI+Pod without PVC: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances,pods"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Selecting standalone VMI without Pod: Backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = CreateBackupForResources(timeout, backupName, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
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
				dvSpec := NewDataVolumeForBlankRawImage("included-test-dv", "100Mi", r.StorageClass)
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "included-test-dv")
				Expect(err).ToNot(HaveOccurred())

				dvSpec = NewDataVolumeForBlankRawImage("other-test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvOther, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "other-test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Crating backup test-backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=include", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
				dvSpec := NewDataVolumeForBlankRawImage("included-test-dv", "100Mi", r.StorageClass)
				dvSpec.Labels = map[string]string{
					"a.test.label": "include",
				}
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "included-test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Crating backup test-backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=include", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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

				vmSpec := CreateVmWithGuestAgent("included-test-vm")
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				_, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
				_, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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

				vmSpec := CreateVmWithGuestAgent("included-test-vm")
				vmSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				vm, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting VM")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				dvSpec2 := NewDataVolumeForBlankRawImage("test-dv-2", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec2.Name))
				_, err = CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec2)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv-2")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				pvcVolume := kvv1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test-dv-2",
					},
				}
				addVolumeToVMI(vmiSpec, pvcVolume, "pvc-volume")
				vmiSpec.Labels = map[string]string{
					"a.test.label": "included",
				}
				vm, err := CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				err = CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Veryfing backup")
				backup, err := GetBackup(timeout, backupName, r.BackupNamespace)
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

var _ = Describe("Resource excludes", func() {
	var client, _ = util.GetK8sClient()
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var namespace *v1.Namespace
	var backupName string
	var restoreName string
	var r = framework.NewKubernetesReporter()

	BeforeEach(func() {
		var err error
		timeout, cancelFunc = context.WithTimeout(context.Background(), 5*time.Minute)
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
		err := DeleteBackup(timeout, backupName, r.BackupNamespace)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		err = client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
			}
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
			dvSpec := NewDataVolumeForBlankRawImage("excluded-test-dv", "100Mi", r.StorageClass)
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvExcluded, err := CreateDataVolumeFromDefinition(clientSet, excludedNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForDataVolumePhase(clientSet, excludedNamespace.Name, cdiv1.Succeeded, "excluded-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dvSpec = NewDataVolumeForBlankRawImage("other-test-dv", "100Mi", r.StorageClass)
			dvOther, err := CreateDataVolumeFromDefinition(clientSet, otherNamespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, otherNamespace.Name, cdiv1.Succeeded, "other-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Crating backup test-backup")
			err = CreateBackupForNamespaceExcludeNamespace(timeout, backupName, otherNamespace.Name, excludedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting DVs")
			err = DeleteDataVolume(clientSet, excludedNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := WaitDataVolumeDeleted(clientSet, excludedNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = DeleteDataVolume(clientSet, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = WaitDataVolumeDeleted(clientSet, otherNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking included DataVolume exists")
			err = WaitForDataVolumePhase(clientSet, otherNamespace.Name, cdiv1.Succeeded, "other-test-dv")
			Expect(err).ToNot(HaveOccurred())

			By("Checking not included DataVolume does not exist")
			ok, err = WaitDataVolumeDeleted(clientSet, excludedNamespace.Name, dvOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = DeleteDataVolume(clientSet, otherNamespace.Name, dvExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should not backup and restore VM from excluded namespace", func() {
			By("Creating VirtualMachines")
			vmSpec := newVMSpecBlankDVTemplate("excluded-test-vm", "100Mi")
			vmExcluded, err := CreateVirtualMachineFromDefinition(*kvClient, excludedNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, excludedNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			vmSpec = newVMSpecBlankDVTemplate("other-test-vm", "100Mi")
			vmOther, err := CreateVirtualMachineFromDefinition(*kvClient, otherNamespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(clientSet, otherNamespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = CreateBackupForNamespaceExcludeNamespace(timeout, backupName, otherNamespace.Name, excludedNamespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VMs")
			err = DeleteVirtualMachine(*kvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := WaitVirtualMachineDeleted(*kvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = DeleteVirtualMachine(*kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = WaitVirtualMachineDeleted(*kvClient, otherNamespace.Name, vmOther.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying included VM exists")
			err = WaitForVirtualMachineStatus(*kvClient, otherNamespace.Name, vmOther.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying ignored VM does not exists")
			ok, err = WaitVirtualMachineDeleted(*kvClient, excludedNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Cleanup")
			err = DeleteVirtualMachine(*kvClient, otherNamespace.Name, vmExcluded.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Exclude resources", func() {
		Context("Standalone DV", func() {
			It("PVC excluded: DV restored, PVC be re-imported", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "persistentvolumeclaims", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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

			It("DV excluded: PVC restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup test-backup")
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "datavolumes", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeletePVC(client, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = WaitForPVCPhase(client, namespace.Name, "test-dv", v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())
				pvc, err := FindPVC(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
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
			It("Pods excluded, VM running: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "pods", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Pods+DV excluded, VM running: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods,datavolumes"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Pods+PVC excluded, VM running: VM+DV restored, PVC re-imported", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods,persistentvolumeclaims"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Pods excluded, VM stopped: VM+DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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

			It("Pods excluded, VM paused: VM+DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := CreateVmWithGuestAgent("test-vm")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = PauseVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI excluded, Pod not excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("PVC excluded: DV restored, PVC re-imported", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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

			It("DV+PVC excluded: VM restored, DV+PVC recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolumes,persistentvolumeclaims"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("DV excluded: VM+PVC restored, DV recreated and bound to the PVC", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolumes"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Running VM excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachine"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Stopped VM excluded: DV+PVC should be restored", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("included-test-vm", "100Mi")
				vm, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "virtualmachine"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Delete VM")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				err = DeleteDataVolume(clientSet, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM does not exists")
				_, err = GetVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {
			It("VM with DV Volume, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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

				By("Creating backup")
				resources := "datavolumes"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM with DV Volume, DV included, PVC excluded: VM+DV recreated, PVC recreated and re-imported", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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

				By("Verifying VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusProvisioning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM with PVC Volume, PVC excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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
				_, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "persistentvolumeclaims"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Standalone VMI", func() {
			It("VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pods"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Pause VMI")
				err = PauseVirtualMachine(*kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "pod"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
			})

			It("[smoke] Pod included, VMI excluded: backup should succeed, only DV and PVC restored", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				vm, err := CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Creating backup")
				resources := "virtualmachineinstances"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = DeleteVirtualMachineInstance(*kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI not present")
				_, err = GetVirtualMachineInstance(*kvClient, namespace.Name, "test-vmi")
				Expect(err).To(HaveOccurred())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI+Pod included, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Creating backup")
				resources := "datavolumes"
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, resources, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
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
			dv, err := FindDataVolume(clientSet, namespace.Name, name)
			Expect(err).ToNot(HaveOccurred())

			dv.SetLabels(addExcludeLabel(dv.GetLabels()))

			_, err = clientSet.CdiV1beta1().DataVolumes(namespace.Name).Update(context.TODO(), dv, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		addExcludeLabelToPVC := func(name string) {
			pvc, err := FindPVC(client, namespace.Name, name)
			Expect(err).ToNot(HaveOccurred())

			pvc.SetLabels(addExcludeLabel(pvc.GetLabels()))

			_, err = client.CoreV1().PersistentVolumeClaims(namespace.Name).Update(context.TODO(), pvc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		addExcludeLabelToVMI := func(name string) {
			vmi, err := (*kvClient).VirtualMachineInstance(namespace.Name).Get(name, &metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			vmi.SetLabels(addExcludeLabel(vmi.GetLabels()))

			_, err = (*kvClient).VirtualMachineInstance(namespace.Name).Update(vmi)
			Expect(err).ToNot(HaveOccurred())
		}

		addExcludeLabelToVM := func(name string) {
			vm, err := (*kvClient).VirtualMachine(namespace.Name).Get(name, &metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			vm.SetLabels(addExcludeLabel(vm.GetLabels()))

			_, err = (*kvClient).VirtualMachine(namespace.Name).Update(vm)
			Expect(err).ToNot(HaveOccurred())
		}

		addExcludeLabelToLauncherPodForVM := func(vmName string) {
			var pod v1.Pod
			pods, err := client.CoreV1().Pods(namespace.Name).List(context.TODO(), metav1.ListOptions{
				LabelSelector: "kubevirt.io=virt-launcher",
			})
			Expect(err).ToNot(HaveOccurred())
			for _, item := range pods.Items {
				if ann, ok := item.GetAnnotations()["kubevirt.io/domain"]; ok && ann == vmName {
					pod = item
				}
			}
			Expect(pod).ToNot(BeNil())

			pod.SetLabels(addExcludeLabel(pod.GetLabels()))

			_, err = client.CoreV1().Pods(namespace.Name).Update(context.TODO(), &pod, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		Context("Standalone DV", func() {
			It("DV included, PVC excluded: PVC should be re-imported", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Add exlude label to PVC")
				addExcludeLabelToPVC("test-dv")

				By("Creating backup")
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "persistentvolumeclaims", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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

			// TODO: BR: check what should happen with PVC here
			XIt("PVC included, DV excluded: PVC should be restored, ownership relation empty", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				dvIncluded, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())

				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Add exclude label to DV")
				addExcludeLabelToDV("test-dv")

				By("Creating backup")
				err = CreateBackupForNamespaceExcludeResources(timeout, backupName, namespace.Name, "persistentvolumeclaims", snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting DVs")
				err = DeleteDataVolume(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = WaitDataVolumeDeleted(clientSet, namespace.Name, dvIncluded.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Creating restore test-restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking PVC exists")
				err = WaitForPVCPhase(client, namespace.Name, "test-dv", v1.ClaimBound)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not exists")
				_, err = FindDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).To(HaveOccurred())

			})
		})

		Context("VM with DVTemplates", func() {
			It("VM included, VMI excluded: should fail if VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to VMI")
				addExcludeLabelToVMI("test-vm")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM+VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating VirtualMachines")
				vmSpec := CreateVmWithGuestAgent("test-vm")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vm")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			// TODO: BR: check what should happen with paused VM, does a freeze hook work there?
			// is there a need to skip freeze when paused?
			XIt("VM+VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating VirtualMachines")
				vmSpec := CreateVmWithGuestAgent("test-vm")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pausing the virtual machine")
				err = PauseVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusPaused)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to DV")
				addExcludeLabelToDV(vmSpec.Spec.DataVolumeTemplates[0].Name)

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM included, DV and PVC excluded: both DV and PVC recreated", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude labels")
				addExcludeLabelToDV(vmSpec.Spec.DataVolumeTemplates[0].Name)
				addExcludeLabelToPVC(vmSpec.Spec.DataVolumeTemplates[0].Name)

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM+PVC included, DV excluded: VM and PVC should be restored, DV recreated and bound to the PVC", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to DV")
				addExcludeLabelToDV(vmSpec.Spec.DataVolumeTemplates[0].Name)

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMs")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitVirtualMachineDeleted(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmIncluded.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI included, VM excluded: backup should fail", func() {
				By("Creating VirtualMachines")
				vmSpec := newVMSpecBlankDVTemplate("test-vm", "100Mi")
				vmIncluded, err := CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, vmSpec.Spec.DataVolumeTemplates[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Starting the virtual machine")
				err = StartVirtualMachine(*kvClient, namespace.Name, vmSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vmSpec.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to VM")
				addExcludeLabelToVM("test-vm")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vmIncluded.Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("VM without DVTemplates", func() {
			It("VM with DV Volume, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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
				_, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label")
				addExcludeLabelToDV("test-dv")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM with DV Volume, DV included, PVC excluded: VM+DV recreated, PVC recreated and re-imported", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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

				By("Verifying VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude labels")
				addExcludeLabelToPVC("test-dv")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
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
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume re-imports content")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume import succeeds")
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying included VM exists")
				err = WaitForVirtualMachineStatus(*kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusProvisioning)
				Expect(err).ToNot(HaveOccurred())

				By("Cleanup")
				err = DeleteVirtualMachine(*kvClient, namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VM with PVC Volume, PVC excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
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
				_, err = CreateVirtualMachineFromDefinition(*kvClient, namespace.Name, vmSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude labels")
				addExcludeLabelToPVC("test-dv")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("[smoke] Standalone VMI", func() {
			It("VMI included, Pod excluded: should fail if VM is running", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newVMISpecWithDV("test-vmi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vmi")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI included, Pod excluded: should succeed if VM is paused", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vmiSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Pause VMI")
				err = PauseVirtualMachine(*kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to pod")
				addExcludeLabelToLauncherPodForVM("test-vmi")

				// time.Sleep(300 * time.Second)
				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = DeleteVirtualMachineInstance(*kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
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
			})

			It("Pod included, VMI excluded: backup should succeed, only DV and PVC restored", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForFedoraWithGuestAgentImage("test-dv", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())
				ok, err := WaitForVirtualMachineInstanceCondition(*kvClient, namespace.Name, vmiSpec.Name, kvv1.VirtualMachineInstanceAgentConnected)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

				By("Adding exclude label to VMI")
				addExcludeLabelToVMI("test-vmi")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VMI+DV")
				err = DeleteVirtualMachineInstance(*kvClient, namespace.Name, vmiSpec.Name)
				Expect(err).ToNot(HaveOccurred())
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				ok, err = WaitPVCDeleted(client, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				By("Creating restore")
				err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
				Expect(err).ToNot(HaveOccurred())

				By("Checking DataVolume does not re-import content")
				err = WaitForDataVolumePhaseButNot(clientSet, namespace.Name, cdiv1.Succeeded, cdiv1.ImportScheduled, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VMI not present")
				_, err = GetVirtualMachineInstance(*kvClient, namespace.Name, "test-vmi")
				Expect(err).To(HaveOccurred())

				By("Cleanup")
				err = DeleteDataVolume(clientSet, namespace.Name, "test-dv")
				Expect(err).ToNot(HaveOccurred())
			})

			It("VMI+Pod included, DV excluded: backup should fail", func() {
				By("Creating DVs")
				dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
				By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
				_, err := CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
				Expect(err).ToNot(HaveOccurred())

				By("Creating VirtualMachineInstance")
				vmiSpec := newBigVMISpecWithDV("test-vmi", "test-dv")
				_, err = CreateVirtualMachineInstanceFromDefinition(*kvClient, namespace.Name, vmiSpec)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForVirtualMachineInstancePhase(*kvClient, namespace.Name, vmiSpec.Name, kvv1.Running)
				Expect(err).ToNot(HaveOccurred())

				By("Adding exclude label to DV")
				addExcludeLabelToDV("test-dv")

				By("Creating backup")
				err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
				Expect(err).ToNot(HaveOccurred())
				err = WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhasePartiallyFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
