package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/pkg/errors"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	veleroCLI = "velero"
)

// TODO: change this to resource not a command!!!
func CreateBackupForNamespace(ctx context.Context, backupName string, namespace string, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", namespace,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("backup cmd =%v\n", backupCmd))
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForNamespaceExcludeNamespace(ctx context.Context, backupName, includedNamespace, excludedNamespace, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", includedNamespace,
		"--exclude-namespaces", excludedNamespace,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("backup cmd =%v\n", backupCmd))
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForNamespaceExcludeResources(ctx context.Context, backupName, namespace, resources, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", namespace,
		"--exclude-resources", resources,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("backup cmd =%v\n", backupCmd))
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForSelector(ctx context.Context, backupName, selector, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--selector", selector,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("backup cmd =%v\n", backupCmd))
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CreateBackupForResources(ctx context.Context, backupName, resources, includedNamespace, snapshotLocation string, backupNamespace string, wait bool) error {
	args := []string{
		"create", "backup", backupName,
		"--include-namespaces", includedNamespace,
		"--include-resources", resources,
		"--namespace", backupNamespace,
	}

	if snapshotLocation != "" {
		args = append(args, "--volume-snapshot-locations", snapshotLocation)
	}

	if wait {
		args = append(args, "--wait")
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
	backupCmd.Stdout = os.Stdout
	backupCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("backup cmd =%v\n", backupCmd))
	err := backupCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func DeleteBackup(ctx context.Context, backupName string, backupNamespace string) error {
	args := []string{
		"delete", "backup", backupName,
		"--confirm",
		"--namespace", backupNamespace,
	}

	backupCmd := exec.CommandContext(ctx, veleroCLI, args...)
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

func GetBackup(ctx context.Context, backupName string, backupNamespace string) (*v1.Backup, error) {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "backup", "get", "-n", backupNamespace, "-o", "json", backupName)

	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return nil, err
	}

	jsonBuf := make([]byte, 16*1024)
	err = checkCMD.Start()
	if err != nil {
		return nil, err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if bytesRead == len(jsonBuf) {
		return nil, errors.New("json returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return nil, err
	}
	backup := v1.Backup{}
	err = json.Unmarshal(jsonBuf, &backup)
	if err != nil {
		return nil, err
	}

	return &backup, nil
}

func GetBackupPhase(ctx context.Context, backupName string, backupNamespace string) (v1.BackupPhase, error) {
	backup, err := GetBackup(ctx, backupName, backupNamespace)
	if err != nil {
		return "", err
	}

	return backup.Status.Phase, nil
}

func WaitForBackupPhase(ctx context.Context, backupName string, backupNamespace string, expectedPhase v1.BackupPhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		backup, err := GetBackup(ctx, backupName, backupNamespace)
		if err != nil {
			return false, err
		}
		phase := backup.Status.Phase
		ginkgo.By(fmt.Sprintf("Waiting for backup phase %v, got %v", expectedPhase, phase))
		if backup.Status.CompletionTimestamp != nil && phase != expectedPhase {
			return false, errors.Errorf("Backup finished with: %v ", phase)
		}
		if phase != expectedPhase {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("backup %s not in phase %s within %v", backupName, expectedPhase, waitTime)
	}
	return nil
}

func CreateSnapshotLocation(ctx context.Context, locationName, provider, region string, backupNamespace string) error {
	args := []string{
		"snapshot-location", "create", locationName,
		"--provider", provider,
		"--config", "region=" + region,
		"--namespace", backupNamespace,
	}

	locationCmd := exec.CommandContext(ctx, veleroCLI, args...)
	ginkgo.By(fmt.Sprintf("snapshot-location cmd =%v\n", locationCmd))

	output, err := locationCmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "already exists") {
		return err
	}

	return nil
}

func CreateRestoreForBackup(ctx context.Context, backupName, restoreName string, backupNamespace string, wait bool) error {
	args := []string{
		"restore", "create", restoreName,
		"--from-backup", backupName,
		"--namespace", backupNamespace,
	}

	if wait {
		args = append(args, "--wait")
	}

	restoreCmd := exec.CommandContext(ctx, veleroCLI, args...)
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr
	ginkgo.By(fmt.Sprintf("restore cmd =%v\n", restoreCmd))
	err := restoreCmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func GetRestore(ctx context.Context, restoreName string, backupNamespace string) (*v1.Restore, error) {
	checkCMD := exec.CommandContext(ctx, veleroCLI, "restore", "get", "-n", backupNamespace, "-o", "json", restoreName)

	stdoutPipe, err := checkCMD.StdoutPipe()
	if err != nil {
		return nil, err
	}

	jsonBuf := make([]byte, 16*1024)
	err = checkCMD.Start()
	if err != nil {
		return nil, err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if bytesRead == len(jsonBuf) {
		return nil, errors.New("json returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = checkCMD.Wait()
	if err != nil {
		return nil, err
	}
	restore := v1.Restore{}
	err = json.Unmarshal(jsonBuf, &restore)
	if err != nil {
		return nil, err
	}

	return &restore, nil
}

func GetRestorePhase(ctx context.Context, restoreName string, backupNamespace string) (v1.RestorePhase, error) {
	restore, err := GetRestore(ctx, restoreName, backupNamespace)
	if err != nil {
		return "", err
	}

	return restore.Status.Phase, nil
}

func WaitForRestorePhase(ctx context.Context, restoreName string, backupNamespace string, expectedPhase v1.RestorePhase) error {
	err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		phase, err := GetRestorePhase(ctx, restoreName, backupNamespace)
		ginkgo.By(fmt.Sprintf("Waiting for restore phase %v, got %v", expectedPhase, phase))
		if err != nil || phase != expectedPhase {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("restore %s not in phase %s within %v", restoreName, expectedPhase, waitTime)
	}
	return nil
}
