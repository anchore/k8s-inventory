package client

import (
	"fmt"

	"github.com/anchore/kai/internal/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func GetClientSet(errs chan error, appConfig *config.Application) *kubernetes.Clientset {
	// use the current context in kubeconfig
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", appConfig.KubeConfig)
	if err != nil {
		errs <- fmt.Errorf("failed to build kube client config: %w", err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		errs <- fmt.Errorf("failed to build kube clientset: %w", err)
	}
	return clientset
}
