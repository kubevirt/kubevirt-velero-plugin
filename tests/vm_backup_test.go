package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvv1 "kubevirt.io/api/core/v1"
	instancetypev1alpha2 "kubevirt.io/api/instancetype/v1alpha2"
	kubecli "kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

var _ = Describe("[smoke] VM Backup", func() {
	var client, _ = util.GetK8sClient()
	var kvClient kubecli.KubevirtClient
	var namespace *v1.Namespace
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string
	var vm *kvv1.VirtualMachine

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

	JustAfterEach(func() {
		By("JustAfterEach")

		if CurrentGinkgoTestDescription().Failed {
			r.FailureCount++
			r.Dump(CurrentGinkgoTestDescription().Duration)
		}
	})

	AfterEach(func() {
		// Deleting the backup also deletes all restores, volume snapshots etc.
		err := framework.DeleteBackup(timeout, backupName, r.BackupNamespace)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
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
		dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
		dvSpec.Annotations[forceBindAnnotation] = "true"

		By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
		dv, err := framework.CreateDataVolumeFromDefinition(kvClient, namespace.Name, dvSpec)
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, dvSpec.Name)
		Expect(err).ToNot(HaveOccurred())
		// creating a started VM, so it works correctly also on WFFC storage
		By("Starting a VM")
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
		vm, err = framework.CreateStartedVirtualMachine(kvClient, namespace.Name, vmSpec)
		Expect(err).ToNot(HaveOccurred())

		By("Stopping a VM")
		err = framework.StopVirtualMachine(kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Creating backup")
		err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting DataVolume")
		err = framework.DeleteDataVolume(kvClient, namespace.Name, dv.Name)
		Expect(err).ToNot(HaveOccurred())

		ok, err := framework.WaitDataVolumeDeleted(kvClient, namespace.Name, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())

		By("Creating restore")
		err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Checking DataVolume exists")
		err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, dvSpec.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForDataVolumePhase(kvClient, namespace.Name, cdiv1.Succeeded, vm.Spec.DataVolumeTemplates[0].Name)
		Expect(err).ToNot(HaveOccurred())
	})

	It("started VM should be restored - with guest agent", func() {
		// creating a started VM, so it works correctly also on WFFC storage
		By("Starting a VM")
		vm, err := framework.CreateStartedVirtualMachine(kvClient, namespace.Name, framework.CreateVmWithGuestAgent("test-vm", r.StorageClass))
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
		ok, err := framework.WaitForVirtualMachineInstanceCondition(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineInstanceAgentConnected)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue(), "VirtualMachineInstanceAgentConnected should be true")

		By("Creating backup")
		err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Stopping VM")
		err = framework.StopVirtualMachine(kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
	})

	It("started VM should be restored - without guest agent", func() {
		// creating a started VM, so it works correctly also on WFFC storage
		By("Starting a VM")
		vm, err := framework.CreateStartedVirtualMachine(kvClient, namespace.Name, framework.CreateVmWithGuestAgent("test-vm", r.StorageClass))
		Expect(err).ToNot(HaveOccurred())

		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())

		By("Creating backup")
		err = framework.CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		phase, err := framework.GetBackupPhase(timeout, backupName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

		By("Stopping VM")
		err = framework.StopVirtualMachine(kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())
		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting VM")
		err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Creating restore")
		err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
		Expect(err).ToNot(HaveOccurred())

		rPhase, err := framework.GetRestorePhase(timeout, restoreName, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

		By("Verifying VM")
		err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("VM and VMI object graph backup", func() {
		It("with instancetype and preference", func() {
			By("Create instancetype and preference")
			instancetype := newVirtualMachineInstancetype(namespace.Name)
			instancetype, err := kvClient.VirtualMachineInstancetype(namespace.Name).
				Create(context.Background(), instancetype, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			preference := newVirtualMachinePreference(namespace.Name)
			preference, err = kvClient.VirtualMachinePreference(namespace.Name).
				Create(context.Background(), preference, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM")
			vmSpec := framework.CreateVmWithInstancetypeAndPreference("test-vm", r.StorageClass, instancetype.Name, preference.Name)
			vmSpec.Labels = map[string]string{
				"a.test.label": "included",
			}
			vm, err = framework.CreateStartedVirtualMachine(kvClient, namespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())
			By("Wait instance type controller revision to be updated on VM spec")
			Eventually(func(g Gomega) {
				vm, err = kvClient.VirtualMachine(vm.Namespace).Get(vm.Name, &metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(vm.Spec.Instancetype.RevisionName).ToNot(BeEmpty())
			}, 2*time.Minute, 2*time.Second).Should(Succeed())

			By("Creating backup")
			err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM, instancetype and preference")
			err = kvClient.VirtualMachineInstancetype(namespace.Name).
				Delete(context.Background(), instancetype.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = kvClient.VirtualMachinePreference(namespace.Name).
				Delete(context.Background(), preference.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.DeleteVirtualMachineAndWait(kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			// Wait until ControllerRevision is deleted
			Eventually(func(g Gomega) metav1.StatusReason {
				_, err := kvClient.AppsV1().ControllerRevisions(namespace.Name).Get(context.Background(), vm.Spec.Instancetype.RevisionName, metav1.GetOptions{})
				return errors.ReasonForError(err)
			}, 2*time.Minute, 2*time.Second).Should(Equal(metav1.StatusReasonNotFound))

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := framework.GetRestorePhase(timeout, restoreName, r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())
		})

		It("with configmap, secret and serviceaccount", func() {
			By("Creating configmap and secret")
			t := time.Now().UnixNano()
			configMapName := fmt.Sprintf("configmap-%d", t)
			secretName := fmt.Sprintf("secret-%d", t)
			err := createConfigMap(kvClient, configMapName, namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			err = createSecret(kvClient, secretName, namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM")
			vmSpec := createVMWithDifferentVolumes("test-vm", r.StorageClass, configMapName, secretName)
			vmSpec.Labels = map[string]string{
				"a.test.label": "included",
			}
			vm, err = framework.CreateStartedVirtualMachine(kvClient, namespace.Name, vmSpec)
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

			By("Deleting VM and volumes")
			err = deleteConfigMap(kvClient, configMapName, namespace.Name)
			err = deleteConfigMap(kvClient, configMapName, namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			err = deleteSecret(kvClient, secretName, namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())

			err = deleteSecret(kvClient, secretName, namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := framework.GetRestorePhase(timeout, restoreName, r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Verifying VM")
			err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusStopped)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying config map and secret exist")
			_, err = kvClient.CoreV1().ConfigMaps(namespace.Name).Get(context.Background(), configMapName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = kvClient.CoreV1().Secrets(namespace.Name).Get(context.Background(), secretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("with access credentials", func() {
			By("Creating access credentials")
			t := time.Now().UnixNano()
			secretName := fmt.Sprintf("secret-%d", t)
			err := createAccessCredentialsSecret(kvClient, secretName, namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Starting a VM")
			vmSpec := createVMWithAccessCredentials("test-vm", r.StorageClass, secretName)
			vmSpec.Labels = map[string]string{
				"a.test.label": "included",
			}
			vm, err = framework.CreateStartedVirtualMachine(kvClient, namespace.Name, vmSpec)
			Expect(err).ToNot(HaveOccurred())

			By("Creating backup")
			err = framework.CreateBackupForSelector(timeout, backupName, "a.test.label=included", snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForBackupPhase(timeout, backupName, r.BackupNamespace, velerov1api.BackupPhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting VM and access credentials secret")
			err = framework.DeleteVirtualMachine(kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitVirtualMachineDeleted(kvClient, namespace.Name, vm.Name)
			Expect(err).ToNot(HaveOccurred())
			err = deleteSecret(kvClient, secretName, namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := framework.GetRestorePhase(timeout, restoreName, r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Verifying VM")
			_, err = kvClient.CoreV1().Secrets(namespace.Name).Get(context.Background(), secretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForVirtualMachineStatus(kvClient, namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying secret exists")
		})
	})
})

func newVirtualMachineInstancetype(namespace string) *instancetypev1alpha2.VirtualMachineInstancetype {
	return &instancetypev1alpha2.VirtualMachineInstancetype{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vm-instancetype-",
			Namespace:    namespace,
		},
		Spec: instancetypev1alpha2.VirtualMachineInstancetypeSpec{
			CPU: instancetypev1alpha2.CPUInstancetype{
				Guest: uint32(1),
			},
			Memory: instancetypev1alpha2.MemoryInstancetype{
				Guest: resource.MustParse("256M"),
			},
		},
	}
}

func newVirtualMachinePreference(namespace string) *instancetypev1alpha2.VirtualMachinePreference {
	return &instancetypev1alpha2.VirtualMachinePreference{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vm-preference-",
			Namespace:    namespace,
		},
		Spec: instancetypev1alpha2.VirtualMachinePreferenceSpec{
			CPU: &instancetypev1alpha2.CPUPreferences{
				PreferredCPUTopology: instancetypev1alpha2.PreferSockets,
			},
		},
	}
}

func createConfigMap(kvClient kubecli.KubevirtClient, name, namespace string) error {
	data := map[string]string{
		"option1": "value1",
		"option2": "value2",
		"option3": "value3",
	}
	_, err := kvClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       data,
	}, metav1.CreateOptions{})
	return err
}

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

func getConfigMapSourcePath(volumeName string) string {
	return filepath.Join("/var/run/kubevirt-private/config-map", volumeName)
}

func createSecret(kvClient kubecli.KubevirtClient, name, namespace string) error {
	data := map[string]string{
		"user":     "admin",
		"password": "community",
	}
	_, err := kvClient.CoreV1().Secrets(namespace).Create(context.Background(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: data,
	}, metav1.CreateOptions{})
	return err
}

func createAccessCredentialsSecret(kvClient kubecli.KubevirtClient, name, namespace string) error {
	customPassword := "imadethisup"
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"fedora": []byte(customPassword),
		},
	}
	_, err := kvClient.CoreV1().Secrets(namespace).Create(context.Background(), &secret, metav1.CreateOptions{})
	return err
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

func createVMWithAccessCredentials(vmName, storageClassName, secretName string) *kvv1.VirtualMachine {
	vmSpec := framework.CreateFedoraVmWithGuestAgent("test-vm", storageClassName)
	vmSpec.Spec.Template.Spec.AccessCredentials = []kvv1.AccessCredential{
		{
			UserPassword: &kvv1.UserPasswordAccessCredential{
				Source: kvv1.UserPasswordAccessCredentialSource{
					Secret: &kvv1.AccessCredentialSecretSource{
						SecretName: secretName,
					},
				},
				PropagationMethod: kvv1.UserPasswordAccessCredentialPropagationMethod{
					QemuGuestAgent: &kvv1.QemuGuestAgentUserPasswordAccessCredentialPropagation{},
				},
			},
		},
	}
	return vmSpec
}

func createVMWithDifferentVolumes(vmName, storageClassName, configMapName, secretName string) *kvv1.VirtualMachine {
	vmSpec := framework.CreateVmWithGuestAgent("test-vm", storageClassName)
	vmSpec.Spec.Template.Spec.Volumes = append(vmSpec.Spec.Template.Spec.Volumes, kvv1.Volume{
		Name: "config-volume",
		VolumeSource: kvv1.VolumeSource{
			ConfigMap: &kvv1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	})
	vmSpec.Spec.Template.Spec.Volumes = append(vmSpec.Spec.Template.Spec.Volumes, kvv1.Volume{
		Name: "secret-volume",
		VolumeSource: kvv1.VolumeSource{
			Secret: &kvv1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	})
	vmSpec.Spec.Template.Spec.Volumes = append(vmSpec.Spec.Template.Spec.Volumes, kvv1.Volume{
		Name: "sa-volume",
		VolumeSource: kvv1.VolumeSource{
			ServiceAccount: &kvv1.ServiceAccountVolumeSource{
				ServiceAccountName: "default",
			},
		},
	})
	vmSpec.Spec.Template.Spec.Domain.Devices.Disks = append(vmSpec.Spec.Template.Spec.Domain.Devices.Disks, kvv1.Disk{
		Name: "config-volume",
	})
	vmSpec.Spec.Template.Spec.Domain.Devices.Disks = append(vmSpec.Spec.Template.Spec.Domain.Devices.Disks, kvv1.Disk{
		Name: "secret-volume",
	})
	vmSpec.Spec.Template.Spec.Domain.Devices.Disks = append(vmSpec.Spec.Template.Spec.Domain.Devices.Disks, kvv1.Disk{
		Name: "sa-volume",
	})

	return vmSpec
}
