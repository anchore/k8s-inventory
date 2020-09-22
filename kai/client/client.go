// Wraps some of the initialization details for the k8s clientset
package client

import (
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/anchore/kai/internal/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const UseInCluster = "use-in-cluster"

// Based on the application configuration, retrieve the k8s clientset
func GetClientSet(errs chan error, appConfig *config.Application) *kubernetes.Clientset {
	// use the current context in kubeconfig
	kubeConfig, err := getKubeConfig(appConfig)
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

func getKubeConfig(appConfig *config.Application) (*rest.Config, error) {
	if appConfig.KubeConfig == UseInCluster {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", appConfig.KubeConfig)
}
