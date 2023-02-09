package tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

var _ = Describe("DV Backup", func() {
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string

	var f = framework.NewFramework()

	BeforeEach(func() {
		timeout, cancelFunc = context.WithTimeout(context.Background(), 5*time.Minute)
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

	Context("[smoke] Backup", func() {
		var dv *cdiv1.DataVolume

		BeforeEach(func() {
			var err error
			dvSpec := framework.NewDataVolumeForBlankRawImage("test-dv", "100Mi", f.StorageClass)
			dvSpec.Annotations[forceBindAnnotation] = "true"

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dv, err = framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForDataVolumePhase(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:9682]Backup should succeed", func() {
			err := framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))
		})

		It("[test_id:9683]DataVolume should be restored", func() {
			By("Crating backup test-backup")
			err := framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting DataVolume")
			err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			ok, err := framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking DataVolume exists")
			err = framework.WaitForDataVolumePhase(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("[negative] Backup", func() {
		var dv *cdiv1.DataVolume
		var sourceNamespace *v1.Namespace

		BeforeEach(func() {
			var err error
			sourceNamespace, err = f.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(sourceNamespace)
		})

		AfterEach(func() {
			err := framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:9684]DataVolume should be restored", func() {
			var err error
			By("Creating source DV")
			sourceVolumeName := "source-volume"
			srcDvSpec := framework.NewDataVolumeForBlankRawImage(sourceVolumeName, "100Mi", f.StorageClass)
			srcDvSpec.Annotations[forceBindAnnotation] = "true"

			By(fmt.Sprintf("Creating DataVolume %s", srcDvSpec.Name))
			srcDv, err := framework.CreateDataVolumeFromDefinition(f.KvClient, sourceNamespace.Name, srcDvSpec)
			Expect(err).ToNot(HaveOccurred())
			err = framework.WaitForDataVolumePhase(f.KvClient, srcDv.Namespace, cdiv1.Succeeded, srcDv.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating source pod")
			podSpec := framework.NewPod("source-use-pod", sourceVolumeName, "while true; do echo hello; sleep 2; done")
			_, err = f.KvClient.CoreV1().Pods(sourceNamespace.Name).Create(context.TODO(), podSpec, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() v1.PodPhase {
				pod, err := f.KvClient.CoreV1().Pods(sourceNamespace.Name).Get(context.TODO(), podSpec.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return pod.Status.Phase
			}, 90*time.Second, 2*time.Second).Should(Equal(v1.PodRunning))

			By("Creating clone DV - object under test")
			dvSpec := framework.NewCloneDataVolume("test-dv", "100Mi", srcDv.Namespace, srcDv.Name, f.StorageClass)
			dv, err = framework.CreateDataVolumeFromDefinition(f.KvClient, f.Namespace.Name, dvSpec)

			By("Creating backup test-backup")
			err = framework.CreateBackupForNamespace(timeout, backupName, f.Namespace.Name, snapshotLocation, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := framework.GetBackupPhase(timeout, backupName, f.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting DataVolume")
			err = framework.DeleteDataVolume(f.KvClient, f.Namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := framework.WaitDataVolumeDeleted(f.KvClient, f.Namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Deleting source pod")
			err = f.KvClient.CoreV1().Pods(sourceNamespace.Name).Delete(context.TODO(), podSpec.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Creating restore test-restore")
			err = framework.CreateRestoreForBackup(timeout, backupName, restoreName, f.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = framework.WaitForRestorePhase(timeout, restoreName, f.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking DataVolume exists")
			err = framework.WaitForDataVolumePhase(f.KvClient, f.Namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
