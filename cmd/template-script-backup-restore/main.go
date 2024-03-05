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
	"flag"
	"fmt"
	"os"
	"strings"

	"kubevirt.io/client-go/log"
)

const (
	// TODO: replace value with your solution namespace
	defaultNamespace = ""
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

//TODO: update flags default if needed
func setupCommonFlags(subcommand *flag.FlagSet) {
	subcommand.StringVar(&backupNS, "namespace", defaultNamespace, "The namespace the backup and restore should operate.")
	subcommand.BoolVar(&verify, "verify-succeeded", false, "Check that the operation completed successfully.")
}

//TODO: update flags default if needed
func setupBackupFlags(backupCmd *flag.FlagSet) {
	backupCmd.StringVar(&includedNS, "include-namespaces", "*", "Namespaces to include in the backup (use '*' for all namespaces).")
	backupCmd.StringVar(&selector, "selector", "", "Only back up resources matching this label selector.")
	backupCmd.StringVar(&includeResources, "include-resources", "", "Resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources).")
	backupCmd.StringVar(&snapshotLocation, "volume-snapshot-Location", "", "List of locations (at most one per provider) where volume snapshots should be stored.")
	setupCommonFlags(backupCmd)
}

//TODO: update flags default if needed
func setupRestoreFlags(restoreCmd *flag.FlagSet) {
	restoreCmd.StringVar(&backupName, "from-backup", "", "Backup to restore from.")
	setupCommonFlags(restoreCmd)
}

//TODO: update flags default if needed
func setupDeleteBackupFlags(deleteBackupCmd *flag.FlagSet) {
	deleteBackupCmd.StringVar(&backupNS, "namespace", defaultNamespace, "The namespace in which the backup exists.")
}

func main() {
	//TODO: update script name
	log.InitializeLogging("SCRIPT_NAME script")

	backupCmd := flag.NewFlagSet("backup", flag.ExitOnError)
	setupBackupFlags(backupCmd)
	deleteBackupCmd := flag.NewFlagSet("delete-backup", flag.ExitOnError)
	setupDeleteBackupFlags(deleteBackupCmd)
	restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	setupRestoreFlags(restoreCmd)

	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		usage(nil)
	}

	// Switch on the subcommand
	// Parse the flags for appropriate FlagSet
	switch os.Args[1] {
	case "backup":
		if len(os.Args) == 2 || strings.HasPrefix(os.Args[2], "-") {
			usage(backupCmd)
		}
		backupName = os.Args[2]
		backupCmd.Parse(os.Args[3:])
		if err := handleBackupCommand(backupName); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "delete-backup":
		if len(os.Args) == 2 || strings.HasPrefix(os.Args[2], "-") {
			usage(deleteBackupCmd)
		}
		backupName = os.Args[2]
		deleteBackupCmd.Parse(os.Args[3:])
		if err := handleDeleteBackupCommand(backupName); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "restore":
		if len(os.Args) == 2 || strings.HasPrefix(os.Args[2], "-") {
			usage(restoreCmd)
		}
		restoreName = os.Args[2]
		restoreCmd.Parse(os.Args[3:])
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

//TODO: Implement create backup based on your solution using the given flags
// use includeNS variable to backup specific namespaces
// use includeResources variable to backup only specific resources
// use selector variable to backup specific selector labels
// use snapshotLocation to backup to specific snapshot location
// use backupNS variable for your backup namespace flag
func createBackupCommand(backupName string) error {
	return nil
}

//TODO: Implement verify backup completed based on your backup solution
func verifyBackupCompleted(backupName string) error {
	return nil
}

//TODO: Implement delete backup based on your solution
// use backupNS variable for your backup namespace flag
func handleDeleteBackupCommand(backupName string) error {
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

//TODO: Implement create restore based on your solution
// use backupName variable to know the name of the backup to restore from
// use backupNS variable for your backup namespace flag
func createRestoreCommand(restoreName string) error {
	return nil
}

//TODO: Implement verify restore completed based on your backup solution
func verifyRestoreCompleted(restoreName string) error {
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
