package util

import (
	"context"

	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	corev1api "k8s.io/api/core/v1"
	k8score "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kvv1 "kubevirt.io/api/core/v1"
	v1 "kubevirt.io/api/core/v1"
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

func AddAnnotation(obj metav1.Object, annotation, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[annotation] = value
	obj.SetAnnotations(annotations)
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

func addVolumes(volumes []kvv1.Volume, namespace string, extra []velero.ResourceIdentifier, log logrus.FieldLogger) []velero.ResourceIdentifier {
	for _, volume := range volumes {
		if volume.DataVolume != nil {
			log.Infof("Adding dataVolume %s to the backup", volume.DataVolume.Name)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
				Namespace:     namespace,
				Name:          volume.DataVolume.Name,
			})
			// Add also the data volume PVC here in case the DV was already garbage collected
			log.Infof("Adding PVC %s to the backup", volume.DataVolume.Name)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.PersistentVolumeClaims,
				Namespace:     namespace,
				Name:          volume.DataVolume.Name,
			})
		} else if volume.PersistentVolumeClaim != nil {
			log.Infof("Adding PVC %s to the backup", volume.PersistentVolumeClaim.ClaimName)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.PersistentVolumeClaims,
				Namespace:     namespace,
				Name:          volume.PersistentVolumeClaim.ClaimName,
			})
		} else if volume.MemoryDump != nil {
			log.Infof("Adding MemoryDump %s to the backup", volume.MemoryDump.ClaimName)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.PersistentVolumeClaims,
				Namespace:     namespace,
				Name:          volume.MemoryDump.ClaimName,
			})
		} else if volume.ConfigMap != nil {
			log.Infof("Adding config map %s to the backup", volume.ConfigMap.Name)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: schema.GroupResource{Group: "", Resource: "configmaps"},
				Namespace:     namespace,
				Name:          volume.ConfigMap.Name,
			})
		} else if volume.Secret != nil {
			log.Infof("Adding secret %s to the backup", volume.Secret.SecretName)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.Secrets,
				Namespace:     namespace,
				Name:          volume.Secret.SecretName,
			})
		} else if volume.ServiceAccount != nil {
			log.Infof("Adding service account %s to the backup", volume.ServiceAccount.ServiceAccountName)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.ServiceAccounts,
				Namespace:     namespace,
				Name:          volume.ServiceAccount.ServiceAccountName,
			})
		}
	}

	return extra
}

func getNamespaceAndNetworkName(vmiNamespace, fullNetworkName string) (string, string) {
	if strings.Contains(fullNetworkName, "/") {
		res := strings.SplitN(fullNetworkName, "/", 2)
		return res[0], res[1]
	}
	return vmiNamespace, fullNetworkName
}

func addAccessCredentials(acs []kvv1.AccessCredential, namespace string, extra []velero.ResourceIdentifier, log logrus.FieldLogger) []velero.ResourceIdentifier {
	for _, ac := range acs {
		if ac.SSHPublicKey != nil && ac.SSHPublicKey.Source.Secret != nil {
			log.Infof("Adding sshPublicKey secret %s to the backup", ac.SSHPublicKey.Source.Secret.SecretName)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.Secrets,
				Namespace:     namespace,
				Name:          ac.SSHPublicKey.Source.Secret.SecretName,
			})
		} else if ac.UserPassword != nil && ac.UserPassword.Source.Secret != nil {
			log.Infof("Adding userpassword secret %s to the backup", ac.UserPassword.Source.Secret.SecretName)
			extra = append(extra, velero.ResourceIdentifier{
				GroupResource: kuberesource.Secrets,
				Namespace:     namespace,
				Name:          ac.UserPassword.Source.Secret.SecretName,
			})
		}
	}
	return extra
}

func AddVMIObjectGraph(spec v1.VirtualMachineInstanceSpec, namespace string, extra []velero.ResourceIdentifier, log logrus.FieldLogger) []velero.ResourceIdentifier {
	extra = addVolumes(spec.Volumes, namespace, extra, log)

	extra = addAccessCredentials(spec.AccessCredentials, namespace, extra, log)

	return extra

}

func IsRestoringToDifferentNamespace(restore *velerov1.Restore, obj metav1.Object) bool {
	for source, target := range restore.Spec.NamespaceMapping {
		if source == obj.GetNamespace() && target != obj.GetNamespace() {
			return true
		}
	}
	return false
}
