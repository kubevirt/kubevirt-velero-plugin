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
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"

	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

const snapshotLocation = "test-location"

var clientSet *cdiclientset.Clientset
var kvClient *kubecli.KubevirtClient

var _ = Describe("DV Backup", func() {
	var client, _ = util.GetK8sClient()
	var namespace *v1.Namespace
	var timeout context.Context
	var cancelFunc context.CancelFunc
	var r = framework.NewKubernetesReporter()

	BeforeSuite(func() {
		var err error
		clientSet, err = util.GetCDIclientset()
		Expect(err).ToNot(HaveOccurred())

		kvClient, err = util.GetKubeVirtclient()
		Expect(err).ToNot(HaveOccurred())

		err = CreateSnapshotLocation(context.TODO(), snapshotLocation, "aws", "minio")
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		var err error
		namespace, err = CreateNamespace(client)
		Expect(err).ToNot(HaveOccurred())

		timeout, cancelFunc = context.WithTimeout(context.Background(), 5*time.Minute)
	})

	AfterEach(func() {
		err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}

		cancelFunc()
	})

	Context("Backup", func() {
		var dv *cdiv1.DataVolume

		BeforeEach(func() {
			var err error
			dvSpec := NewDataVolumeForBlankRawImage("test-dv", "100Mi")
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dv, err = CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := DeleteDataVolume(clientSet, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			err = DeleteBackup(timeout, "test-backup", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Backup should succeed", func() {
			err := CreateBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, "test-backup", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))
		})

		It("DataVolume should be restored", func() {
			By("Crating backup test-backup")
			err := CreateBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := GetBackupPhase(timeout, "test-backup", r.BackupNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting DataVolume")
			err = DeleteDataVolume(clientSet, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			ok, err := WaitDataVolumeDeleted(clientSet, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = CreateRestoreForBackup(timeout, "test-backup", "test-restore", r.BackupNamespace, true)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForRestorePhase(timeout, "test-restore", r.BackupNamespace, velerov1api.RestorePhaseCompleted)
			Expect(err).ToNot(HaveOccurred())

			By("Checking DataVolume exists")
			err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
