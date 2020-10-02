/*
Retrieve Kubernetes In-Use Image data from the Kubernetes API. Runs adhoc and periodically, using the k8s go SDK
*/
package kai

import (
	"fmt"
	"sync"
	"time"

	"github.com/anchore/kai/kai/event"
	"github.com/anchore/kai/kai/presenter"

	"k8s.io/client-go/rest"

	"github.com/anchore/kai/internal/bus"
	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/kai/client"
	"github.com/anchore/kai/kai/logger"
	"github.com/anchore/kai/kai/result"
	"github.com/wagoodman/go-partybus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// According to configuration, periodically retrieve image results and report them to the Event Bus for printing/reporting
func PeriodicallyGetImageResults(errs chan error, appConfig *config.Application) {
	// Report one result right away
	GetAndPublishImageResults(errs, appConfig)

	// Then fire off a ticker that reports according to a configurable polling interval
	ticker := time.NewTicker(time.Duration(appConfig.PollingIntervalSeconds) * time.Second)
	for range ticker.C {
		GetAndPublishImageResults(errs, appConfig)
	}
}

// According to configuration, retrieve image results and publish them to the Event Bus
// If the config comes from Anchore, there may be multiple clusters (which is not supported from direct configuration)
func GetAndPublishImageResults(errs chan error, appConfig *config.Application) {
	kubeConfig, err := client.GetKubeConfig(appConfig)
	if err != nil {
		errs <- err
	} else {
		imagesResult := GetImageResults(errs, kubeConfig, appConfig.KubeConfig.Cluster, appConfig.Namespaces)
		bus.Publish(partybus.Event{
			Type:   event.ImageResultsRetrieved,
			Source: imagesResult,
			Value:  presenter.GetPresenter(appConfig.PresenterOpt, imagesResult),
		})
	}
}

// Atomic method for getting in-use image results, in parallel for multiple namespaces
func GetImageResults(errs chan error, kubeConfig *rest.Config, clusterName string, namespaces []string) result.Result {
	searchNamespaces := resolveNamespaces(namespaces)
	namespaceChan := make(chan []result.Namespace, len(searchNamespaces))
	var wg sync.WaitGroup
	for _, searchNamespace := range searchNamespaces {
		wg.Add(1)
		go func(searchNamespace string, wg *sync.WaitGroup) {
			defer wg.Done()
			clientSet := client.GetClientSet(errs, kubeConfig)
			pods, err := clientSet.CoreV1().Pods(searchNamespace).List(metav1.ListOptions{})
			if err != nil {
				errs <- fmt.Errorf("failed to List Pods: %w", err)
			}
			log.Debugf("There are %d pods in the cluster in namespace \"%s\"", len(pods.Items), searchNamespace)
			namespaceChan <- parseNamespaceImages(pods, clusterName)
		}(searchNamespace, &wg)
	}
	wg.Wait()
	resolvedNamespaces := make([]result.Namespace, 0)
	for i := 0; i < len(searchNamespaces); i++ {
		channelNamespaceMsg := <-namespaceChan
		resolvedNamespaces = append(resolvedNamespaces, channelNamespaceMsg...)
	}
	close(namespaceChan)

	serverVersion, err := client.GetClientSet(errs, kubeConfig).Discovery().ServerVersion()
	if err != nil {
		errs <- fmt.Errorf("failed to get Cluster Server Version: %w", err)
	}

	return result.Result{
		Timestamp:             time.Now().UTC().Format(time.RFC3339),
		Results:               resolvedNamespaces,
		ServerVersionMetadata: serverVersion,
	}
}

// Helper function for retrieving the namespaces in the configured cluster (see client.GetClientSet)
func ListNamespaces(appConfig *config.Application) ([]string, error) {
	kubeConfig, err := client.GetKubeConfig(appConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig from app config: %w", err)
	}
	namespaces, err := client.GetClientSet(nil, kubeConfig).CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaceNameSlice := make([]string, len(namespaces.Items))
	for _, namespace := range namespaces.Items {
		namespaceNameSlice = append(namespaceNameSlice, namespace.ObjectMeta.Name)
	}

	return namespaceNameSlice, nil
}

// If the configured namespaces to search contains "all", we can execute a single request to get in-use image data.
func resolveNamespaces(namespaces []string) []string {
	// If Namespaces contains "all", just search all namespaces
	if len(namespaces) == 0 {
		return []string{""}
	}
	resolvedNamespaces := make([]string, 0)
	for _, namespaceStr := range namespaces {
		if namespaceStr == "all" {
			return []string{""}
		}
		resolvedNamespaces = append(resolvedNamespaces, namespaceStr)
	}
	return resolvedNamespaces
}

// Parse Pod List results into a list of Namespaces (each with unique Images)
func parseNamespaceImages(pods *v1.PodList, cluster string) []result.Namespace {
	namespaceMap := make(map[string]*result.Namespace)
	for _, pod := range pods.Items {
		namespace := pod.ObjectMeta.Namespace
		if namespace == "" || len(pod.Status.ContainerStatuses) == 0 {
			continue
		}

		if value, ok := namespaceMap[namespace]; ok {
			value.AddImages(pod, cluster)
		} else {
			namespaceMap[namespace] = result.NewNamespace(pod, cluster)
		}
	}

	namespaces := make([]result.Namespace, 0)
	for _, value := range namespaceMap {
		namespaces = append(namespaces, *value)
	}
	return namespaces
}

func SetLogger(logger logger.Logger) {
	log.Log = logger
}

func SetBus(b *partybus.Bus) {
	bus.SetPublisher(b)
}
