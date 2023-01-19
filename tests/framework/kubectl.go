package framework

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
)

//RunKubectlCommand runs a kubectl Cmd and returns output and err
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
