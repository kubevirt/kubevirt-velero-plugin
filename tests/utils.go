package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/pkg/errors"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	kvv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

const (
	pollInterval = 3 * time.Second
	waitTime     = 270 * time.Second
	veleroCLI    = "velero"
)

func CreateVmWithGuestAgent(vmName string) *kvv1.VirtualMachine {
	no := false
	var zero int64 = 0
	dataVolumeName := vmName + "-dv"
	size := "5Gi"

	networkData := `ethernets:
  eth0:
    addresses:
    - fd10:0:2::2/120
    dhcp4: true
    gateway6: fd10:0:2::1
    match: {}
    nameservers:
      addresses:
      - 10.96.0.10
      search:
      - default.svc.cluster.local
      - svc.cluster.local
      - cluster.local
version: 2`

	vmiSpec := kvv1.VirtualMachineInstanceSpec{
		Domain: kvv1.DomainSpec{
			Resources: kvv1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceMemory): resource.MustParse("256M"),
				},
			},
			Machine: &kvv1.Machine{
				Type: "q35",
			},
			Devices: kvv1.Devices{
				Rng: &kvv1.Rng{},
				Disks: []kvv1.Disk{
					{
						Name: "volume0",
						DiskDevice: kvv1.DiskDevice{
							Disk: &kvv1.DiskTarget{
								Bus: "virtio",
							},
						},
					},
					{
						Name: "volume1",
						DiskDevice: kvv1.DiskDevice{
							Disk: &kvv1.DiskTarget{
								Bus: "virtio",
							},
						},
					},
				},
				Interfaces: []kvv1.Interface{{
					Name: "default",
					InterfaceBindingMethod: kvv1.InterfaceBindingMethod{
						Masquerade: &kvv1.InterfaceMasquerade{},
					},
				}},
			},
		},
		Networks: []kvv1.Network{{
			Name: "default",
			NetworkSource: kvv1.NetworkSource{
				Pod: &kvv1.PodNetwork{},
			},
		}},
		Volumes: []kvv1.Volume{
			{
				Name: "volume0",
				VolumeSource: kvv1.VolumeSource{
					DataVolume: &kvv1.DataVolumeSource{
						Name: dataVolumeName,
					},
				},
			},
			{
				Name: "volume1",
				VolumeSource: kvv1.VolumeSource{
					CloudInitNoCloud: &kvv1.CloudInitNoCloudSource{
						NetworkData: networkData,
					},
				},
			},
		},
		TerminationGracePeriodSeconds: &zero,
	}

	fedoraUrl := "docker://quay.io/kubevirt/fedora-with-test-tooling-container-disk"
	nodePullMethod := cdiv1.RegistryPullNode

	return &kvv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmName,
		},
		Spec: kvv1.VirtualMachineSpec{
			Running: &no,
			Template: &kvv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: vmName,
				},
				Spec: vmiSpec,
			},
			DataVolumeTemplates: []kvv1.DataVolumeTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: dataVolumeName,
					},
					Spec: cdiv1.DataVolumeSpec{
						Source: &cdiv1.DataVolumeSource{
							Registry: &cdiv1.DataVolumeSourceRegistry{
								URL:        &fedoraUrl,
								PullMethod: &nodePullMethod,
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
				},
			},
		},
	}
}

func CreateDataVolumeFromDefinition(clientSet *cdiclientset.Clientset, namespace string, def *cdiv1.DataVolume) (*cdiv1.DataVolume, error) {
	var dataVolume *cdiv1.DataVolume
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		dataVolume, err = clientSet.CdiV1beta1().DataVolumes(namespace).Create(context.TODO(), def, metav1.CreateOptions{})
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

func CreateVirtualMachineFromDefinition(client kubecli.KubevirtClient, namespace string, def *kvv1.VirtualMachine) (*kvv1.VirtualMachine, error) {
	var virtualMachine *kvv1.VirtualMachine
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		virtualMachine, err = client.VirtualMachine(namespace).Create(def)
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return virtualMachine, nil
}

func CreateVirtualMachineInstanceFromDefinition(client kubecli.KubevirtClient, namespace string, def *kvv1.VirtualMachineInstance) (*kvv1.VirtualMachineInstance, error) {
	var virtualMachineInstance *kvv1.VirtualMachineInstance
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		virtualMachineInstance, err = client.VirtualMachineInstance(namespace).Create(def)
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return virtualMachineInstance, nil
}

func CreateNamespace(client *kubernetes.Clientset) (*v1.Namespace, error) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kvp-e2e-tests-",
			Namespace:    "",
		},
		Status: v1.NamespaceStatus{},
	}

	var nsObj *v1.Namespace
	err := wait.PollImmediate(2*time.Second, waitTime, func() (bool, error) {
		var err error
		nsObj, err = client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil // done
		}
		klog.Warningf("Unexpected error while creating %q namespace: %v", ns.GenerateName, err)
		return false, err // keep trying
	})
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Created new namespace %q\n", nsObj.Name)
	return nsObj, nil
}

// FindPVC Finds the passed in PVC
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

func FindDataVolume(clientSet *cdiclientset.Clientset, namespace string, dataVolumeName string) (*cdiv1.DataVolume, error) {
	return clientSet.CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
}

// WaitForDataVolumePhase waits for DV's phase to be in a particular phase (Pending, Bound, or Lost)
func WaitForDataVolumePhase(clientSet *cdiclientset.Clientset, namespace string, phase cdiv1.DataVolumePhase, dataVolumeName string) error {
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Waiting for status %s\n", phase)
	var lastPhase cdiv1.DataVolumePhase

	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		dataVolume, err := clientSet.CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		if dataVolume.Status.Phase != phase {
			if dataVolume.Status.Phase != lastPhase {
				lastPhase = dataVolume.Status.Phase
				fmt.Fprintf(ginkgo.GinkgoWriter, "\nINFO: Waiting for status %s, got %s", phase, dataVolume.Status.Phase)
			} else {
				fmt.Fprint(ginkgo.GinkgoWriter, ".")
			}
			return false, err
		}

		fmt.Fprintf(ginkgo.GinkgoWriter, "\nINFO: Waiting for status %s, got %s\n", phase, dataVolume.Status.Phase)
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("DataVolume %s not in phase %s within %v", dataVolumeName, phase, waitTime)
	}
	return nil
}

// WaitForDataVolumePhaseButNot waits for DV's phase to be in a particular phase without going through another phase
func WaitForDataVolumePhaseButNot(clientSet *cdiclientset.Clientset, namespace string, phase cdiv1.DataVolumePhase, unwanted cdiv1.DataVolumePhase, dataVolumeName string) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		dataVolume, err := clientSet.CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
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

// DeleteDataVolume deletes the DataVolume with the given name
func DeleteDataVolume(clientSet *cdiclientset.Clientset, namespace, name string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := clientSet.CdiV1beta1().DataVolumes(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func DeleteVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := client.VirtualMachine(namespace).Delete(name, &metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func DeleteVirtualMachineInstance(client kubecli.KubevirtClient, namespace, name string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := client.VirtualMachineInstance(namespace).Delete(name, &metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

// DeletePVC deletes the passed in PVC
func DeletePVC(clientSet *kubernetes.Clientset, namespace string, pvcName string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func WaitDataVolumeDeleted(clientSet *cdiclientset.Clientset, namespace, dvName string) (bool, error) {
	var result bool
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		_, err := clientSet.CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dvName, metav1.GetOptions{})
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

func WaitPVCDeleted(clientSet *kubernetes.Clientset, namespace, pvcName string) (bool, error) {
	var result bool
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		_, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
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

func WaitForVirtualMachineInstanceCondition(client kubecli.KubevirtClient, namespace, name string, conditionType kvv1.VirtualMachineInstanceConditionType) (bool, error) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "Waiting for %s condition\n", conditionType)
	var result bool

	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vmi, err := client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, condition := range vmi.Status.Conditions {
			if condition.Type == conditionType && condition.Status == v1.ConditionTrue {
				result = true

				fmt.Fprintf(ginkgo.GinkgoWriter, " got %s\n", conditionType)
				return true, nil
			}
		}

		fmt.Fprint(ginkgo.GinkgoWriter, ".")
		return false, nil
	})

	return result, err
}

func WaitForVirtualMachineInstancePhase(client kubecli.KubevirtClient, namespace, name string, phase kvv1.VirtualMachineInstancePhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vmi, err := client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Waiting for status %s, got %s\n", phase, vmi.Status.Phase)
		return vmi.Status.Phase == phase, nil
	})

	return err
}

func WaitForVirtualMachineStatus(client kubecli.KubevirtClient, namespace, name string, statuses ...kvv1.VirtualMachinePrintableStatus) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vm, err := client.VirtualMachine(namespace).Get(name, &metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		for _, status := range statuses {
			if vm.Status.PrintableStatus == status {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}

func WaitForVirtualMachineInstanceStatus(client kubecli.KubevirtClient, namespace, name string, phase kvv1.VirtualMachineInstancePhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vm, err := client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		return vm.Status.Phase == phase, nil
	})

	return err
}

func WaitVirtualMachineDeleted(client kubecli.KubevirtClient, namespace, name string) (bool, error) {
	var result bool
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		_, err := client.VirtualMachine(namespace).Get(name, &metav1.GetOptions{})
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

func NewDataVolumeForFedoraWithGuestAgentImage(dataVolumeName string) *cdiv1.DataVolume {
	fedoraUrl := "docker://quay.io/kubevirt/fedora-with-test-tooling-container-disk"
	nodePullMethod := cdiv1.RegistryPullNode

	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL:        &fedoraUrl,
					PullMethod: &nodePullMethod,
				},
			},
			PVC: &v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceStorage): resource.MustParse("5Gi"),
					},
				},
			},
		},
	}
}

func NewDataVolumeForBlankRawImage(dataVolumeName, size string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
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

func CreateBackupForNamespace(ctx context.Context, backupName string, namespace string, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", namespace,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Fprintf(ginkgo.GinkgoWriter, "backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForNamespaceExcludeNamespace(ctx context.Context, backupName, includedNamespace, excludedNamespace, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", includedNamespace,
		"--exclude-namespaces", excludedNamespace,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Fprintf(ginkgo.GinkgoWriter, "backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForNamespaceExcludeResources(ctx context.Context, backupName, namespace, resources, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", namespace,
		"--exclude-resources", resources,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Fprintf(ginkgo.GinkgoWriter, "backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForSelector(ctx context.Context, backupName, selector, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--selector", selector,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Fprintf(ginkgo.GinkgoWriter, "backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForResources(ctx context.Context, backupName, resources, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-resources", resources,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Fprintf(ginkgo.GinkgoWriter, "backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func DeleteBackup(ctx context.Context, backupName string, backupNamespace string) error {
	args := []string{
		"delete", "backup", backupName,
		"--confirm",
		"--namespace", backupNamespace,
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Fprintf(ginkgo.GinkgoWriter, "backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	return nil
}

func GetBackup(ctx context.Context, backupName string, backupNamespace string) (*velerov1api.Backup, error) {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "backup", "get", "-n", backupNamespace, "-o", "json", backupName)

	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return nil, err
	}

	jsonBuf := make([]byte, 16*1024)
	err = checkCMD.Start()
	if err != nil {
		return nil, err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if bytesRead == len(jsonBuf) {
		return nil, errors.New("json returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return nil, err
	}
	backup := velerov1api.Backup{}
	err = json.Unmarshal(jsonBuf, &backup)
	if err != nil {
		return nil, err
	}

	return &backup, nil
}

func GetBackupPhase(ctx context.Context, backupName string, backupNamespace string) (velerov1api.BackupPhase, error) {
	backup, err := GetBackup(ctx, backupName, backupNamespace)
	if err != nil {
		return "", err
	}

	return backup.Status.Phase, nil
}

func WaitForBackupPhase(ctx context.Context, backupName string, backupNamespace string, expectedPhase velerov1api.BackupPhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		backup, err := GetBackup(ctx, backupName, backupNamespace)
		if err != nil {
			return false, err
		}
		phase := backup.Status.Phase
		ginkgo.By(fmt.Sprintf("Waiting for backup phase %v, got %v", expectedPhase, phase))
		if backup.Status.CompletionTimestamp != nil && phase != expectedPhase {
			return false, errors.Errorf("Backup finished with: %v ", phase)
		}
		if phase != expectedPhase {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("backup %s not in phase %s within %v", backupName, expectedPhase, waitTime)
	}
	return nil
}

func CreateSnapshotLocation(ctx context.Context, locationName, provider, region string) error {
	args := []string{
		"snapshot-location", "create", locationName,
		"--provider", provider,
		"--config", "region=" + region,
	}

	locationCmd := exec.CommandContext(ctx, veleroCLI, args...)
	fmt.Fprintf(ginkgo.GinkgoWriter, "snapshot-location cmd =%v\n", locationCmd)

	output, err := locationCmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "already exists") {
		return err
	}

	return nil
}

func CreateRestoreForBackup(ctx context.Context, backupName, restoreName string, backupNamespace string, wait bool) error {
	args := []string{
		"restore", "create", restoreName,
		"--from-backup", backupName,
		"--namespace", backupNamespace,
	}

	if wait {
		args = append(args, "--wait")
	}

	restoreCmd := exec.CommandContext(ctx, veleroCLI, args...)
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr
	fmt.Fprintf(ginkgo.GinkgoWriter, "restore cmd =%v\n", restoreCmd)
	err := restoreCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func GetRestore(ctx context.Context, restoreName string, backupNamespace string) (*velerov1api.Restore, error) {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "restore", "get", "-n", backupNamespace, "-o", "json", restoreName)

	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return nil, err
	}

	jsonBuf := make([]byte, 16*1024)
	err = checkCMD.Start()
	if err != nil {
		return nil, err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if bytesRead == len(jsonBuf) {
		return nil, errors.New("json returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return nil, err
	}
	restore := velerov1api.Restore{}
	err = json.Unmarshal(jsonBuf, &restore)
	if err != nil {
		return nil, err
	}

	return &restore, nil
}

func GetRestorePhase(ctx context.Context, restoreName string, backupNamespace string) (velerov1api.RestorePhase, error) {
	restore, err := GetRestore(ctx, restoreName, backupNamespace)
	if err != nil {
		return "", err
	}

	return restore.Status.Phase, nil
}

func WaitForRestorePhase(ctx context.Context, restoreName string, backupNamespace string, expectedPhase velerov1api.RestorePhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		phase, err := GetRestorePhase(ctx, restoreName, backupNamespace)
		ginkgo.By(fmt.Sprintf("Waiting for restore phase %v, got %v", expectedPhase, phase))
		if err != nil || phase != expectedPhase {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("restore %s not in phase %s within %v", restoreName, expectedPhase, waitTime)
	}
	return nil
}

func StartVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return client.VirtualMachine(namespace).Start(name, &kvv1.StartOptions{})
}

func PauseVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return client.VirtualMachineInstance(namespace).Pause(name)
}

func StopVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return client.VirtualMachine(namespace).Stop(name)
}

func GetVirtualMachine(client kubecli.KubevirtClient, namespace, name string) (*kvv1.VirtualMachine, error) {
	return client.VirtualMachine(namespace).Get(name, &metav1.GetOptions{})
}

func GetVirtualMachineInstance(client kubecli.KubevirtClient, namespace, name string) (*kvv1.VirtualMachineInstance, error) {
	return client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
}

func PrintEventsForKind(client kubecli.KubevirtClient, kind, namespace, name string) {
	events, _ := client.EventsV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	for _, event := range events.Items {
		if event.Regarding.Kind == kind && event.Regarding.Name == name {
			fmt.Fprintf(ginkgo.GinkgoWriter, "  INFO: event for %s/%s: %s, %s, %s\n",
				event.Regarding.Kind, event.Regarding.Name, event.Type, event.Reason, event.Note)
		}
	}
}

func PrintEvents(client kubecli.KubevirtClient, namespace, name string) {
	events, _ := client.EventsV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	for _, event := range events.Items {
		fmt.Fprintf(ginkgo.GinkgoWriter, "  INFO: event for %s/%s: %s, %s, %s\n",
			event.Regarding.Kind, event.Regarding.Name, event.Type, event.Reason, event.Note)
	}
}
