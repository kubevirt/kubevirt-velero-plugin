package framework

import (
	"fmt"

	"github.com/onsi/ginkgo"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

const (
	alpineUrl               = "docker://quay.io/kubevirt/alpine-container-disk-demo:v0.57.1"
	alpineWithGuestAgentUrl = "docker://quay.io/kubevirt/alpine-with-test-tooling-container-disk:v0.57.1"
)

// DISKS for VMS
func NewDataVolumeForVmWithGuestAgentImage(dataVolumeName string, storageClass string) *cdiv1.DataVolume {
	nodePullMethod := cdiv1.RegistryPullNode
	containerDiskUrl := alpineWithGuestAgentUrl

	dvSpec := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL:        &containerDiskUrl,
					PullMethod: &nodePullMethod,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}
	if storageClass != "" {
		dvSpec.Spec.PVC.StorageClassName = &storageClass
	}

	return dvSpec
}

func NewDataVolumeForBlankRawImage(dataVolumeName, size string, storageClass string) *cdiv1.DataVolume {
	dvSpec := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceStorage: resource.MustParse(size),
					},
				},
			},
		},
	}
	if storageClass != "" {
		dvSpec.Spec.PVC.StorageClassName = &storageClass
	}

	return dvSpec
}

// VMs
func CreateVmWithoutGuestAgent(vmName string, storageClassName string) *v1.VirtualMachine {
	return CreateVm(vmName, storageClassName, alpineUrl)
}

func CreateVmWithGuestAgent(vmName string, storageClassName string) *v1.VirtualMachine {
	return CreateVm(vmName, storageClassName, alpineWithGuestAgentUrl)
}

func CreateVm(vmName string, storageClassName string, containerDiskUrl string) *v1.VirtualMachine {
	no := false
	var zero int64 = 0
	dataVolumeName := vmName + "-dv"
	size := "1Gi"

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

	vmiSpec := v1.VirtualMachineInstanceSpec{
		Domain: v1.DomainSpec{
			Resources: v1.ResourceRequirements{
				Requests: k8sv1.ResourceList{
					k8sv1.ResourceName(k8sv1.ResourceMemory): resource.MustParse("256M"),
				},
			},
			Machine: &v1.Machine{
				Type: "q35",
			},
			Devices: v1.Devices{
				Rng: &v1.Rng{},
				Disks: []v1.Disk{
					{
						Name: "volume0",
						DiskDevice: v1.DiskDevice{
							Disk: &v1.DiskTarget{
								Bus: "virtio",
							},
						},
					},
					{
						Name: "volume1",
						DiskDevice: v1.DiskDevice{
							Disk: &v1.DiskTarget{
								Bus: "virtio",
							},
						},
					},
				},
				Interfaces: []v1.Interface{{
					Name: "default",
					InterfaceBindingMethod: v1.InterfaceBindingMethod{
						Masquerade: &v1.InterfaceMasquerade{},
					},
				}},
			},
		},
		Networks: []v1.Network{{
			Name: "default",
			NetworkSource: v1.NetworkSource{
				Pod: &v1.PodNetwork{},
			},
		}},
		Volumes: []v1.Volume{
			{
				Name: "volume0",
				VolumeSource: v1.VolumeSource{
					DataVolume: &v1.DataVolumeSource{
						Name: dataVolumeName,
					},
				},
			},
			{
				Name: "volume1",
				VolumeSource: v1.VolumeSource{
					CloudInitNoCloud: &v1.CloudInitNoCloudSource{
						NetworkData: networkData,
					},
				},
			},
		},
		TerminationGracePeriodSeconds: &zero,
	}

	nodePullMethod := cdiv1.RegistryPullNode

	vmSpec := &v1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmName,
		},
		Spec: v1.VirtualMachineSpec{
			Running: &no,
			Template: &v1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: vmName,
				},
				Spec: vmiSpec,
			},
			DataVolumeTemplates: []v1.DataVolumeTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: dataVolumeName,
					},
					Spec: cdiv1.DataVolumeSpec{
						Source: &cdiv1.DataVolumeSource{
							Registry: &cdiv1.DataVolumeSourceRegistry{
								URL:        &containerDiskUrl,
								PullMethod: &nodePullMethod,
							},
						},
						PVC: &k8sv1.PersistentVolumeClaimSpec{
							AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
							Resources: k8sv1.ResourceRequirements{
								Requests: k8sv1.ResourceList{
									k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
								},
							},
						},
					},
				},
			},
		},
	}
	if storageClassName != "" {
		vmSpec.Spec.DataVolumeTemplates[0].Spec.PVC.StorageClassName = &storageClassName
	}
	return vmSpec
}

func CreateVirtualMachineFromDefinition(client kubecli.KubevirtClient, namespace string, def *v1.VirtualMachine) (*v1.VirtualMachine, error) {
	var virtualMachine *v1.VirtualMachine
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		virtualMachine, err = client.VirtualMachine(namespace).Create(def)
		if err == nil || errors.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return virtualMachine, nil
}

func CreateVirtualMachineInstanceFromDefinition(client kubecli.KubevirtClient, namespace string, def *v1.VirtualMachineInstance) (*v1.VirtualMachineInstance, error) {
	var virtualMachineInstance *v1.VirtualMachineInstance
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		var err error
		virtualMachineInstance, err = client.VirtualMachineInstance(namespace).Create(def)
		if err == nil || errors.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return virtualMachineInstance, nil
}

func DeleteVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		propagationForeground := metav1.DeletePropagationForeground
		err := client.VirtualMachine(namespace).Delete(name, &metav1.DeleteOptions{
			PropagationPolicy: &propagationForeground,
		})
		if err == nil || errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func DeleteVirtualMachineInstance(client kubecli.KubevirtClient, namespace, name string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		err := client.VirtualMachineInstance(namespace).Delete(name, &metav1.DeleteOptions{})
		if err == nil || errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func WaitForVirtualMachineInstanceCondition(client kubecli.KubevirtClient, namespace, name string, conditionType v1.VirtualMachineInstanceConditionType) (bool, error) {
	ginkgo.By(fmt.Sprintf("Waiting for %s condition", conditionType))
	var result bool

	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vmi, err := client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, condition := range vmi.Status.Conditions {
			if condition.Type == conditionType && condition.Status == k8sv1.ConditionTrue {
				result = true

				ginkgo.By(fmt.Sprintf(" got %s", conditionType))
				return true, nil
			}
		}

		return false, nil
	})

	return result, err
}

func WaitForVirtualMachineInstancePhase(client kubecli.KubevirtClient, namespace, name string, phase v1.VirtualMachineInstancePhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vmi, err := client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		ginkgo.By(fmt.Sprintf("INFO: Waiting for status %s, got %s", phase, vmi.Status.Phase))
		return vmi.Status.Phase == phase, nil
	})

	return err
}

func WaitForVirtualMachineStatus(client kubecli.KubevirtClient, namespace, name string, statuses ...v1.VirtualMachinePrintableStatus) error {
	ginkgo.By(fmt.Sprintf("Waiting for any of %s statuses", statuses))

	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vm, err := client.VirtualMachine(namespace).Get(name, &metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		for _, status := range statuses {
			if vm.Status.PrintableStatus == status {
				ginkgo.By(fmt.Sprintf(" got %s", status))

				return true, nil
			}
		}

		return false, nil
	})

	return err
}

func WaitForVirtualMachineInstanceStatus(client kubecli.KubevirtClient, namespace, name string, phase v1.VirtualMachineInstancePhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		vm, err := client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
		if errors.IsNotFound(err) {
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
			if errors.IsNotFound(err) {
				result = true
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	return result, err
}

func StartVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return client.VirtualMachine(namespace).Start(name, &v1.StartOptions{})
}

func PauseVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return client.VirtualMachineInstance(namespace).Pause(name)
}

func StopVirtualMachine(client kubecli.KubevirtClient, namespace, name string) error {
	return client.VirtualMachine(namespace).Stop(name)
}

func GetVirtualMachine(client kubecli.KubevirtClient, namespace, name string) (*v1.VirtualMachine, error) {
	return client.VirtualMachine(namespace).Get(name, &metav1.GetOptions{})
}

func GetVirtualMachineInstance(client kubecli.KubevirtClient, namespace, name string) (*v1.VirtualMachineInstance, error) {
	return client.VirtualMachineInstance(namespace).Get(name, &metav1.GetOptions{})
}
