package util

import (
	"os"
	"strings"

	"github.com/pkg/errors"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
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

func IsResourceIncluded(resource string, backup *velerov1.Backup) bool {
	if len(backup.Spec.IncludedResources) == 0 {
		// Not a "--include-resources" backup, assume the resource is included
		return true
	}

	for _, res := range backup.Spec.IncludedResources {
		if strings.EqualFold(res, resource) {
			return true
		}
		if strings.EqualFold(res+"s", resource) {
			return true
		}
		if strings.EqualFold(res, resource+"s") {
			return true
		}
	}

	return false
}

func AddAnnotation(item runtime.Unstructured, annotation, value string) {
	metadata, err := meta.Accessor(item)
	if err != nil {
		return
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[annotation] = value

	metadata.SetAnnotations(annotations)
}
