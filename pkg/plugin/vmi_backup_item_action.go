/*
 * This file is part of the Kubevirt Velero Plugin project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2021 Red Hat, Inc.
 *
 */

package plugin

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

// VMIBackupItemAction is a backup item action for backing up DataVolumes
type VMIBackupItemAction struct {
	log    logrus.FieldLogger
	client kubernetes.Interface
}

const (
	AnnIsOwned = "cdi.kubevirt.io/velero.isOwned"
)

// NewVMIBackupItemAction instantiates a VMIBackupItemAction.
func NewVMIBackupItemAction(log logrus.FieldLogger, client kubernetes.Interface) *VMIBackupItemAction {
	return &VMIBackupItemAction{log: log, client: client}
}

// AppliesTo returns information about which resources this action should be invoked for.
// The IncludedResources and ExcludedResources slices can include both resources
// and resources with group names. These work: "ingresses", "ingresses.extensions".
// A VMIBackupItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.
func (p *VMIBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{
				"VirtualMachineInstance",
			},
		},
		nil
}

// Execute returns VM's DataVolumes as extra items to back up.
func (p *VMIBackupItemAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	p.log.Info("Executing VMIBackupItemAction")

	if backup == nil {
		return nil, nil, fmt.Errorf("backup object nil!")
	}

	extra := []velero.ResourceIdentifier{}

	vmi := new(kvcore.VirtualMachineInstance)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vmi); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	//zxh：vmi处于running状态时，关联的pod和关联的pvc必须要include。pod要是没有处于冻结状态呢？就不需要判断pod是否被备份了吗？
	if !util.IsVMIPaused(vmi) {
		if !util.IsResourceInBackup("pods", backup) && util.IsResourceInBackup("persistentvolumeclaims", backup) {
			return nil, nil, fmt.Errorf("VM is running but launcher pod is not included in the backup")
		}

		//zxh： 检测pod上是否有 velero的exclude标签。这个pod不能被排除掉
		excluded, err := p.isPodExcludedByLabel(vmi)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}

		if excluded {
			return nil, nil, fmt.Errorf("VM is running but launcher pod is not included in the backup")
		}
	}

	//zxh： vmi创建的vm 必须要备份vm，vm不能被排除
	if isVMIOwned(vmi) {
		if !util.IsResourceInBackup("virtualmachines", backup) {
			return nil, nil, fmt.Errorf("VMI owned by a VM and the VM is not included in the backup")
		}

		excluded, err := isVMExcludedByLabel(vmi)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}

		if excluded {
			return nil, nil, fmt.Errorf("VMI owned by a VM and the VM is not included in the backup")
		}

		util.AddAnnotation(item, AnnIsOwned, "true") //在vmi中添加注解 标识vmi是vm创建的
	} else {
		//zxh： 如果不是vm创建的vmi，这需要判断判断vmi关联的pvc和dv是否已经被备份了。，如果是vm创建的vmi 则在处理vm的是否已经做过这个判断了不需要再重复进行判断了
		restore, err := util.RestorePossible(vmi.Spec.Volumes, backup, vmi.Namespace, func(volume kvcore.Volume) bool { return false }, p.log)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		if !restore {
			return nil, nil, fmt.Errorf("VM has DataVolume or PVC volumes and DataVolumes/PVCs is not included in the backup")
		}
	}

	//zxh: 解析vmi关联的pod，这里没有解析关联的热加载pod
	extra, err := p.addLauncherPod(vmi, extra)
	if err != nil {
		return nil, nil, err
	}

	// zxh： 解析vmi关联的卷和凭据信息（secret）
	extra = util.AddVMIObjectGraph(vmi.Spec, vmi.GetNamespace(), extra, p.log)

	return item, extra, nil
}

//zxh 判断vmi是否由其他资源创建的
func isVMIOwned(vmi *kvcore.VirtualMachineInstance) bool {
	return len(vmi.OwnerReferences) > 0
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var isVMExcludedByLabel = func(vmi *kvcore.VirtualMachineInstance) (bool, error) {
	client, err := util.GetKubeVirtclient()
	if err != nil {
		return false, err
	}

	vm, err := (*client).VirtualMachine(vmi.Namespace).Get(context.Background(), vmi.Name, &metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	label, ok := vm.GetLabels()[util.VELERO_EXCLUDE_LABEL]
	return ok && label == "true", nil
}

func (p *VMIBackupItemAction) isPodExcludedByLabel(vmi *kvcore.VirtualMachineInstance) (bool, error) {
	pod, err := p.getLauncherPod(vmi)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, fmt.Errorf("pod for running VMI not found")
	}

	labels := pod.GetLabels()
	if labels == nil {
		return false, nil
	}

	label, ok := labels[util.VELERO_EXCLUDE_LABEL]
	return ok && label == "true", nil
}

//zxh: 解析vmi关联的pod
func (p *VMIBackupItemAction) getLauncherPod(vmi *kvcore.VirtualMachineInstance) (*core.Pod, error) {
	pods, err := p.client.CoreV1().Pods(vmi.GetNamespace()).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "kubevirt.io=virt-launcher", //zxh: 虚拟机pod都存在这个标签, 可以根据这个标签筛选pod
	})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		//zxh: vmi 关联的pod存在如下标签
		if pod.Annotations["kubevirt.io/domain"] == vmi.GetName() {
			return &pod, nil
		}
	}

	return nil, nil
}

func (p *VMIBackupItemAction) addLauncherPod(vmi *kvcore.VirtualMachineInstance, extra []velero.ResourceIdentifier) ([]velero.ResourceIdentifier, error) {
	pod, err := p.getLauncherPod(vmi)
	if err != nil {
		return nil, err
	}
	if pod != nil {
		extra = append(extra, velero.ResourceIdentifier{
			GroupResource: kuberesource.Pods,
			Namespace:     vmi.GetNamespace(),
			Name:          pod.GetName(),
		})
	}

	return extra, nil
}
