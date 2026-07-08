package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"

	kvv1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
	. "kubevirt.io/kubevirt-velero-plugin/tests/framework/matcher"
)

const (
	dvName           = "test-dv"
	dvTemplateName   = "test-dv-template"
	dvForPVCName     = "test-pvc"
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

		if vm != nil {
			err := framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
			}
		}

		cancelFunc()
	})

	It("[test_id:10267]Stopped VM should be restored", Label("PartnerComp"), func() {
		By(fmt.Sprintf("Creating DataVolume %s", dvName))
		err := f.CreateBlankDataVolume()
		Expect(err).ToNot(HaveOccurred())

		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())
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
		err = f.RunBackupScript(timeout, backupName, "", "", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())

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
		err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Checking DataVolume exists")
		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())
		framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, vm.Spec.DataVolumeTemplates[0].Name, 180, HaveSucceeded())
	})

	It("[test_id:10268]started VM should be restored - with guest agent", func() {
		// creating a started VM, so it works correctly also on WFFC storage
		var err error
		By("Starting a VM")
		vm, err = framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, framework.CreateVmWithGuestAgent("test-vm", f.StorageClass))
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

	It("[test_id:10269]started VM should be restored - without guest agent", func() {
		// creating a started VM, so it works correctly also on WFFC storage
		var err error
		By("Starting a VM")
		vm, err = framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, framework.CreateVmWithGuestAgent("test-vm", f.StorageClass))
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

	DescribeTable("should respect power state configuration after restore", func(startVM bool, restoreLabel map[string]string, expectedState kvv1.VirtualMachinePrintableStatus) {
		By("Creating a VM")
		var err error
		vm, err = framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, framework.CreateVmWithGuestAgent("test-vm", f.StorageClass))
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		if !startVM {
			By("Stopping VM")
			err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
		}

		By("Creating a backup for the VM")
		err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		if startVM {
			By("Stopping VM")
			err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
		}

		By("Deleting the VM")
		err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore with specific label")
		err = framework.CreateRestoreWithLabels(timeout, backupName, restoreName, f.BackupNamespace, true, restoreLabel)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Validating the restored VM state")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, expectedState)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("Restore with Always run strategy label should start the VM",
			false,
			map[string]string{"velero.kubevirt.io/restore-run-strategy": "Always"},
			kvv1.VirtualMachineStatusRunning,
		),
		Entry("Restore with Halted run strategy label should stop the VM",
			true,
			map[string]string{"velero.kubevirt.io/restore-run-strategy": "Halted"},
			kvv1.VirtualMachineStatusStopped,
		),
	)

	It("VM should be restored with new firmware UUID when using appropriate label", func() {
		By("Starting a VM")
		var err error
		vm = framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
		vm.Spec.Template.Spec.Domain.Firmware = &kvv1.Firmware{
			// Choosing arbitrary UUID
			UUID: types.UID("123e4567-e89b-12d3-a456-426614174000"),
		}

		vm, err = framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, vm)
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
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

		By("Creating restore with new firmware UUID label")
		err = framework.CreateRestoreWithLabels(timeout, backupName, restoreName, f.BackupNamespace, true, map[string]string{"velero.kubevirt.io/generate-new-firmware-uuid": "true"})
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying restored VM")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		By("Checking new firmware UUID")
		restoredVM, err := f.KvClient.VirtualMachine(f.Namespace.Name).Get(context.TODO(), vm.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(restoredVM.Spec.Template.Spec.Domain.Firmware.UUID).ToNot(Equal(vm.Spec.Template.Spec.Domain.Firmware.UUID))
	})

	It("VM with backend storage PVC should be backed up and restored appropriately", func() {
		By("Updating VM state storage class")
		framework.UpdateVMStateStorageClass(f.KvClient)

		By("Creating a VM with backend storage PVC")
		var err error
		vm = framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
		vm.Spec.Template.Spec.Domain.Devices.TPM = &kvv1.TPMDevice{Persistent: ptr.To(true)}
		vm, err = framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, vm)
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		By("Expecting the creation of a backend storage PVC")
		pvc, err := getPersistentStatePVC(f.KvClient, vm)
		Expect(err).ToNot(HaveOccurred())

		By("Stopping the VM")
		err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Creating backup")
		err = f.RunBackupScript(timeout, backupName, "", "", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())
		err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		By("Checking backend storage PVC exists")
		pvc2, err := getPersistentStatePVC(f.KvClient, vm)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc2.Name).To(Equal(pvc.Name))
	})

	It("started VM should be restored with new MAC address", func() {
		// creating a started VM, so it works correctly also on WFFC storage
		var err error
		By("Starting a VM")
		vm, err = framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, framework.CreateVmWithGuestAgent("test-vm", f.StorageClass))
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		By("Retrieving the original MAC address")
		vm, err = f.KvClient.VirtualMachine(f.Namespace.Name).Get(context.TODO(), vm.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		originalMAC := vm.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress
		if originalMAC == "" {
			// This means there is no KubeMacPool running. We can simply choose a random address
			originalMAC = "DE-AD-00-00-BE-AF"
			update := func(vm *kvv1.VirtualMachine) *kvv1.VirtualMachine {
				vm.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress = originalMAC
				return vm
			}
			retryOnceOnErr(updateVm(f.KvClient, f.Namespace.Name, vm.Name, update)).Should(BeNil())

			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())
		}

		By("Creating backup")
		err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = framework.CreateRestoreWithClearedMACAddress(timeout, backupName, restoreName, f.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, f.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying restored VM")
		err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		By("Retrieving the restored MAC address")
		vm, err = f.KvClient.VirtualMachine(f.Namespace.Name).Get(context.TODO(), vm.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		restoredMAC := vm.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress
		Expect(restoredMAC).ToNot(Equal(originalMAC))
	})

	Context("VM and VMI object graph backup", func() {
		Context("with instancetypes and preferences", func() {
			nsDelFunc := func() {
				err := f.KvClient.VirtualMachineInstancetype(f.Namespace.Name).
					Delete(context.Background(), instancetypeName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = f.KvClient.VirtualMachinePreference(f.Namespace.Name).
					Delete(context.Background(), preferenceName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			clusterDelFunc := func() {
				err := f.KvClient.VirtualMachineClusterInstancetype().
					Delete(context.Background(), instancetypeName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = f.KvClient.VirtualMachineClusterPreference().
					Delete(context.Background(), preferenceName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			clusterCleanup := func() {
				err := f.KvClient.VirtualMachineClusterInstancetype().
					Delete(context.Background(), instancetypeName, metav1.DeleteOptions{})
				if err != nil {
					Expect(errors.IsNotFound(err)).To(BeTrue())
				}
				err = f.KvClient.VirtualMachineClusterPreference().
					Delete(context.Background(), preferenceName, metav1.DeleteOptions{})
				if err != nil {
					Expect(errors.IsNotFound(err)).To(BeTrue())
				}
			}

			updateInstancetypeFunc := func() {
				instancetype, err := f.KvClient.VirtualMachineInstancetype(f.Namespace.Name).Get(context.Background(), instancetypeName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				instancetype.Spec.CPU.Guest = instancetype.Spec.CPU.Guest + 1
				instancetype.Spec.Memory.Guest.Add(resource.MustParse("128Mi"))
				_, err = f.KvClient.VirtualMachineInstancetype(f.Namespace.Name).Update(context.Background(), instancetype, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			updateClusterInstancetypeFunc := func() {
				instancetype, err := f.KvClient.VirtualMachineClusterInstancetype().Get(context.Background(), instancetypeName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				instancetype.Spec.CPU.Guest = instancetype.Spec.CPU.Guest + 1
				instancetype.Spec.Memory.Guest.Add(resource.MustParse("128Mi"))
				_, err = f.KvClient.VirtualMachineClusterInstancetype().Update(context.Background(), instancetype, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			DescribeTable("with instancetype and preference", Label("PartnerComp"), func(itFunc, pFunc, vmFunc func() error, instancetypeUpdateFunc, delFunc, cleanupFunc func()) {
				if cleanupFunc != nil {
					defer cleanupFunc()
				}
				By("Create instancetype and preference")
				err := itFunc()
				Expect(err).ToNot(HaveOccurred())
				err = pFunc()
				Expect(err).ToNot(HaveOccurred())

				By("Starting a VM")
				err = vmFunc()
				Expect(err).ToNot(HaveOccurred())
				vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-instancetype-and-preference", dvName)
				Expect(err).ToNot(HaveOccurred())

				By("Wait instance type controller revision to be updated on VM spec")
				Eventually(func(g Gomega) {
					vm, err = f.KvClient.VirtualMachine(f.Namespace.Name).Get(context.Background(), vm.Name, metav1.GetOptions{})
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(vm.Status.InstancetypeRef).ToNot(BeNil())
					g.Expect(vm.Status.InstancetypeRef.ControllerRevisionRef).ToNot(BeNil())
					g.Expect(vm.Status.InstancetypeRef.ControllerRevisionRef.Name).ToNot(BeEmpty())
					g.Expect(vm.Status.PreferenceRef).ToNot(BeNil())
					g.Expect(vm.Status.PreferenceRef.ControllerRevisionRef).ToNot(BeNil())
					g.Expect(vm.Status.PreferenceRef.ControllerRevisionRef.Name).ToNot(BeEmpty())
					_, err := f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Status.InstancetypeRef.ControllerRevisionRef.Name, metav1.GetOptions{})
					g.Expect(err).ToNot(HaveOccurred())
					_, err = f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Status.PreferenceRef.ControllerRevisionRef.Name, metav1.GetOptions{})
					g.Expect(err).ToNot(HaveOccurred())
				}, 2*time.Minute, 2*time.Second).Should(Succeed())

				By("Fetching the vCPU and memory configuration from the VMI")
				originalVMI, err := f.KvClient.VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Fetching copies of the original ControllerRevisions")
				itControllerRevision, err := f.KvClient.AppsV1().ControllerRevisions(vm.Namespace).Get(context.Background(), vm.Status.InstancetypeRef.ControllerRevisionRef.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				pControllerRevision, err := f.KvClient.AppsV1().ControllerRevisions(vm.Namespace).Get(context.Background(), vm.Status.PreferenceRef.ControllerRevisionRef.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Mutating the existing instance type and preference objects")
				instancetypeUpdateFunc()

				By("Creating backup")
				err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting VM, instancetype and preference")
				delFunc()

				ok, err := framework.DeleteVirtualMachineAndWait(f.KvClient, f.Namespace.Name, vm.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				// Wait until ControllerRevision is deleted
				Eventually(func(g Gomega) metav1.StatusReason {
					_, err := f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Status.InstancetypeRef.ControllerRevisionRef.Name, metav1.GetOptions{})
					if err != nil && errors.ReasonForError(err) != metav1.StatusReasonNotFound {
						return errors.ReasonForError(err)
					}
					_, err = f.KvClient.AppsV1().ControllerRevisions(f.Namespace.Name).Get(context.Background(), vm.Status.PreferenceRef.ControllerRevisionRef.Name, metav1.GetOptions{})
					return errors.ReasonForError(err)
				}, 2*time.Minute, 2*time.Second).Should(Equal(metav1.StatusReasonNotFound))

				By("Creating restore")
				err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying VM is running")
				err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
				Expect(err).ToNot(HaveOccurred())

				By("Ensuring the original ControllerRevisions are referenced by the VirtualMachine")
				vm, err := f.KvClient.VirtualMachine(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(vm.Spec.Instancetype.RevisionName).To(Equal(itControllerRevision.Name))
				Expect(vm.Spec.Preference.RevisionName).To(Equal(pControllerRevision.Name))
				Expect(vm.Status.InstancetypeRef.ControllerRevisionRef.Name).To(Equal(itControllerRevision.Name))
				Expect(vm.Status.PreferenceRef.ControllerRevisionRef.Name).To(Equal(pControllerRevision.Name))

				By("Ensuring the restored VMI is using the same vCPU and memory configuration as the original")
				restoredVMI, err := f.KvClient.VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(restoredVMI.Spec.Domain.CPU).To(Equal(originalVMI.Spec.Domain.CPU))
				Expect(restoredVMI.Spec.Domain.Memory.Guest.Equal(*originalVMI.Spec.Domain.Memory.Guest)).To(BeTrue())
			},
				Entry("[test_id:10270]namespace scope", f.CreateInstancetype, f.CreatePreference, f.CreateVMWithInstancetypeAndPreference, updateInstancetypeFunc, nsDelFunc, nil),
				Entry("[test_id:10274]cluster scope", f.CreateClusterInstancetype, f.CreateClusterPreference, f.CreateVMWithClusterInstancetypeAndClusterPreference, updateClusterInstancetypeFunc, clusterDelFunc, clusterCleanup),
			)
		})

		It("[test_id:10271]with configmap, secret and serviceaccount", Label("PartnerComp"), func() {
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
			err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
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
			err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying config map and secret exist")
			_, err = f.KvClient.CoreV1().ConfigMaps(f.Namespace.Name).Get(context.Background(), configMapName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = f.KvClient.CoreV1().Secrets(f.Namespace.Name).Get(context.Background(), secretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:10272]with access credentials", Label("PartnerComp"), func() {
			By("Creating access credentials")
			err := f.CreateAccessCredentialsSecret()
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM")
			err = f.CreateVMWithAccessCredentials()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-access-credentials", dvName)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
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
			err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying secret exists")
			_, err = f.KvClient.CoreV1().Secrets(f.Namespace.Name).Get(context.Background(), acSecretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:10273]VM with standalone PVC", Label("PartnerComp"), func() {
			By(fmt.Sprintf("Creating DataVolume %s to create PVC", dvForPVCName))
			err := f.CreatePVCUsingDataVolume()
			Expect(err).ToNot(HaveOccurred())

			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvForPVCName, 180, HaveSucceeded())

			// creating a started VM, so it works correctly also on WFFC storage
			By("Starting a VM")
			err = f.CreateVMWithPVC()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-pvc", dvForPVCName)
			Expect(err).ToNot(HaveOccurred())

			By("Stopping a VM")
			err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			err = framework.DeleteDataVolumeWithoutDeletingPVC(f.KvClient, f.Namespace.Name, dvForPVCName)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating backup")
			err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM")
			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting PVC")
			err = framework.DeletePVC(f.KvClient, f.Namespace.Name, dvForPVCName)
			Expect(err).ToNot(HaveOccurred())

			_, err = framework.WaitPVCDeleted(f.KvClient, f.Namespace.Name, dvForPVCName)
			Expect(err).ToNot(HaveOccurred())

			By("Creating restore")
			err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
			err = framework.StartVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Checking PVC exists")
			err = framework.WaitForPVCPhase(f.K8sClient, f.Namespace.Name, dvForPVCName, v1.ClaimBound)
			Expect(err).ToNot(HaveOccurred())
		})
		It("VM with NetworkAttachmentDefinition should be backed up and restored", func() {
			_, err := f.K8sClient.Discovery().ServerResourcesForGroupVersion("k8s.cni.cncf.io/v1")
			if err != nil {
				Skip("NetworkAttachmentDefinition CRD not available on this cluster")
			}

			const nadName = "test-nad"

			By("Creating NetworkAttachmentDefinition")
			err = f.CreateNetworkAttachmentDefinition()
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM with Multus secondary network")
			err = f.CreateVMWithNAD()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-nad", dvName)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Dumping VM YAML before backup")
			dumpVMYaml(f.KvClient, f.Namespace.Name, vm.Name)

			By("Creating backup with label selector (only VM is labeled, NAD must be discovered by plugin)")
			err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Stopping VM before deletion")
			err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM and NAD")
			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			err = deleteNetworkAttachmentDefinition(f, nadName, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating restore")
			err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Dumping VM YAML after restore")
			dumpVMYaml(f.KvClient, f.Namespace.Name, vm.Name)

			By("Verifying NetworkAttachmentDefinition was restored")
			nadRestored, err := getNetworkAttachmentDefinition(f, nadName, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(nadRestored).ToNot(BeEmpty(), "NAD should have been restored")

			By("Verifying VM still references the NAD in its spec")
			restoredVM, err := f.KvClient.VirtualMachine(f.Namespace.Name).Get(context.TODO(), vm.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			foundNAD := false
			for _, net := range restoredVM.Spec.Template.Spec.Networks {
				if net.Multus != nil && net.Multus.NetworkName == nadName {
					foundNAD = true
					break
				}
			}
			Expect(foundNAD).To(BeTrue(), "Restored VM should reference the NAD")
		})

		It("VM with NAD should be backed up and restored to a different namespace with namespace mapping", func() {
			_, err := f.K8sClient.Discovery().ServerResourcesForGroupVersion("k8s.cni.cncf.io/v1")
			if err != nil {
				Skip("NetworkAttachmentDefinition CRD not available on this cluster")
			}

			const nadName = "test-nad"

			By("Creating target namespace for namespace-mapped restore")
			targetNs, err := f.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			By("Creating NetworkAttachmentDefinition")
			err = f.CreateNetworkAttachmentDefinition()
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM with Multus secondary network")
			err = f.CreateVMWithNAD()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-nad", dvName)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Dumping VM and NAD YAML before backup")
			dumpVMYaml(f.KvClient, f.Namespace.Name, vm.Name)
			dumpNADYaml(f, nadName, f.Namespace.Name)

			By("Creating backup with label selector (only VM is labeled, NAD must be discovered by plugin)")
			err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Stopping VM")
			err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Creating restore with namespace mapping and clear MAC address")
			err = framework.CreateRestoreWithNamespaceMapping(timeout, backupName, restoreName, f.BackupNamespace,
				map[string]string{f.Namespace.Name: targetNs.Name},
				map[string]string{"velero.kubevirt.io/clear-mac-address": "true"},
				true)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, targetNs.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Dumping VM and NAD YAML after restore")
			dumpVMYaml(f.KvClient, targetNs.Name, vm.Name)
			dumpNADYaml(f, nadName, targetNs.Name)

			By("Verifying NetworkAttachmentDefinition was restored")
			nadRestored, err := getNetworkAttachmentDefinition(f, nadName, targetNs.Name)
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: NAD %q in namespace %q: %s\n", nadName, targetNs.Name, nadRestored)
			Expect(nadRestored).ToNot(BeEmpty(), "NAD should have been restored")

			By("Verifying VM still references the NAD in its spec")
			restoredVM, err := f.KvClient.VirtualMachine(targetNs.Name).Get(context.TODO(), vm.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			foundNAD := false
			for _, net := range restoredVM.Spec.Template.Spec.Networks {
				fmt.Fprintf(GinkgoWriter, "INFO: VM network entry: Name=%s, Multus=%v\n", net.Name, net.Multus)
				if net.Multus != nil && net.Multus.NetworkName == nadName {
					foundNAD = true
					break
				}
			}
			Expect(foundNAD).To(BeTrue(), "Restored VM should reference the NAD")

			By("Verifying original VM still exists in source namespace")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: Original VM %q in source namespace %q is Stopped\n", vm.Name, f.Namespace.Name)
		})

		It("VM with NAD-OVN should be backed up and restored to a different namespace with namespace mapping", func() {
			_, err := f.K8sClient.Discovery().ServerResourcesForGroupVersion("k8s.cni.cncf.io/v1")
			if err != nil {
				Skip("NetworkAttachmentDefinition CRD not available on this cluster")
			}

			const nadName = "nad-1"

			By("Creating target namespace for namespace-mapped restore")
			targetNs, err := f.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			By("Creating NetworkAttachmentDefinition")
			err = f.CreateNetworkAttachmentDefinitionOvn()
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM with Multus secondary network")
			err = f.CreateVMWithNADOvn()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-with-nad-ovn", dvName)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Dumping VM and NAD YAML before backup")
			dumpVMYaml(f.KvClient, f.Namespace.Name, vm.Name)
			dumpNADYaml(f, nadName, f.Namespace.Name)

			By("Creating backup with label selector (only VM is labeled, NAD must be discovered by plugin)")
			err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Stopping VM")
			err = framework.StopVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())

			By("Creating restore with namespace mapping and clear MAC address")
			err = framework.CreateRestoreWithNamespaceMapping(timeout, backupName, restoreName, f.BackupNamespace,
				map[string]string{f.Namespace.Name: targetNs.Name},
				map[string]string{"velero.kubevirt.io/clear-mac-address": "true"},
				true)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, targetNs.Name, vm.Name, kvv1.VirtualMachineStatusStopped, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Dumping VM and NAD YAML after restore")
			dumpVMYaml(f.KvClient, targetNs.Name, vm.Name)
			dumpNADYaml(f, nadName, targetNs.Name)

			By("Verifying NetworkAttachmentDefinition was restored")
			nadRestored, err := getNetworkAttachmentDefinition(f, nadName, targetNs.Name)
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: NAD %q in namespace %q: %s\n", nadName, targetNs.Name, nadRestored)
			Expect(nadRestored).ToNot(BeEmpty(), "NAD should have been restored")

			By("Verifying NAD config has remapped netAttachDefName to target namespace")
			nadConfig, err := getNetworkAttachmentDefinitionConfig(f, nadName, targetNs.Name)
			Expect(err).ToNot(HaveOccurred())
			expectedNetAttachDefName := fmt.Sprintf("%s/%s", targetNs.Name, nadName)
			fmt.Fprintf(GinkgoWriter, "INFO: NAD config: %s\n", nadConfig)
			fmt.Fprintf(GinkgoWriter, "INFO: Expected netAttachDefName substring: %s\n", expectedNetAttachDefName)
			Expect(nadConfig).To(ContainSubstring(expectedNetAttachDefName),
				fmt.Sprintf("NAD config should reference target namespace: expected %s in config %s", expectedNetAttachDefName, nadConfig))

			By("Verifying VM still references the NAD in its spec")
			restoredVM, err := f.KvClient.VirtualMachine(targetNs.Name).Get(context.TODO(), vm.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			foundNAD := false
			for _, net := range restoredVM.Spec.Template.Spec.Networks {
				fmt.Fprintf(GinkgoWriter, "INFO: VM network entry: Name=%s, Multus=%v\n", net.Name, net.Multus)
				if net.Multus != nil && net.Multus.NetworkName == nadName {
					foundNAD = true
					break
				}
			}
			Expect(foundNAD).To(BeTrue(), "Restored VM should reference the NAD")

			By("Verifying original VM still exists in source namespace")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: Original VM %q in source namespace %q is Stopped\n", vm.Name, f.Namespace.Name)
		})

		It("Backup hooks on virt-launcher pod should execute when VM is selected via labelSelector", func() {
			var err error

			By("Creating a VM with a custom label")
			vm = framework.CreateVmWithGuestAgent("test-vm", f.StorageClass)
			// Only the VM carries this label — the virt-launcher pod will NOT have it,
			vm.Labels = map[string]string{"test-backup": "true"}

			vm, err = framework.CreateStartedVirtualMachine(f.KvClient, f.Namespace.Name, vm)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying the virt-launcher pod does NOT carry the custom label")
			virtLauncherPod := framework.FindLauncherPod(f.K8sClient, f.Namespace.Name, vm.Name)
			Expect(virtLauncherPod.Labels).ToNot(HaveKey("test-backup"),
				"virt-launcher pod must not carry the 'test-backup' label")
			fmt.Fprintf(GinkgoWriter, "INFO: virt-launcher pod %q labels: %v\n", virtLauncherPod.Name, virtLauncherPod.Labels)

			By("Creating backup with labelSelector matching only the VM label")
			err = framework.CreateBackupForSelector(timeout, backupName, "test-backup=true", f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Verifying backup hooks were executed on the virt-launcher pod included via AdditionalItems")
			backup, err := framework.GetBackup(timeout, backupName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(backup.Status.HookStatus).ToNot(BeNil(), "HookStatus should be populated")
			Expect(backup.Status.HookStatus.HooksAttempted).To(Equal(2),
				"Hooks attempted should be 2")
			Expect(backup.Status.HookStatus.HooksFailed).To(BeZero(), "backup hooks should not fail")
		})

		//todo this test vm is not starting correctly, need more eyes to fix.
		PIt("[test_id:10275]VM with hotplug disk", Label("PartnerComp"), func() {
			By("Starting a VM")
			err := f.CreateVMForHotplug()
			Expect(err).ToNot(HaveOccurred())
			vm, err = framework.WaitVirtualMachineRunning(f.KvClient, f.Namespace.Name, "test-vm-for-hotplug", dvTemplateName)
			Expect(err).ToNot(HaveOccurred())

			By("Create datavolume to hotplug")
			err = f.CreateBlankDataVolume()
			Expect(err).ToNot(HaveOccurred())

			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

			By("Adding Hotplug volume to VM")
			hotplugVolName := addVolumeAndVerify(f.KvClient, vm, dvName)

			By("Creating backup")
			err = f.RunBackupScript(timeout, backupName, "", "a.test.label=included", f.Namespace.Name, snapshotLocation, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM")
			err = framework.DeleteVirtualMachine(f.KvClient, f.Namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting hotplug DataVolume")
			err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dvName)
			Expect(err).ToNot(HaveOccurred())

			ok, err := framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dvName)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = f.RunRestoreScript(timeout, backupName, restoreName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())

			By("Checking hotpluged data volume exists")
			framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())

			verifyVolumeAndDiskAdded(f.KvClient, vm.Namespace, vm.Name, hotplugVolName)
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

func addVolumeAndVerify(kvClient kubecli.KubevirtClient, vm *kvv1.VirtualMachine, dvName string) string {
	volumeSource := &kvv1.HotplugVolumeSource{
		DataVolume: &kvv1.DataVolumeSource{
			Name: dvName,
		},
	}
	addVolumeName := "hotplug-volume"
	addVolumeOptions := &kvv1.AddVolumeOptions{
		Name: addVolumeName,
		Disk: &kvv1.Disk{
			DiskDevice: kvv1.DiskDevice{
				Disk: &kvv1.DiskTarget{
					Bus: kvv1.DiskBusSCSI,
				},
			},
			Serial: addVolumeName,
		},
		VolumeSource: volumeSource,
	}

	Eventually(func() error {
		return kvClient.VirtualMachine(vm.Namespace).AddVolume(context.Background(), vm.Name, addVolumeOptions)
	}, 3*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

	verifyVolumeAndDiskAdded(kvClient, vm.Namespace, vm.Name, addVolumeName)

	return addVolumeName
}

func verifyVolumeAndDiskAdded(kvClient kubecli.KubevirtClient, namespace, name, volumeName string) {
	Eventually(func() error {
		updatedVM, err := kvClient.VirtualMachine(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(updatedVM.Status.VolumeRequests) > 0 {
			return fmt.Errorf("waiting on all VolumeRequests to be processed")
		}
		updatedVMI, err := framework.GetVirtualMachineInstance(kvClient, namespace, name)
		if err != nil {
			return err
		}

		foundVolume := false
		foundDisk := false

		for _, volume := range updatedVMI.Spec.Volumes {
			if volume.Name == volumeName {
				foundVolume = true
				break
			}
		}
		for _, disk := range updatedVMI.Spec.Domain.Devices.Disks {
			if disk.Name == volumeName {
				foundDisk = true
				break
			}
		}

		if !foundDisk || !foundVolume {
			return fmt.Errorf("waiting on new disk and volume to appear in VMI")
		}

		return nil
	}, 90*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
}

func dumpVMYaml(kvClient kubecli.KubevirtClient, namespace, name string) {
	vm, err := kvClient.VirtualMachine(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to get VM %s/%s for dump: %v\n", namespace, name, err)
		return
	}
	data, err := json.MarshalIndent(vm, "", "  ")
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to marshal VM %s/%s: %v\n", namespace, name, err)
		return
	}
	fmt.Fprintf(GinkgoWriter, "=== VM %s/%s YAML ===\n%s\n=== End VM ===\n", namespace, name, string(data))
}

func dumpNADYaml(f *framework.Framework, name, namespace string) {
	cfg, err := f.LoadConfig()
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to load config for NAD dump: %v\n", err)
		return
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to create dynamic client for NAD dump: %v\n", err)
		return
	}
	nadGVR := schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}
	nad, err := dynClient.Resource(nadGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to get NAD %s/%s for dump: %v\n", namespace, name, err)
		return
	}
	data, err := json.MarshalIndent(nad.Object, "", "  ")
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "WARN: failed to marshal NAD %s/%s: %v\n", namespace, name, err)
		return
	}
	fmt.Fprintf(GinkgoWriter, "=== NAD %s/%s ===\n%s\n=== End NAD ===\n", namespace, name, string(data))
}

func deleteNetworkAttachmentDefinition(f *framework.Framework, name, namespace string) error {
	cfg, err := f.LoadConfig()
	if err != nil {
		return err
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	nadGVR := schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}
	err = dynClient.Resource(nadGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func getNetworkAttachmentDefinition(f *framework.Framework, name, namespace string) (string, error) {
	cfg, err := f.LoadConfig()
	if err != nil {
		return "", err
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	nadGVR := schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}
	nad, err := dynClient.Resource(nadGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return nad.GetName(), nil
}

func getNetworkAttachmentDefinitionConfig(f *framework.Framework, name, namespace string) (string, error) {
	cfg, err := f.LoadConfig()
	if err != nil {
		return "", err
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	nadGVR := schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions"}
	nad, err := dynClient.Resource(nadGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	spec, ok := nad.Object["spec"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("NAD %s/%s spec not found or not a map", namespace, name)
	}
	config, ok := spec["config"].(string)
	if !ok {
		return "", fmt.Errorf("NAD %s/%s config not found or not a string", namespace, name)
	}
	return config, nil
}

func getPersistentStatePVC(kvClient kubecli.KubevirtClient, vm *kvv1.VirtualMachine) (*v1.PersistentVolumeClaim, error) {
	const pvcPrefix = "persistent-state-for"
	pvcs, err := kvClient.CoreV1().PersistentVolumeClaims(vm.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: pvcPrefix + "=" + vm.Name,
	})
	if err != nil {
		return nil, err
	}
	pvc := &v1.PersistentVolumeClaim{}
	if len(pvcs.Items) == 0 {
		// Kubevirt introduced the backend PVC labeling in 1.4.0.
		// If backend PVC is no labeled, let's fallback to the old naming convention.
		pvc, err = kvClient.CoreV1().PersistentVolumeClaims(vm.Namespace).Get(context.TODO(), pvcPrefix+"-"+vm.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		pvc = &pvcs.Items[0]
	}
	return pvc, nil
}
