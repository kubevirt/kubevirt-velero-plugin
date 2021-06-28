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
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

var _ = Describe("DV Backup", func() {
	var client, _ = util.GetK8sClient()
	var clientSet *cdiclientset.Clientset
	var namespace *v1.Namespace
	var timeout context.Context
	var cancelFunc context.CancelFunc

	const snapshotLocation = "test-location"

	BeforeSuite(func() {
		var err error
		clientSet, err = util.GetCDIclientset()
		Expect(err).ToNot(HaveOccurred())

		err = createSnapshotLocation(context.TODO(), snapshotLocation, "aws", "minio")
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

	var newDataVolumeForBlankRawImage = func(dataVolumeName, size string) *cdiv1.DataVolume {
		return &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:        dataVolumeName,
				Annotations: map[string]string{},
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
		}
	}

	Context("Backup", func() {
		var dv *cdiv1.DataVolume

		BeforeEach(func() {
			var err error
			dvSpec := newDataVolumeForBlankRawImage("test-dv", "100Mi")
			By(fmt.Sprintf("Creating DataVolume %s", dvSpec.Name))
			dv, err = CreateDataVolumeFromDefinition(clientSet, namespace.Name, dvSpec)
			Expect(err).ToNot(HaveOccurred())

			err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := DeleteDataVolume(clientSet, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			err = deleteBackup(timeout, "test-backup")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Backup should succeed", func() {
			err := createBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := getBackupPhase(timeout, "test-backup")
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))
		})

		It("DataVolume should be restored", func() {
			By("Crating backup test-backup")
			err := createBackupForNamespace(timeout, "test-backup", namespace.Name, snapshotLocation, true)
			Expect(err).ToNot(HaveOccurred())

			phase, err := getBackupPhase(timeout, "test-backup")
			Expect(err).ToNot(HaveOccurred())
			Expect(phase).To(Equal(velerov1api.BackupPhaseCompleted))

			By("Deleting DataVolume")
			err = DeleteDataVolume(clientSet, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			ok, err := WaitDataVolumeDeleted(clientSet, namespace.Name, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			By("Creating restore test-restore")
			err = createRestoreForBackup(timeout, "test-backup", "test-restore", true)
			Expect(err).ToNot(HaveOccurred())

			rPhase, err := getRestorePhase(timeout, "test-restore")
			Expect(err).ToNot(HaveOccurred())
			Expect(rPhase).To(Equal(velerov1api.RestorePhaseCompleted))

			By("Checking DataVolume exists")
			err = WaitForDataVolumePhase(clientSet, namespace.Name, cdiv1.Succeeded, "test-dv")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
