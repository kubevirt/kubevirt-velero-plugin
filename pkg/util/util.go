package util

import (
	"context"

	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1api "k8s.io/api/core/v1"
	k8score "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kvv1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const VELERO_EXCLUDE_LABEL = "velero.io/exclude-from-backup"

func GetK8sClient() (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return client, nil
}

func GetKubeVirtclient() (*kubecli.KubevirtClient, error) {
	kubeConfig := os.Getenv("KUBECONFIG")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}
	kubevirtClient, err := kubecli.GetKubevirtClientFromRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &kubevirtClient, nil
}

func IsResourceIncluded(resourceKind string, backup *velerov1.Backup) bool {
	if len(backup.Spec.IncludedResources) == 0 {
		// Not a "--include-resources" backup, assume the resource is included
		return true
	}

	for _, res := range backup.Spec.IncludedResources {
		if equalIgnorePlural(res, resourceKind) {
			return true
		}
	}

	return false
}

func IsResourceExcluded(resourceKind string, backup *velerov1.Backup) bool {
	if len(backup.Spec.ExcludedResources) == 0 {
		// Not a "--exclude-resources" backup, assume the resource is included
		return false
	}

	for _, res := range backup.Spec.ExcludedResources {
		if equalIgnorePlural(res, resourceKind) {
			return true
		}
	}

	return false
}

func equalIgnorePlural(str1, str2 string) bool {
	if strings.EqualFold(str1, str2) {
		return true
	}
	if strings.EqualFold(str1+"s", str2) {
		return true
	}
	if strings.EqualFold(str1, str2+"s") {
		return true
	}

	return false
}

func IsResourceInBackup(resourceKind string, backup *velerov1.Backup) bool {
	return IsResourceIncluded(resourceKind, backup) && !IsResourceExcluded(resourceKind, backup)
}

func AddAnnotation(item runtime.Unstructured, annotation, value string) {
	metadata, err := meta.Accessor(item)
	if err != nil {
		return
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[annotation] = value

	metadata.SetAnnotations(annotations)
}

func IsVMIPaused(vmi *kvv1.VirtualMachineInstance) bool {
	for _, c := range vmi.Status.Conditions {
		if c.Type == kvv1.VirtualMachineInstancePaused && c.Status == k8score.ConditionTrue {
			return true
		}
	}

	return false
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var GetPVC = func(ns, name string) (*corev1api.PersistentVolumeClaim, error) {
	client, err := GetK8sClient()
	if err != nil {
		return nil, err
	}

	pvc, err := (*client).CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get PVC %s/%s", ns, name)
	}

	return pvc, nil
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
	client, err := GetKubeVirtclient()
	if err != nil {
		return nil, err
	}

	dv, err := (*client).CdiClient().CdiV1beta1().DataVolumes(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get DV %s/%s", ns, name)
	}

	return dv, nil
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var IsDVExcludedByLabel = func(namespace, dvName string) (bool, error) {
	client, err := GetKubeVirtclient()
	if err != nil {
		return false, err
	}

	dv, err := (*client).CdiClient().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dvName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	labels := dv.GetLabels()
	if labels == nil {
		return false, nil
	}

	label, ok := labels[VELERO_EXCLUDE_LABEL]
	return ok && label == "true", nil
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var IsPVCExcludedByLabel = func(namespace, pvcName string) (bool, error) {
	client, err := GetK8sClient()
	if err != nil {
		return false, err
	}

	pvc, err := (*client).CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	labels := pvc.GetLabels()
	if labels == nil {
		return false, nil
	}

	label, ok := labels[VELERO_EXCLUDE_LABEL]
	return ok && label == "true", nil
}

// RestorePossible returns false in cases when restoring a VM would not be possible due to missing objects
func RestorePossible(volumes []kvv1.Volume, backup *velerov1.Backup, namespace string, skipVolume func(volume kvv1.Volume) bool, log logrus.FieldLogger) (bool, error) {
	// Restore will not be possible if a DV or PVC volume outside VM's DVTemplates is not backed up
	for _, volume := range volumes {
		if volume.VolumeSource.DataVolume != nil && !skipVolume(volume) {
			if !IsResourceInBackup("datavolume", backup) {
				log.Infof("DataVolume not included in backup")
				return false, nil
			}

			excluded, err := IsDVExcludedByLabel(namespace, volume.VolumeSource.DataVolume.Name)
			if err != nil {
				return false, err
			}
			if excluded {
				log.Infof("DataVolume not included in backup")
				return false, nil
			}
		}

		if volume.VolumeSource.PersistentVolumeClaim != nil {
			if !IsResourceInBackup("persistentvolumeclaims", backup) {
				log.Infof("PVC not included in backup")
				return false, nil
			}

			excluded, err := IsPVCExcludedByLabel(namespace, volume.VolumeSource.PersistentVolumeClaim.ClaimName)
			if err != nil {
				return false, err
			}
			if excluded {
				log.Infof("PVC not included in backup")
				return false, nil
			}
		}
		// TODO: what about other types of volumes?
	}

	return true, nil
}
