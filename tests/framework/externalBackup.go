package framework

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// RunBackupScript runs the configured backup-restore script.
// The script will run with appropriate args and verify backup completed successfully
// if script flag was not define, we will run veleroCLI instead
func (f *Framework) RunBackupScript(ctx context.Context, backupName, resources, selector, includedNamespace, snapshotLocation, backupNamespace string) error {
	if f.BackupScript.BackupScript == "" {
		return runVeleroCLIBackup(ctx, backupName, resources, selector, includedNamespace, snapshotLocation, backupNamespace)
	}
	args := []string{
		"backup", backupName,
		"--include-namespaces", includedNamespace,
		"--namespace", backupNamespace,
		"--verify-succeeded",
	}

	if resources != "" {
		args = append(args, "--include-resources", resources)
	}
	if selector != "" {
		args = append(args, "--selector", selector)
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	backupCmd := exec.CommandContext(ctx, f.BackupScript.BackupScript, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("backup cmd =%v\n", backupCmd))
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func runVeleroCLIBackup(ctx context.Context, backupName, resources, selector, includedNamespace, snapshotLocation, backupNamespace string) error {
	var err error
	if resources != "" {
		err = CreateBackupForResources(ctx, backupName, resources, includedNamespace, snapshotLocation, backupNamespace, true)
	} else if selector != "" {
		err = CreateBackupForSelector(ctx, backupName, selector, includedNamespace, snapshotLocation, backupNamespace, true)
	} else {
		err = CreateBackupForNamespace(ctx, backupName, includedNamespace, snapshotLocation, backupNamespace, true)
	}
	if err != nil {
		return err
	}
	err = WaitForBackupPhase(ctx, backupName, backupNamespace, velerov1api.BackupPhaseCompleted)
	return err
}

// RunRestoreScript runs the configured backup-restore script.
// The script will run with appropriate args and verify backup completed successfully
// if script flag was not define, we will run veleroCLI instead
func (f *Framework) RunRestoreScript(ctx context.Context, backupName, restoreName string, backupNamespace string) error {
	if f.BackupScript.BackupScript == "" {
		err := CreateRestoreForBackup(ctx, backupName, restoreName, backupNamespace, true)
		if err != nil {
			return err
		}
		err = WaitForRestorePhase(ctx, restoreName, backupNamespace, velerov1api.RestorePhaseCompleted)
		return err
	}
	args := []string{
		"restore", restoreName,
		"--from-backup", backupName,
		"--namespace", backupNamespace,
		"--verify-succeeded",
	}

	restoreCmd := exec.CommandContext(ctx, f.BackupScript.BackupScript, args...)
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("restore cmd =%v\n", restoreCmd))
	err := restoreCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// RunDeleteBackupScript runs the configured backup-restore script.
// The script will run with appropriate args and verify backup completed successfully
// if script flag was not define, we will run veleroCLI instead
func (f *Framework) RunDeleteBackupScript(ctx context.Context, backupName string, backupNamespace string) error {
	if f.BackupScript.BackupScript == "" {
		return DeleteBackup(ctx, backupName, backupNamespace)
	}
	args := []string{
		"delete-backup", backupName,
		"--namespace", backupNamespace,
	}

	backupCmd := exec.CommandContext(ctx, f.BackupScript.BackupScript, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("backup cmd =%v\n", backupCmd))
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	return nil
}
