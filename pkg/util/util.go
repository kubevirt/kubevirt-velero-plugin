package util

import (
	"context"

	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1api "k8s.io/api/core/v1"
	k8score "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kvv1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	// MetadataBackupLabel indicates that the object will be backed up for metadata purposes.
	// This allows skipping restore and consistency-specific checks while ensuring the object is backed up.
	MetadataBackupLabel = "velero.kubevirt.io/metadataBackup"

	// RestoreRunStrategy indicates that the backed up VMs will be powered with the specified run strategy after restore.
	RestoreRunStrategy = "velero.kubevirt.io/restore-run-strategy"

	// ClearMacAddressLabel indicates that the MAC address should be cleared as part of the restore workflow.
	ClearMacAddressLabel = "velero.kubevirt.io/clear-mac-address"

	// GenerateNewFirmwareUUIDLabel indicates that a new firmware UUID should be generated for VMs as part of the restore workflow.
	GenerateNewFirmwareUUIDLabel = "velero.kubevirt.io/generate-new-firmware-uuid"

	// VeleroExcludeLabel is used to exclude an object from Velero backups.
	VeleroExcludeLabel = "velero.io/exclude-from-backup"

	// Resource UID labeling constants for selective restore
	PVCUIDLabel = "velero.kubevirt.io/pvc-uid"

	// Collision detection annotations to preserve original values
	OriginalPVCUIDAnnotation = "velero.kubevirt.io/original-pvc-uid"
	OriginalVolumeSnapshotUIDAnnotation = "velero.kubevirt.io/original-volumesnapshot-uid"
)

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

	return client, nil
}

func GetLauncherPod(vmiName, vmiNamespace string) (*k8score.Pod, error) {
	pods, err := ListPods(vmiName, vmiNamespace)
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if pod.Annotations["kubevirt.io/domain"] == vmiName {
			return &pod, nil
		}
	}

	return nil, nil
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
		gr := schema.ParseGroupResource(res)
		if equalIgnorePlural(gr.Resource, resourceKind) {
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
		gr := schema.ParseGroupResource(res)
		if equalIgnorePlural(gr.Resource, resourceKind) {
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
var ListPods = func(name, ns string) (*corev1api.PodList, error) {
	client, err := GetK8sClient()
	if err != nil {
		return nil, err
	}

	pods, err := client.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "kubevirt.io=virt-launcher",
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get launcher pod from VMI %s/%s", ns, name)
	}

	return pods, nil
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
var ListPVCs = func(labelSelector, namespace string) (*corev1api.PersistentVolumeClaimList, error) {
	client, err := GetK8sClient()
	if err != nil {
		return nil, err
	}

	pvcs, err := (*client).CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	return pvcs, nil
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
	client, err := GetKubeVirtclient()
	if err != nil {
		return nil, err
	}

	dv, err := (*client).CdiClient().CdiV1beta1().DataVolumes(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "failed to get DV %s/%s", ns, name)
	}

	return dv, nil
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var IsDVExcludedByLabel = func(namespace, dvName string) (bool, error) {
	dv, err := GetDV(namespace, dvName)
	if err != nil {
		return false, err
	}

	labels := dv.GetLabels()
	if labels == nil {
		return false, nil
	}

	label, ok := labels[VeleroExcludeLabel]
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

	label, ok := labels[VeleroExcludeLabel]
	return ok && label == "true", nil
}

func checkRestoreDataVolumePossible(backup *velerov1.Backup, namespace, name string) (bool, error) {
	// IsDVExcludedByLabel first checks if DV exists
	// If not no use of checking restore of DV
	excluded, err := IsDVExcludedByLabel(namespace, name)
	if err != nil {
		return false, err
	}
	if excluded {
		return false, nil
	}

	if !IsResourceInBackup("datavolume", backup) {
		return false, nil
	}
	return true, nil
}

func checkRestorePVCPossible(backup *velerov1.Backup, namespace, claimName string) (bool, error) {
	if !IsResourceInBackup("persistentvolumeclaims", backup) {
		return false, nil
	}

	excluded, err := IsPVCExcludedByLabel(namespace, claimName)
	if err != nil {
		return false, err
	}
	if excluded {
		return false, nil
	}

	return true, nil
}

// RestorePossible returns false in cases when restoring a VM would not be possible due to missing objects
func RestorePossible(volumes []kvv1.Volume, backup *velerov1.Backup, namespace string, skipVolume func(volume kvv1.Volume) bool, log logrus.FieldLogger) (bool, error) {
	// Restore will not be possible if a DV or PVC volume outside VM's DVTemplates is not backed up
	for _, volume := range volumes {
		if volume.VolumeSource.DataVolume != nil && !skipVolume(volume) {
			possible, err := checkRestoreDataVolumePossible(backup, namespace, volume.VolumeSource.DataVolume.Name)
			if k8serrors.IsNotFound(err) {
				// If DV doesnt exist check that the related PVC exists
				// and can be backed up
				possible, err = checkRestorePVCPossible(backup, namespace, volume.VolumeSource.DataVolume.Name)
				if err != nil || !possible {
					log.Infof("PVC of DV volume source %s not included in backup", volume.VolumeSource.DataVolume.Name)
					return possible, err
				}
			} else if err != nil || !possible {
				log.Infof("DataVolume %s not included in backup", volume.VolumeSource.DataVolume.Name)
				return possible, err
			}
		}

		if volume.VolumeSource.PersistentVolumeClaim != nil {
			possible, err := checkRestorePVCPossible(backup, namespace, volume.VolumeSource.PersistentVolumeClaim.ClaimName)
			if err != nil || !possible {
				log.Infof("PVC %s not included in backup", volume.VolumeSource.PersistentVolumeClaim.ClaimName)
				return possible, err
			}
		}
		// TODO: what about other types of volumes?
	}

	return true, nil
}

func getNamespaceAndNetworkName(vmiNamespace, fullNetworkName string) (string, string) {
	if strings.Contains(fullNetworkName, "/") {
		res := strings.SplitN(fullNetworkName, "/", 2)
		return res[0], res[1]
	}
	return vmiNamespace, fullNetworkName
}

func GetRestoreRunStrategy(restore *velerov1.Restore) (kvv1.VirtualMachineRunStrategy, bool) {
	if metav1.HasLabel(restore.ObjectMeta, RestoreRunStrategy) {
		return kvv1.VirtualMachineRunStrategy(restore.Labels[RestoreRunStrategy]), true
	}
	return "", false
}

func IsMetadataBackup(backup *velerov1.Backup) bool {
	return metav1.HasLabel(backup.ObjectMeta, MetadataBackupLabel)
}

func ShouldClearMacAddress(restore *velerov1.Restore) bool {
	return metav1.HasLabel(restore.ObjectMeta, ClearMacAddressLabel)
}

func ClearMacAddress(vmiSpec *kvv1.VirtualMachineInstanceSpec) {
	for i := 0; i < len(vmiSpec.Domain.Devices.Interfaces); i++ {
		vmiSpec.Domain.Devices.Interfaces[i].MacAddress = ""
	}
}

func ShouldGenerateNewFirmwareUUID(restore *velerov1.Restore) bool {
	return metav1.HasLabel(restore.ObjectMeta, GenerateNewFirmwareUUIDLabel)
}

// GenerateNewFirmwareUUID generates a new random firmware UUID for the restored VM
func GenerateNewFirmwareUUID(vmiSpec *kvv1.VirtualMachineInstanceSpec, name, namespace, uid string) {
	if vmiSpec.Domain.Firmware == nil {
		vmiSpec.Domain.Firmware = &kvv1.Firmware{}
	}
	vmiSpec.Domain.Firmware.UUID = types.UID(uuid.New().String())
}
