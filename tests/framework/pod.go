package framework

import (
	"context"
	"fmt"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/kubecli"
)

const (
	busyboxImage = "quay.io/quay/busybox:latest"
)

func PodWithPvcSpec(podName, pvcName string, cmd, args []string) *v1.Pod {
	volumeName := "pv1"
	// uid is used as the qemu group in fsGroup
	const uid int64 = 107

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				"cdi.kubevirt.io/testing": podName,
			},
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				FSGroup: ptr.To[int64](uid),
			},
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    podName,
					Image:   busyboxImage,
					Command: cmd,
					Args:    args,
					Resources: v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
						Requests: map[v1.ResourceName]resource.Quantity{
							v1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: "/pvc",
						},
					},
					SecurityContext: &v1.SecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](uid),
						RunAsGroup:   ptr.To[int64](uid),
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{
								"ALL",
							},
						},
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
						AllowPrivilegeEscalation: ptr.To(false),
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

func RunPodAndWaitPhase(kvClient kubecli.KubevirtClient, namespace string, podSpec *v1.Pod, expectedPhase v1.PodPhase) *v1.Pod {
	ginkgo.By("creating a pod")
	pod, err := kvClient.CoreV1().Pods(namespace).Create(context.Background(), podSpec, metav1.CreateOptions{})
	Expect(err).WithOffset(1).ToNot(HaveOccurred())

	ginkgo.By("Wait for pod to reach a completed phase")
	Eventually(func() v1.PodPhase {
		updatedPod, err := kvClient.CoreV1().Pods(namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "Failed getting pod phase: %s\n", err)
			return v1.PodUnknown
		}
		return updatedPod.Status.Phase
	}, 2*time.Minute, 5*time.Second).WithOffset(1).Should(Equal(expectedPhase))

	return pod
}

func FindLauncherPod(client *kubernetes.Clientset, namespace string, vmName string) v1.Pod {
	var pod v1.Pod
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "kubevirt.io=virt-launcher",
	})
	Expect(err).WithOffset(1).ToNot(HaveOccurred())
	for _, item := range pods.Items {
		if ann, ok := item.GetAnnotations()["kubevirt.io/domain"]; ok && ann == vmName {
			pod = item
		}
	}
	Expect(pod).WithOffset(1).ToNot(BeNil())
	return pod
}

func DeletePod(kvClient kubecli.KubevirtClient, namespace, podName string) {
	ginkgo.By("Delete pod")
	zero := int64(0)
	err := kvClient.CoreV1().Pods(namespace).Delete(context.Background(), podName,
		metav1.DeleteOptions{
			GracePeriodSeconds: &zero,
		})
	Expect(err).WithOffset(1).ToNot(HaveOccurred())

	ginkgo.By("verify deleted")
	Eventually(func() error {
		_, err := kvClient.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		return err
	}, 3*time.Minute, 5*time.Second).
		WithOffset(1).
		Should(Satisfy(apierrs.IsNotFound), "pod should disappear")
}
