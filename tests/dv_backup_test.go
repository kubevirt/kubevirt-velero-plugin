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
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

const snapshotLocation = "test-location"

var kvClient *kubecli.KubevirtClient

var _ = Describe("DV Backup", func() {
	var client, _ = util.GetK8sClient()
	var namespace *v1.Namespace
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var backupName string
	var restoreName string

	var r = framework.NewKubernetesReporter()

	BeforeSuite(func() {
		var err error

		kvClient, err = util.GetKubeVirtclient()
		Expect(err).ToNot(HaveOccurred())

		err = CreateSnapshotLocation(context.TODO(), snapshotLocation, "aws", r.Region, r.BackupNamespace)
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		var err error
		timeout, cancelFunc = context.WithTimeout(context.Background(), 5*time.Minute)
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
		err := DeleteBackup(timeout, backupName, r.BackupNamespace)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
		}

		err = client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}

		cancelFunc()
	})

	Context("[smoke] Backup", func() {
		var dv *cdiv1.DataVolume

		BeforeEach(func() {
			var err error
			dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi", r.StorageClass)
			dvSpec.Annotations[forceBindAnnotation] = "true"

			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dv, err = CreateDataVolumeFromDefinition(*kvClient, namespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForDataVolumePhase(*kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := DeleteDataVolume(*kvClient, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Backup should succeed", func() {
			err := CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, backupName, r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))
		})

		It("DataVolume should be restored", func() {
			By("Crating backup test-backup")
			err := CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, backupName, r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting DataVolume")
			err = DeleteDataVolume(*kvClient, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			ok, err := WaitDataVolumeDeleted(*kvClient, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking DataVolume exists")
			err = WaitForDataVolumePhase(*kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("[negative] Backup", func() {
		var dv *cdiv1.DataVolume
		var sourceNamespace *v1.Namespace

		BeforeEach(func() {
			var err error
			sourceNamespace, err = CreateNamespace(client)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := DeleteDataVolume(*kvClient, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			err = client.CoreV1().Namespaces().Delete(context.TODO(), sourceNamespace.Name, metav1.DeleteOptions{})
			if err != nil && !apierrs.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}

		})

		It("xxx DataVolume should be restored", func() {
			var err error
			By("Creating source DV")
			sourceVolumeName := "source-volume"
			srcDvSpec := NewDataVolumeForBlankRawImage(sourceVolumeName, "100Mi", r.StorageClass)
			srcDvSpec.Annotations[forceBindAnnotation] = "true"

			By(fmt.Sprintf("Creating DataVolume %s", srcDvSpec.Name))
			srcDv, err := CreateDataVolumeFromDefinition(*kvClient, sourceNamespace.Name, srcDvSpec)
			Expect(err).ToNot(HaveOccurred())
			err = WaitForDataVolumePhase(*kvClient, srcDv.Namespace, cdiv1.Succeeded, srcDv.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Creating source pod")
			podSpec := NewPod("source-use-pod", sourceVolumeName, "while true; do echo hello; sleep 2; done")
			_, err = (*kvClient).CoreV1().Pods(sourceNamespace.Name).Create(context.TODO(), podSpec, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() v1.PodPhase {
				pod, err := (*kvClient).CoreV1().Pods(sourceNamespace.Name).Get(context.TODO(), podSpec.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return pod.Status.Phase
			}, 90*time.Second, 2*time.Second).Should(Equal(v1.PodRunning))

			By("Creating clone DV - object under test")
			dvSpec := NewCloneDataVolume("test-dv", "100Mi", srcDv.Namespace, srcDv.Name, r.StorageClass)
			dv, err = CreateDataVolumeFromDefinition(*kvClient, namespace.Name, dvSpec)

			By("Creating backup test-backup")
			err = CreateBackupForNamespace(timeout, backupName, namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, backupName, r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting DataVolume")
			err = DeleteDataVolume(*kvClient, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			ok, err := WaitDataVolumeDeleted(*kvClient, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Deleting source pod")
			err = (*kvClient).CoreV1().Pods(sourceNamespace.Name).Delete(context.TODO(), podSpec.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Creating restore test-restore")
			err = CreateRestoreForBackup(timeout, backupName, restoreName, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForRestorePhase(timeout, restoreName, r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking DataVolume exists")
			err = WaitForDataVolumePhase(*kvClient, namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
