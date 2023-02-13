package framework

import (
	"context"
	"fmt"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	pollInterval = 3 * time.Second
	waitTime     = 600 * time.Second
)

func IsDataVolumeGC(kvClient kubecli.KubevirtClient) bool {
	config, err := kvClient.CdiClient().CdiV1beta1().CDIConfigs().Get(context.TODO(), "config", metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	return config.Spec.DataVolumeTTLSeconds == nil || *config.Spec.DataVolumeTTLSeconds >= 0
}

func CreateDataVolumeFromDefinition(clientSet kubecli.KubevirtClient, namespace string, def *cdiv1.DataVolume) (*cdiv1.DataVolume, error) {
	var dataVolume *cdiv1.DataVolume
	// TODO: use cdiclient.Interface
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		dataVolume, err = clientSet.CdiClient().CdiV1beta1().DataVolumes(namespace).Create(context.TODO(), def, metav1.CreateOptions{})
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}

	return dataVolume, nil
}

// FindPVC Finds the PVC by name
func FindPVC(clientSet *kubernetes.Clientset, namespace, pvcName string) (*v1.PersistentVolumeClaim, error) {
	return clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
}

// WaitForPVC waits for a PVC
func WaitForPVC(clientSet *kubernetes.Clientset, namespace, name string) (*v1.PersistentVolumeClaim, error) {
	var pvc *v1.PersistentVolumeClaim
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		pvc, err = FindPVC(clientSet, namespace, name)
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return pvc, nil
}

// WaitForPVCPhase waits for a PVC to reach a given phase
func WaitForPVCPhase(clientSet *kubernetes.Clientset, namespace, name string, phase v1.PersistentVolumeClaimPhase) error {
	var pvc *v1.PersistentVolumeClaim
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		pvc, err = FindPVC(clientSet, namespace, name)
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil || pvc.Status.Phase != phase {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("PVC %s not in phase %s within %v", name, phase, waitTime)
	}
	return nil
}

func FindDataVolume(kvClient kubecli.KubevirtClient, namespace string, dataVolumeName string) (*cdiv1.DataVolume, error) {
	return kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
}

// WaitForDataVolumePhase waits for DV's phase to be in a particular phase (Pending, Bound, or Lost)
func WaitForDataVolumePhase(kvClient kubecli.KubevirtClient, namespace string, phase cdiv1.DataVolumePhase, dataVolumeName string) error {
	ginkgo.By(fmt.Sprintf("INFO: Waiting for status %s", phase))
	var lastPhase cdiv1.DataVolumePhase

	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		dataVolume, err := kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		if dataVolume.Status.Phase != phase {
			if dataVolume.Status.Phase != lastPhase {
				lastPhase = dataVolume.Status.Phase
				ginkgo.By(fmt.Sprintf("INFO: Waiting for status %s, got %s", phase, dataVolume.Status.Phase))
			}
			return false, err
		}

		ginkgo.By(fmt.Sprintf("INFO: Waiting for status %s, got %s", phase, dataVolume.Status.Phase))
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("DataVolume %s not in phase %s within %v", dataVolumeName, phase, waitTime)
	}
	return nil
}

// WaitForDataVolumePhaseButNot waits for DV's phase to be in a particular phase without going through another phase
func WaitForDataVolumePhaseButNot(kvClient kubecli.KubevirtClient, namespace string, phase cdiv1.DataVolumePhase, unwanted cdiv1.DataVolumePhase, dataVolumeName string) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		dataVolume, err := kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if dataVolume.Status.Phase == unwanted {
			return false, fmt.Errorf("reached unawanted phase %s", unwanted)
		}
		if dataVolume.Status.Phase == phase {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
		// return fmt.Errorf("DataVolume %s not in phase %s within %v", dataVolumeName, phase, waitTime)
	}
	return nil
}

// DeleteDataVolume deletes the DataVolume with the given name in case of GC it makes sure to also delete the relevant pvc
func DeleteDataVolume(kvClient kubecli.KubevirtClient, namespace, name string) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	if err != nil || !IsDataVolumeGC(kvClient) {
		return err
	}

	return DeletePVC(kvClient, namespace, name)
}

func DeleteDataVolumeWithoutDeletingPVC(kvClient kubecli.KubevirtClient, namespace, name string) error {
	propagationOrphan := metav1.DeletePropagationOrphan
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{
			PropagationPolicy: &propagationOrphan,
		})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}

// DeletePVC deletes the PVC by name
func DeletePVC(kvClient kubecli.KubevirtClient, namespace string, pvcName string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := kvClient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}

func DataVolumeDeleted(kvClient kubecli.KubevirtClient, namespace, dvName string) (bool, error) {
	_, err := kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dvName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func WaitOnlyDataVolumeDeleted(kvClient kubecli.KubevirtClient, namespace, dvName string) (bool, error) {
	var result bool
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		result, err = DataVolumeDeleted(kvClient, namespace, dvName)
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "ERROR: Get Data volume to check if deleted failed, retrying: %v\n", err.Error())
			return false, nil
		}
		return result, nil
	})

	return result, err
}

func WaitDataVolumeDeleted(kvClient kubecli.KubevirtClient, namespace, dvName string) (bool, error) {
	result, err := WaitOnlyDataVolumeDeleted(kvClient, namespace, dvName)
	if err != nil || !IsDataVolumeGC(kvClient) {
		return result, err
	}

	return WaitPVCDeleted(kvClient, namespace, dvName)
}

func WaitPVCDeleted(kvClient kubecli.KubevirtClient, namespace, pvcName string) (bool, error) {
	var result bool
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		_, err := kvClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				result = true
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	return result, err
}

func NewPVC(pvcName, size, storageClass string) *v1.PersistentVolumeClaim {
	pvcSpec := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse(size),
				},
			},
		},
	}

	if storageClass != "" {
		pvcSpec.Spec.StorageClassName = &storageClass
	}

	return pvcSpec
}

func NewPod(podName, pvcName, cmd string) *v1.Pod {
	importerImage := "quay.io/quay/busybox:latest"
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Annotations: map[string]string{
				"cdi.kubevirt.io/testing": podName,
			},
		},
		Spec: v1.PodSpec{
			// this may be causing an issue
			TerminationGracePeriodSeconds: &[]int64{10}[0],
			RestartPolicy:                 v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "runner",
					Image:   importerImage,
					Command: []string{"/bin/sh", "-c", cmd},
					Resources: v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "storage",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
}

func NewCloneDataVolume(name, size, srcNamespace, srcPvcName string, storageClassName string) *cdiv1.DataVolume {
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Namespace: srcNamespace,
					Name:      srcPvcName,
				},
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

	if storageClassName != "" {
		dv.Spec.PVC.StorageClassName = &storageClassName
	}
	return dv
}

// ThisPVCWith fetches the latest state of the PersistentVolumeClaim based on namespace and name. If the object does not exist, nil is returned.
func ThisPVCWith(kvClient kubecli.KubevirtClient, namespace string, name string) func() (*v1.PersistentVolumeClaim, error) {
	return func() (p *v1.PersistentVolumeClaim, err error) {
		p, err = kvClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil, nil
		}
		//Since https://github.com/kubernetes/client-go/issues/861 we manually add the Kind
		p.Kind = "PersistentVolumeClaim"
		return
	}
}

// ThisDV fetches the latest state of the DataVolume. If the object does not exist, nil is returned.
func ThisDV(kvClient kubecli.KubevirtClient, dv *v1beta1.DataVolume) func() (*v1beta1.DataVolume, error) {
	return ThisDVWith(kvClient, dv.Namespace, dv.Name)
}

// ThisDVWith fetches the latest state of the DataVolume based on namespace and name. If the object does not exist, nil is returned.
func ThisDVWith(kvClient kubecli.KubevirtClient, namespace string, name string) func() (*v1beta1.DataVolume, error) {
	return func() (p *v1beta1.DataVolume, err error) {
		p, err = kvClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil, nil
		}
		//Since https://github.com/kubernetes/client-go/issues/861 we manually add the Kind
		p.Kind = "DataVolume"
		return
	}
}

func EventuallyDVWith(kvClient kubecli.KubevirtClient, namespace, name string, timeoutSec int, matcher gomegatypes.GomegaMatcher) {
	if !IsDataVolumeGC(kvClient) {
		Eventually(ThisDVWith(kvClient, namespace, name), timeoutSec, time.Second).Should(matcher)
		return
	}

	// wait PVC exists before making sure DV not nil to prevent
	// race of checking dv before it was even created
	Eventually(func() bool {
		pvc, err := ThisPVCWith(kvClient, namespace, name)()
		Expect(err).ToNot(HaveOccurred())
		return pvc != nil
	}, timeoutSec, time.Second).Should(BeTrue())

	ginkgo.By("Verifying DataVolume garbage collection")
	var dv *v1beta1.DataVolume
	Eventually(func() *v1beta1.DataVolume {
		var err error
		dv, err = ThisDVWith(kvClient, namespace, name)()
		Expect(err).ToNot(HaveOccurred())
		return dv
	}, timeoutSec, time.Second).Should(Or(BeNil(), matcher))

	if dv != nil {
		if dv.Status.Phase != v1beta1.Succeeded {
			return
		}
		if dv.Annotations["cdi.kubevirt.io/storage.deleteAfterCompletion"] == "true" {
			Eventually(ThisDV(kvClient, dv), timeoutSec).Should(BeNil())
		}
	}

	Eventually(func() bool {
		pvc, err := ThisPVCWith(kvClient, namespace, name)()
		Expect(err).ToNot(HaveOccurred())
		return pvc != nil && pvc.Spec.VolumeName != ""
	}, timeoutSec, time.Second).Should(BeTrue())
}
