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
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kvcore "kubevirt.io/api/core/v1"
)

// VMIRestorePlugin is a VMI restore item action plugin for Velero (duh!)
type VMIRestorePlugin struct {
	log logrus.FieldLogger
}

// Copied over from KubeVirt
// TODO: Consider making it public in KubeVirt
var restrictedVmiLabels = []string{
	kvcore.CreatedByLabel, //zxh: 标识创建该资源的实体或组件。这可以是用户、服务账户、控制器或其他系统组件。这对于审计和追踪资源的创建者非常有用
	kvcore.MigrationJobLabel, //zxh:  与迁移任务相关联。当从一个节点迁移到另一个节点时，这个标签可以帮助标识哪些资源是迁移过程的一部分。这对于监控和管理迁移流程特别重要。
	kvcore.NodeNameLabel, // 标记Pod或资源绑定到的具体节点名称。这对于理解Pod的部署位置以及在故障排除时定位问题非常有帮助。
	kvcore.MigrationTargetNodeNameLabel, //指定迁移的目标节点。在资源迁移场景下，这个标签指明了资源将要迁移到的节点，有助于自动化迁移过程并确保迁移的准确性。
	kvcore.NodeSchedulable, //控制节点是否可调度。如果设置为false，Kubernetes不会在该节点上调度新的Pods，这通常用于维护窗口或当节点处于不稳定状态时。
	kvcore.InstallStrategyLabel, //指示安装策略。这可以用于指定如何安装或更新应用，例如，是使用滚动更新、重新创建还是其他策略。这对于自动化部署和更新流程至关重要。
}

// NewVMIRestorePlugin instantiates a RestorePlugin.
func NewVMIRestoreItemAction(log logrus.FieldLogger) *VMIRestorePlugin {
	return &VMIRestorePlugin{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *VMIRestorePlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{
			"VirtualMachineInstance",
		},
	}, nil
}

func (p *VMIRestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Running VMIRestorePlugin")

	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	vmi := new(kvcore.VirtualMachineInstance)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), vmi); err != nil {
		return nil, errors.WithStack(err)
	}

	owned, ok := vmi.Annotations[AnnIsOwned] //zxh: 备份的时候会给VMI中添加注解，边界该VMI是否由其他资源创建的，如果是，则不需要独立恢复
	if ok && owned == "true" {
		p.log.Info("VMI is owned by a VM, it doesn't need to be restored")
		return velero.NewRestoreItemActionExecuteOutput(input.Item).WithoutRestore(), nil //zxh： 跳过恢复vmi
	}

	metadata, err := meta.Accessor(input.Item)
	if err != nil {
		return nil, err
	}

	//这句话意味着如果这些受限制的标签没有被清除，那么虚拟机实例（VMI，Virtual Machine Instance）将不会被系统接受或启动。
	//这是因为这些标签可能包含了一些不应该在VMI的生命周期中持久存在的信息，或者它们可能与VMI的当前状态不兼容，导致系统拒绝VMI的启动请求
	//这句话说明了受限制标签的内容。它们包含了关于底层KVM（Kernel-based Virtual Machine）对象的运行时信息。运行时信息是在程序执行期间动态生成的数据，可能包括但不限于：进程状态、内存使用情况、CPU使用率、网络统计信息等。
	//对于KVM而言，这可能涉及到了虚拟化层的状态信息，如虚拟机的运行状态、资源分配情况等。
	// Restricted labels must be cleared otherwise the VMI will be rejected.
	// The restricted labels contain runtime information about the underlying KVM object.
	labels := removeRestrictedLabels(vmi.GetLabels())
	metadata.SetLabels(labels)

	return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
}

//zxh： 如果vmi不是由其他资源创建的，要把vmi上的一些标签给移除掉
func removeRestrictedLabels(labels map[string]string) map[string]string {
	for _, label := range restrictedVmiLabels {
		delete(labels, label)
	}
	return labels
}
