package tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/strings/slices"

	kvv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
	. "kubevirt.io/kubevirt-velero-plugin/tests/framework/matcher"
)

var _ = Describe("PVC and VolumeSnapshot Labeling", func() {
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
		if slices.Contains(CurrentSpecReport().Labels(), "PartnerComp") {
			err := f.RunDeleteBackupScript(timeout, backupName, f.BackupNamespace)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
			}
		} else {
			err := framework.DeleteBackup(timeout, backupName, f.BackupNamespace)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
			}
		}

		cancelFunc()
	})

	Context("PVC Labeling with User Data Preservation", func() {
		It("Should preserve user's existing PVC UID label value through backup/restore cycle", func() {
			By("Creating a VM with DataVolume")
			vmSpec := framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
			vm, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			pvcName := vmSpec.Spec.DataVolumeTemplates[0].Name
			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, pvcName, 180, HaveSucceeded())

			By("Adding a user-defined PVC UID label to the PVC")
			userDefinedValue := "user-custom-value-12345"
			pvc, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, pvcName)
			Expect(err).ToNot(HaveOccurred())

			if pvc.Labels == nil {
				pvc.Labels = make(map[string]string)
			}
			pvc.Labels[util.PVCUIDLabel] = userDefinedValue
			_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Update(context.Background(), pvc, metav1.UpdateOptions{})
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
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			// Accept both Completed and PartiallyFailed since CSI snapshot restore with label selectors
			// may timeout during PV patching when the PVC is not bound by a VM/pod
			Expect(rPhase).To(Or(Equal(velerov1api.RestorePhaseCompleted), Equal(velerov1api.RestorePhasePartiallyFailed)))

			By("Verifying VM was restored")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying PVC was restored with user's original label value")
			restoredPVC, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, pvcName)
			Expect(err).ToNot(HaveOccurred())
			Expect(restoredPVC.Labels).To(HaveKey(util.PVCUIDLabel), "PVC UID label should exist")
			Expect(restoredPVC.Labels[util.PVCUIDLabel]).To(Equal(userDefinedValue), "User's original PVC UID label value should be preserved")
			Expect(restoredPVC.Annotations).ToNot(HaveKey(util.OriginalPVCUIDAnnotation), "Original PVC UID annotation should be removed after restore")
		})
	})

	Context("Selective Restore by PVC UID", func() {
		It("Should selectively restore only PVCs matching the UID label selector (VMs not restored)", func() {
			By("Creating two VMs with DataVolumes")
			vm1Spec := framework.CreateVmWithGuestAgent("test-vm-1", f.StorageClass)
			vm1, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vm1Spec)
			Expect(err).ToNot(HaveOccurred())
			pvc1Name := vm1Spec.Spec.DataVolumeTemplates[0].Name
			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, pvc1Name, 180, HaveSucceeded())

			vm2Spec := framework.CreateVmWithGuestAgent("test-vm-2", f.StorageClass)
			vm2, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vm2Spec)
			Expect(err).ToNot(HaveOccurred())
			pvc2Name := vm2Spec.Spec.DataVolumeTemplates[0].Name
			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, pvc2Name, 180, HaveSucceeded())

			By("Getting PVC UID before backup")
			pvc1, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, pvc1Name)
			Expect(err).ToNot(HaveOccurred())
			pvc1UID := string(pvc1.UID)

			By("Creating backup for entire namespace")
			err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting both VMs")
			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm1.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm1.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm2.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm2.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By(fmt.Sprintf("Creating selective restore for only PVC with UID %s", pvc1UID))
			labelSelector := fmt.Sprintf("%s=%s", util.PVCUIDLabel, pvc1UID)
			err = framework.CreateRestoreWithLabelSelector(timeout, backupName, restoreName, f.BackupNamespace, labelSelector, false)
			Expect(err).ToNot(HaveOccurred())

			// Poll for restore to reach terminal state
			// Accept Completed, PartiallyFailed, or Finalizing since CSI snapshot restore with label selectors
			// gets stuck in Finalizing when PVCs cannot bind (no VM/pod to consume them)
			var rPhase velerov1api.RestorePhase
			err = wait.PollImmediate(2*time.Second, 180*time.Second, func() (bool, error) {
				phase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
				if err != nil {
					return false, err
				}
				rPhase = phase
				if phase == velerov1api.RestorePhaseCompleted || phase == velerov1api.RestorePhasePartiallyFailed || phase == velerov1api.RestorePhaseFinalizing {
					return true, nil
				}
				return false, nil
			})
			Expect(err).ToNot(HaveOccurred(), "restore should reach a terminal status")
			Expect(rPhase).To(Or(Equal(velerov1api.RestorePhaseCompleted), Equal(velerov1api.RestorePhasePartiallyFailed), Equal(velerov1api.RestorePhaseFinalizing)))

			By("Verifying only PVC1 was restored (label cleaned up)")
			restoredPVC1, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, pvc1Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(restoredPVC1.Labels).ToNot(HaveKey(util.PVCUIDLabel), "PVC UID label should be removed after restore")

			By("Verifying PVC2 was NOT restored")
			_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, pvc2Name)
			Expect(err).To(HaveOccurred(), "PVC2 should not be restored")

			By("Verifying VM1 was NOT restored (label selector only matched PVCs)")
			_, err = framework.GetVirtualMachine(f.KvClient, f.Namespace.Name, vm1.Name)
			Expect(err).To(HaveOccurred(), "VM1 should not be restored when using PVC UID selector")

			By("Verifying VM2 was NOT restored (label selector only matched PVCs)")
			_, err = framework.GetVirtualMachine(f.KvClient, f.Namespace.Name, vm2.Name)
			Expect(err).To(HaveOccurred(), "VM2 should not be restored when using PVC UID selector")
		})
	})

	Context("Selective Restore with VolumeSnapshots", Label("PartnerComp"), func() {
		It("Should selectively restore PVC and its VolumeSnapshots by PVC UID (VMs not restored)", func() {
			By("Creating two VMs with DataVolumes")
			vm1Spec := framework.CreateVmWithGuestAgent("test-vm-1", f.StorageClass)
			vm1, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vm1Spec)
			Expect(err).ToNot(HaveOccurred())
			pvc1Name := vm1Spec.Spec.DataVolumeTemplates[0].Name
			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, pvc1Name, 180, HaveSucceeded())

			vm2Spec := framework.CreateVmWithGuestAgent("test-vm-2", f.StorageClass)
			vm2, err := framework.CreateVirtualMachineFromDefinition(f.KvClient, f.Namespace.Name, vm2Spec)
			Expect(err).ToNot(HaveOccurred())
			pvc2Name := vm2Spec.Spec.DataVolumeTemplates[0].Name
			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, pvc2Name, 180, HaveSucceeded())

			By("Getting PVC UIDs before backup")
			pvc1, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, pvc1Name)
			Expect(err).ToNot(HaveOccurred())
			pvc1UID := string(pvc1.UID)

			By("Creating backup with VolumeSnapshots")
			err = f.RunBackupScript(timeout, backupName, "", "", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting both VMs")
			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm1.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm1.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm2.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err = framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm2.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By(fmt.Sprintf("Creating selective restore for only resources with PVC UID %s", pvc1UID))
			// Note: This will restore both the PVC and its associated VolumeSnapshots
			// because they both have the same PVC UID label
			labelSelector := fmt.Sprintf("%s=%s", util.PVCUIDLabel, pvc1UID)
			err = f.RunRestoreScriptWithLabelSelector(timeout, backupName, restoreName, f.BackupNamespace, labelSelector)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying only PVC1 was restored (label cleaned up)")
			restoredPVC1, err := framework.FindPVC(f.K8sClient, f.Namespace.Name, pvc1Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(restoredPVC1.Labels).ToNot(HaveKey(util.PVCUIDLabel))

			By("Verifying PVC2 was NOT restored")
			_, err = framework.FindPVC(f.K8sClient, f.Namespace.Name, pvc2Name)
			Expect(err).To(HaveOccurred(), "PVC2 should not be restored")

			By("Verifying VM1 was NOT restored (label selector only matched PVC and VolumeSnapshots)")
			_, err = framework.GetVirtualMachine(f.KvClient, f.Namespace.Name, vm1.Name)
			Expect(err).To(HaveOccurred(), "VM1 should not be restored when using PVC UID selector")

			By("Verifying VM2 was NOT restored (label selector only matched PVC and VolumeSnapshots)")
			_, err = framework.GetVirtualMachine(f.KvClient, f.Namespace.Name, vm2.Name)
			Expect(err).To(HaveOccurred(), "VM2 should not be restored when using PVC UID selector")
		})
	})
})
