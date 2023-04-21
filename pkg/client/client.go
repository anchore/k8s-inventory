// Wraps some of the initialization details for the k8s clientset
package client

import (
	"fmt"
	"path/filepath"

	"github.com/anchore/k8s-inventory/internal/log"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/mitchellh/go-homedir"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const UseInCluster = "use-in-cluster"

// Based on the application configuration, retrieve the k8s clientset
func GetClientSet(kubeConfig *rest.Config) (*kubernetes.Clientset, error) {
	// create the clientset
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kube clientset: %w", err)
	}

	return clientset, nil
}

func GetKubeConfig(appConfig *config.Application) (*rest.Config, error) {
	switch {
	case appConfig.KubeConfig.IsKubeConfigFromFile():
		if appConfig.KubeConfig.Path == UseInCluster {
			log.Debug("using in-cluster kube config")
			return rest.InClusterConfig()
		}
		log.Debugf("using kube config from file: %s", appConfig.KubeConfig.Path)
		return clientcmd.BuildConfigFromFlags("", appConfig.KubeConfig.Path)
	case appConfig.KubeConfig.IsNonFileKubeConfigValid():
		log.Debug("using kube config from conf")
		return appConfig.KubeConfig.GetKubeConfigFromConf()
	default:
		home, err := homedir.Dir()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve home dir: %w", err)
		}
		log.Debug("using kube config from ~/.kube/config")
		return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
	}
}
