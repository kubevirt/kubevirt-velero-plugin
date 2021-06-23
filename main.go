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

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	framework.NewServer().
		BindFlags(pflag.CommandLine).
		//RegisterObjectStore("kubevirt-velero-plugin/object-store-plugin", newObjectStorePlugin).
		//RegisterVolumeSnapshotter("kubevirt-velero-plugin/volume-snapshotter-plugin", newNoOpVolumeSnapshotterPlugin).
		//RegisterRestoreItemAction("kubevirt-velero-plugin/restore-plugin", newRestorePlugin).
		RegisterBackupItemAction("kubevirt-velero-plugin/backup-datavolume-action", newNewDVBackupItemAction).
		Serve()
}

func newNewDVBackupItemAction(logger logrus.FieldLogger) (interface{}, error) {
	logger.Debug("Creating DVBackupItemAction")
	return plugin.NewDVBackupItemAction(logger), nil
}

// func newObjectStorePlugin(logger logrus.FieldLogger) (interface{}, error) {
// 	logger.Debug("Creating object store plugin")
// 	return plugin.NewFileObjectStore(logger), nil
// }

// func newRestorePlugin(logger logrus.FieldLogger) (interface{}, error) {
// 	logger.Debug("Creating restore plugin")
// 	return plugin.NewRestorePlugin(logger), nil
// }

// func newNoOpVolumeSnapshotterPlugin(logger logrus.FieldLogger) (interface{}, error) {
// 	logger.Debug("Creating volume snapshotter plugin")
// 	return plugin.NewNoOpVolumeSnapshotter(logger), nil
// }
