package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	"github.com/pkg/errors"
)

// RunKubectlCommand runs a kubectl Cmd and returns output and err
func (f *Framework) RunKubectlCommand(args ...string) error {
	cmd := f.CreateKubectlCommand(args...)
	outBytes, err := cmd.CombinedOutput()
	fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", string(outBytes))

	return err
}

// CreateKubectlCommand returns the Cmd to execute kubectl
func (f *Framework) CreateKubectlCommand(args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.KubectlPath

	cmd := exec.Command(path, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	return cmd
}

// RunKubectlCreateYamlCommand replaces storageclassname placeholder with configured tests storageClass
// in a given yaml, creates it and returns err
func (f *Framework) RunKubectlCreateYamlCommand(yamlPath string) error {
	kubeconfig := f.KubeConfig
	path := f.KubectlPath
	storageClass := f.StorageClass

	cmdString := fmt.Sprintf("cat %s | sed 's/{{KVP_STORAGE_CLASS}}/%s/g' | %s create -n %s -f -", yamlPath, storageClass, path, f.Namespace.Name)
	cmd := exec.Command("bash", "-c", cmdString)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)
	outBytes, err := cmd.CombinedOutput()
	fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", string(outBytes))

	return err
}

// KubectlDescribeVeleroBackup execs to the velero pod and runs a describe command on the given
// backup name including details of the resources that were backed up
func (f *Framework) KubectlDescribeVeleroBackup(ctx context.Context, podName, backupName string) (map[string]interface{}, error) {
	var result map[string]interface{}
	kubeconfig := f.KubeConfig
	path := f.KubectlPath
	cmd := exec.CommandContext(ctx, path, "exec", "-it", podName, "-n", f.BackupNamespace, "--", "/velero", "backup", "describe", "--details", "-o", "json", backupName)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}

	jsonBuf := make([]byte, 16*1024)
	err = cmd.Start()
	if err != nil {
		return result, err
	}

	bytesRead, err := io.ReadFull(stdoutPipe, jsonBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return result, err
	}
	if bytesRead == len(jsonBuf) {
		return result, errors.New("json returned bigger than max allowed")
	}

	jsonBuf = jsonBuf[0:bytesRead]
	err = cmd.Wait()
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(jsonBuf, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}
