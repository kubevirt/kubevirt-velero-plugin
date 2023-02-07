package tests_test

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"

	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
	"kubevirt.io/kubevirt-velero-plugin/tests/framework"
)

const (
	pollInterval     = 2 * time.Second
	nsDeletedTimeout = 270 * time.Second
)

var (
	kubectlPath = flag.String("kubectl-path", "kubectl", "The path to the kubectl binary")
	kubeConfig  = flag.String("kubeconfig", "/var/run/kubernetes/admin.kubeconfig", "The absolute path to the kubeconfig file")
)

func TestTests(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(Fail)
	BuildTestSuite()
	RunSpecs(t, "KubeVirt Velero Plugin")
}

func BuildTestSuite() {
	BeforeSuite(func() {
		fmt.Fprintf(GinkgoWriter, "Reading parameters\n")
		// Read flags, and configure client instances
		framework.ClientsInstance.KubectlPath = *kubectlPath
		framework.ClientsInstance.KubeConfig = *kubeConfig

		fmt.Fprintf(GinkgoWriter, "Kubectl path: %s\n", framework.ClientsInstance.KubectlPath)
		fmt.Fprintf(GinkgoWriter, "Kubeconfig: %s\n", framework.ClientsInstance.KubeConfig)

		cfg, err := framework.ClientsInstance.LoadConfig()
		if err != nil {
			// Can't use Expect here due this being called outside of an It block, and Expect
			// requires any calls to it to be inside an It block.
			Fail("ERROR, unable to load RestConfig")
		}
		kubevirtClient, err := kubecli.GetKubevirtClientFromRESTConfig(cfg)
		if err != nil {
			Fail(fmt.Sprintf("ERROR, unable to create KubevirtClient: %v", err))
		}
		framework.ClientsInstance.KvClient = kubevirtClient

		k8sClient, err := util.GetK8sClient()
		if err != nil {
			Fail(fmt.Sprintf("ERROR, unable to create K8sClient: %v", err))
		}
		framework.ClientsInstance.K8sClient = k8sClient
	})

	AfterSuite(func() {
		Eventually(func() []v1.Namespace {
			nsList, _ := getTestNamespaceList(framework.ClientsInstance.K8sClient)
			fmt.Fprintf(GinkgoWriter, "DEBUG: AfterSuite nsList: %v\n", nsList.Items)
			return nsList.Items
		}, nsDeletedTimeout, pollInterval).Should(BeEmpty())
	})
}

// getTestNamespaceList returns a list of namespaces that have been created by the functional tests.
func getTestNamespaceList(client *kubernetes.Clientset) (*v1.NamespaceList, error) {
	//Ensure that no namespaces with the prefix label exist
	return client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: framework.TestNamespacePrefix,
	})
}
