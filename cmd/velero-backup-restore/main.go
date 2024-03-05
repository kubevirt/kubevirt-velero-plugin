/*
 * This file is part of the KubeVirt project
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
 * Copyright 2024 Red Hat, Inc.
 *
 */

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"kubevirt.io/client-go/log"
)

const (
	veleroCLI = "velero"
)

var (
	backupNS         string
	includedNS       string
	includeResources string
	selector         string
	snapshotLocation string
	verify           bool
	deleteBackup     bool
	backupName       string
	restoreName      string
)

func setupCommonFlags(subcommand *flag.FlagSet) {
	subcommand.StringVar(&backupNS, "namespace", "velero", "The namespace in which Velero should operate.")
	subcommand.BoolVar(&verify, "verify-succeeded", false, "Check that the operation completed successfully.")
}

func setupBackupFlags(backupCmd *flag.FlagSet) {
	backupCmd.StringVar(&includedNS, "include-namespaces", "*", "Namespaces to include in the backup (use '*' for all namespaces).")
	backupCmd.StringVar(&selector, "selector", "", "Only back up resources matching this label selector.")
	backupCmd.StringVar(&includeResources, "include-resources", "", "Resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources).")
	backupCmd.StringVar(&snapshotLocation, "volume-snapshot-Location", "", "List of locations (at most one per provider) where volume snapshots should be stored.")
	setupCommonFlags(backupCmd)
}

func setupRestoreFlags(restoreCmd *flag.FlagSet) {
	restoreCmd.StringVar(&backupName, "from-backup", "", "Backup to restore from.")
	setupCommonFlags(restoreCmd)
}

func setupDeleteBackupFlags(deleteBackupCmd *flag.FlagSet) {
	deleteBackupCmd.StringVar(&backupNS, "namespace", "velero", "The namespace in which Velero should operate.")
}

func main() {
	log.InitializeLogging("Velero-backup-restore script")

	backupCmd := flag.NewFlagSet("backup", flag.ExitOnError)
	setupBackupFlags(backupCmd)
	deleteBackupCmd := flag.NewFlagSet("delete-backup", flag.ExitOnError)
	setupDeleteBackupFlags(deleteBackupCmd)
	restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	setupRestoreFlags(restoreCmd)

	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	// os.Arg[2] should be the name of the backup/restore
	if len(os.Args) < 3 {
		usage(nil)
	}

	// Switch on the subcommand
	// Parse the flags for appropriate FlagSet
	switch os.Args[1] {
	case "backup":
		backupName = os.Args[2]
		backupCmd.Parse(os.Args[3:])
		if strings.HasPrefix(backupName, "-") {
			usage(backupCmd)
		}
		if err := handleBackupCommand(backupName); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "delete-backup":
		backupName = os.Args[2]
		deleteBackupCmd.Parse(os.Args[3:])
		if strings.HasPrefix(backupName, "-") {
			usage(deleteBackupCmd)
		}
		if err := handleDeleteBackupCommand(backupName); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "restore":
		restoreName = os.Args[2]
		restoreCmd.Parse(os.Args[3:])
		if strings.HasPrefix(restoreName, "-") {
			usage(restoreCmd)
		}
		if err := handleRestoreCommand(restoreName); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		usage(nil)
	}

	log.Log.Info("Exiting...")
}

func handleBackupCommand(backupName string) error {
	if err := createBackupCommand(backupName); err != nil {
		return fmt.Errorf("Failed to create backup: %s", err.Error())
	}
	if verify {
		if err := verifyBackupCompleted(backupName); err != nil {
			return fmt.Errorf("Failed to verify backup completed successfully: %s", err.Error())
		}
	}
	return nil
}

func createBackupCommand(backupName string) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", includedNS,
		"--namespace", backupNS,
		"--wait",
	}

	if includeResources != "" {
		args = append(args, "--include-resources", includeResources)
	}
	if selector != "" {
		args = append(args, "--selector", selector)
	}
	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	backupCmd := exec.Command(veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Printf("Running backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func verifyBackupCompleted(backupName string) error {
	checkCMD := exec.Command(veleroCLI, "backup", "get", "-n", backupNS, "-o", "json", backupName)

	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return err
	}

	jsonBuf := make([]byte, 16*1024)
	err = checkCMD.Start()
	if err != nil {
		return err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	if bytesRead == len(jsonBuf) {
		return errors.New("json returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return err
	}

	backup := velerov1api.Backup{}
	err = json.Unmarshal(jsonBuf, &backup)
	if err != nil {
		return err
	}
	if backup.Status.Phase != velerov1api.BackupPhaseCompleted {
		return fmt.Errorf("Backup phase is %s instead of completed", backup.Status.Phase)
	}

	return nil
}

func handleDeleteBackupCommand(backupName string) error {
	args := []string{
		"delete", "backup", backupName,
		"--confirm",
		"--namespace", backupNS,
	}

	backupCmd := exec.Command(veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	fmt.Printf("Delete backup cmd =%v\n", backupCmd)
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	return nil
}

func handleRestoreCommand(restoreName string) error {
	if backupName == "" {
		return fmt.Errorf("please specify which backup you want to restore from")
	}
	if err := createRestoreCommand(restoreName); err != nil {
		return fmt.Errorf("Failed to restore: %s", err.Error())
	}
	if verify {
		if err := verifyRestoreCompleted(restoreName); err != nil {
			return fmt.Errorf("Failed to verify restore completed successfully: %s", err.Error())
		}
	}
	return nil
}

func createRestoreCommand(restoreName string) error {
	args := []string{
		"restore", "create", restoreName,
		"--from-backup", backupName,
		"--namespace", backupNS,
		"--wait",
	}

	restoreCmd := exec.Command(veleroCLI, args...)
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr
	fmt.Printf("Running restore cmd =%v\n", restoreCmd)
	err := restoreCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func verifyRestoreCompleted(restoreName string) error {
	checkCMD := exec.Command(veleroCLI, "restore", "get", "-n", backupNS, "-o", "json", restoreName)

	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return err
	}

	jsonBuf := make([]byte, 16*1024)
	err = checkCMD.Start()
	if err != nil {
		return err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}
	if bytesRead == len(jsonBuf) {
		return errors.New("json returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return err
	}
	restore := velerov1api.Restore{}
	err = json.Unmarshal(jsonBuf, &restore)
	if err != nil {
		return err
	}

	if restore.Status.Phase != velerov1api.RestorePhaseCompleted {
		return fmt.Errorf("Restore phase is %s instead of completed", restore.Status.Phase)
	}

	return nil
}

func usage(cmd *flag.FlagSet) {
	if cmd == nil {
		fmt.Printf("Usage:\n"+
			"\t%s [command] NAME\n"+
			"Available Commands:\n"+
			"\tbackup           create backup\n"+
			"\tdelete-backup    delete backup\n"+
			"\trestore          restore a backup\n", os.Args[0])
	} else {
		fmt.Printf("Usage:\n"+
			"\t%s %s NAME\n"+
			"Available Commands:\n", os.Args[0], cmd.Name())
		cmd.PrintDefaults()
	}
	os.Exit(1)
}
