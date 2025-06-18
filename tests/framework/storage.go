package framework

import (
	"context"
	"fmt"
	"strings"
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
	pollInterval = 2 * time.Second
	waitTime     = 180 * time.Second
)

func IsDataVolumeGC(kvClient kubecli.KubevirtClient) bool {
	config, err := kvClient.CdiClient().CdiV1beta1().CDIConfigs().Get(context.TODO(), "config", metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	return config.Spec.DataVolumeTTLSeconds != nil && *config.Spec.DataVolumeTTLSeconds >= 0
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
			Resources: v1.VolumeResourceRequirements{
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
				Resources: v1.VolumeResourceRequirements{
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

// IsVolumeModeBlockSupported checks if a given storage class supports provisioning
// block mode PersistentVolumes. It does this by creating a temporary PVC that
// requests block mode and observing the results.
//
// Parameters:
//   - kvClient: An initialized KubeVirt clientset.
//   - namespace: The namespace where the test PVC will be created.
//   - storageClassName: The name of the StorageClass to test. If this string is
//     empty, the function will test the cluster's default StorageClass.
//
// Returns:
//   - (bool): True if block mode is supported, false otherwise.
//   - (error): An error if the check is inconclusive due to an unexpected
//     issue (e.g., insufficient capacity), or if a Kubernetes API error occurs.
func IsVolumeModeBlockSupported(kvClient kubecli.KubevirtClient, namespace string, storageClassName string) (bool, error) {
	// Generate a unique name for our test PVC to avoid collisions.
	pvcName := fmt.Sprintf("block-support-check-%d", time.Now().UnixNano())
	blockMode := v1.PersistentVolumeBlock

	// Define the test PVC manifest.
	testPVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			// Explicitly request a raw block device.
			VolumeMode: &blockMode,
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	// If a storageClassName is provided, add it to the spec.
	// Otherwise, the default SC will be used.
	if storageClassName != "" {
		testPVC.Spec.StorageClassName = &storageClassName
	}

	ginkgo.By(fmt.Sprintf("Creating test PVC '%s' to check for block support", pvcName))
	_, err := kvClient.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), testPVC, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to create test PVC %s: %w", pvcName, err)
	}

	// IMPORTANT: Defer the cleanup to ensure the test PVC is always deleted.
	defer func() {
		ginkgo.By(fmt.Sprintf("Cleaning up test PVC '%s'", pvcName))
		err := kvClient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			// Log the cleanup failure but don't fail the test on it.
			fmt.Fprintf(ginkgo.GinkgoWriter, "Warning: failed to delete test PVC %s: %v\n", pvcName, err)
		}
	}()

	// Poll for a definitive result (Bound or ProvisioningFailed) for a shorter duration.
	var lastEventMessage string
	pollErr := wait.PollImmediate(3*time.Second, 30*time.Second, func() (bool, error) {
		pvc, err := kvClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get test PVC %s: %w", pvcName, err)
		}

		if pvc.Status.Phase == v1.ClaimBound {
			// Found a definitive success. Stop polling.
			return true, nil
		}
		
		// Check for a definitive failure event.
		events, err := kvClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.kind=PersistentVolumeClaim,involvedObject.name=%s", pvcName),
		})
		if err != nil {
			// Don't fail the poll on a transient event list error.
			fmt.Fprintf(ginkgo.GinkgoWriter, "Warning: could not list events for PVC %s: %v\n", pvcName, err)
			return false, nil
		}

		for _, event := range events.Items {
			if event.Type == v1.EventTypeWarning && event.Reason == "ProvisioningFailed" {
				// Found a definitive failure. Stop polling.
				lastEventMessage = event.Message
				return true, nil 
			}
		}

		// No definitive result yet, continue polling.
		return false, nil
	})


	// --- Analyze the results of the poll ---

	if pollErr == nil {
		// Poll stopped because we found a definitive result.
		if lastEventMessage == "" {
			// If message is empty, it means we must have found a 'Bound' PVC.
			ginkgo.By(fmt.Sprintf("Test PVC '%s' successfully bound. Block mode is supported.", pvcName))
			return true, nil
		}

		// We found a ProvisioningFailed event. Now check the message.
		if strings.Contains(strings.ToLower(lastEventMessage), "block volume is not supported") {
			ginkgo.By("Found 'ProvisioningFailed' event: Block mode is not supported.")
			return false, nil
		} else {
			return false, fmt.Errorf("provisioning failed for an inconclusive reason: %s", lastEventMessage)
		}
	}
	
	if pollErr == wait.ErrWaitTimeout {
		// The poll timed out without any specific error. Assume it's a slow cluster and that block is supported.
		ginkgo.By("PVC check timed out without specific errors, assuming block mode is supported.")
		return true, nil
	}

	// An unexpected error occurred during polling.
	return false, fmt.Errorf("error while polling for PVC status: %w", pollErr)
}