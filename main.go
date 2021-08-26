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

package main

import (
	"kubevirt.io/kubevirt-velero-plugin/pkg/plugin"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	framework.NewServer().
		BindFlags(pflag.CommandLine).
		RegisterRestoreItemAction("kubevirt-velero-plugin/restore-vm-action", newVMRestoreItemAction).
		RegisterRestoreItemAction("kubevirt-velero-plugin/restore-vmi-action", newVMIRestoreItemAction).
		RegisterRestoreItemAction("kubevirt-velero-plugin/restore-pod-action", newPodRestoreItemAction).
		RegisterBackupItemAction("kubevirt-velero-plugin/backup-datavolume-action", newDVBackupItemAction).
		RegisterBackupItemAction("kubevirt-velero-plugin/backup-virtualmachine-action", newVMBackupItemAction).
		RegisterBackupItemAction("kubevirt-velero-plugin/backup-virtualmachineinstance-action", newVMIBackupItemAction).
		Serve()
}

func newDVBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	logger.Debug("Creating DVBackupItemAction")
	return plugin.NewDVBackupItemAction(logger), nil
}

func newVMBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	logger.Debug("Creating VMBackupItemAction")
	return plugin.NewVMBackupItemAction(logger), nil
}

func newVMIBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	logger.Debug("Creating VMIBackupItemAction")
	client, err := util.GetK8sClient()
	if err != nil {
		return nil, err
	}

	return plugin.NewVMIBackupItemAction(logger, client), nil
}

func newVMRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	logger.Debug("Creating VMIRestoreItemAction")
	return plugin.NewVMRestoreItemAction(logger), nil
}

func newVMIRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	logger.Debug("Creating VMIRestoreItemAction")
	return plugin.NewVMIRestoreItemAction(logger), nil
}

func newPodRestoreItemAction(logger logrus.FieldLogger) (interface{}, error) {
	logger.Debug("Creating PodRestoreItemAction")
	return plugin.NewPodRestoreItemAction(logger), nil
}
