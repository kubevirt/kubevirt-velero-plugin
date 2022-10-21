package tests

import (
	"context"
	"fmt"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
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
)

const (
	waitTime            = 600 * time.Second
	forceBindAnnotation = "cdi.kubevirt.io/storage.bind.immediate.requested"
	snapshotLocation    = ""
)

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

	ginkgo.By(fmt.Sprintf("INFO: Created new namespace %q\n", nsObj.Name))
	return nsObj, nil
}

func PrintEventsForKind(client kubecli.KubevirtClient, kind, namespace, name string) {
	events, _ := client.EventsV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	for _, event := range events.Items {
		if event.Regarding.Kind == kind && event.Regarding.Name == name {
			ginkgo.By(fmt.Sprintf("  INFO: event for %s/%s: %s, %s, %s\n",
				event.Regarding.Kind, event.Regarding.Name, event.Type, event.Reason, event.Note))
		}
	}
}

func PrintEvents(client kubecli.KubevirtClient, namespace, name string) {
	events, _ := client.EventsV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	for _, event := range events.Items {
		ginkgo.By(fmt.Sprintf("  INFO: event for %s/%s: %s, %s, %s\n",
			event.Regarding.Kind, event.Regarding.Name, event.Type, event.Reason, event.Note))
	}
}

func PodWithPvcSpec(podName, pvcName string, cmd, args []string) *v1.Pod {
	image := "busybox"
	volumeName := "pv1"

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    podName,
					Image:   image,
					Command: cmd,
					Args:    args,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: "/pvc",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: volumeName,
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

var newVMSpecBlankDVTemplate = func(vmName, size string) *kvv1.VirtualMachine {
	no := false
	var zero int64 = 0
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
				Spec: kvv1.VirtualMachineInstanceSpec{
					Domain: kvv1.DomainSpec{
						Resources: kvv1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceMemory): resource.MustParse("256M"),
							},
						},
						Machine: &kvv1.Machine{
							Type: "",
						},
						Devices: kvv1.Devices{
							Disks: []kvv1.Disk{
								{
									Name: "volume0",
									DiskDevice: kvv1.DiskDevice{
										Disk: &kvv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
						},
					},
					Volumes: []kvv1.Volume{
						{
							Name: "volume0",
							VolumeSource: kvv1.VolumeSource{
								DataVolume: &kvv1.DataVolumeSource{
									Name: vmName + "-dv",
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &zero,
				},
			},
			DataVolumeTemplates: []kvv1.DataVolumeTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: vmName + "-dv",
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
				},
			},
		},
	}
}

var newVMSpec = func(vmName, size string, volumeSource kvv1.VolumeSource) *kvv1.VirtualMachine {
	no := false
	var zero int64 = 0
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
				Spec: kvv1.VirtualMachineInstanceSpec{
					Domain: kvv1.DomainSpec{
						Resources: kvv1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceMemory): resource.MustParse("256M"),
							},
						},
						Machine: &kvv1.Machine{
							Type: "",
						},
						Devices: kvv1.Devices{
							Disks: []kvv1.Disk{
								{
									Name: "volume0",
									DiskDevice: kvv1.DiskDevice{
										Disk: &kvv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
						},
					},
					Volumes: []kvv1.Volume{
						{
							Name:         "volume0",
							VolumeSource: volumeSource,
						},
					},
					TerminationGracePeriodSeconds: &zero,
				},
			},
		},
	}
}

func addVolumeToVMI(vmi *kvv1.VirtualMachineInstance, source kvv1.VolumeSource, volumeName string) *kvv1.VirtualMachineInstance {
	vmi.Spec.Domain.Devices.Disks = append(vmi.Spec.Domain.Devices.Disks, kvv1.Disk{
		Name: volumeName,
		DiskDevice: kvv1.DiskDevice{
			Disk: &kvv1.DiskTarget{
				Bus: "virtio",
			},
		},
	})
	vmi.Spec.Volumes = append(vmi.Spec.Volumes, kvv1.Volume{
		Name:         volumeName,
		VolumeSource: source,
	})
	return vmi
}

func newVMISpec(vmiName string) *kvv1.VirtualMachineInstance {
	var zero int64 = 0

	vmi := &kvv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmiName,
		},
		Spec: kvv1.VirtualMachineInstanceSpec{
			Domain: kvv1.DomainSpec{
				Resources: kvv1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceMemory): resource.MustParse("512M"),
					},
				},
				Machine: &kvv1.Machine{
					Type: "q35",
				},
				Devices: kvv1.Devices{
					Rng:   &kvv1.Rng{},
					Disks: []kvv1.Disk{},
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
			Volumes:                       []kvv1.Volume{},
			TerminationGracePeriodSeconds: &zero,
		},
	}

	return vmi
}

func newBigVMISpecWithDV(vmiName, dvName string) *kvv1.VirtualMachineInstance {
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
	vmi := newVMISpec(vmiName)

	dvSource := kvv1.VolumeSource{
		DataVolume: &kvv1.DataVolumeSource{
			Name: dvName,
		},
	}
	networkDataSource := kvv1.VolumeSource{
		CloudInitNoCloud: &kvv1.CloudInitNoCloudSource{
			NetworkData: networkData,
		},
	}
	vmi = addVolumeToVMI(vmi, dvSource, "volume0")
	vmi = addVolumeToVMI(vmi, networkDataSource, "volume1")
	return vmi
}

func newVMISpecWithDV(vmiName, dvName string) *kvv1.VirtualMachineInstance {
	vmi := newVMISpec(vmiName)

	source := kvv1.VolumeSource{
		DataVolume: &kvv1.DataVolumeSource{
			Name: dvName,
		},
	}
	vmi = addVolumeToVMI(vmi, source, "volume0")
	return vmi
}

func newVMISpecWithPVC(vmiName, pvcName string) *kvv1.VirtualMachineInstance {
	vmi := newVMISpec(vmiName)

	source := kvv1.VolumeSource{
		PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcName,
		},
	}
	vmi = addVolumeToVMI(vmi, source, "volume0")
	return vmi
}
