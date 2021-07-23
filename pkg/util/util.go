package util

import (
	"os"

	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubecli "kubevirt.io/client-go/kubecli"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

func GetK8sClient() (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return client, nil
}

func GetCDIclientset() (*cdiclientset.Clientset, error) {
	kubeConfig := os.Getenv("KUBECONFIG")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}
	cdiClient, err := cdiclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cdiClient, nil
}

func GetKubeVirtclient() (*kubecli.KubevirtClient, error) {
	kubeConfig := os.Getenv("KUBECONFIG")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}
	kubevirtClient, err := kubecli.GetKubevirtClientFromRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &kubevirtClient, nil
}
