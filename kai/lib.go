/*
Retrieve Kubernetes In-Use Image data from the Kubernetes API. Runs adhoc and periodically, using the k8s go SDK
*/
package kai

import (
	"fmt"
	"sync"
	"time"

	"github.com/anchore/kai/internal/errors"
	"github.com/anchore/kai/internal/util"

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
func PeriodicallyGetImageResults(errs chan *errors.KaiError, appConfig *config.Application) {
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
func GetAndPublishImageResults(errs chan *errors.KaiError, appConfig *config.Application) {
	kubeConfig, err := client.GetKubeConfig(appConfig)
	if err != nil {
		errs <- errors.NewFromError(err, "failed to get kubeconfig")
		return
	}
	imagesResult, err := GetImageResults(errs, kubeConfig, appConfig.Namespaces, appConfig.KubernetesRequestTimeoutSeconds)
	if err != nil {
		errs <- errors.NewFromError(err, "failed to get image results")
		return
	}
	bus.Publish(partybus.Event{
		Type:   event.ImageResultsRetrieved,
		Source: imagesResult,
		Value:  presenter.GetPresenter(appConfig.PresenterOpt, imagesResult),
	})
}

// Atomic method for getting in-use image results, in parallel for multiple namespaces
func GetImageResults(errs chan *errors.KaiError, kubeConfig *rest.Config, namespaces []string, timeoutSeconds int64) (result.Result, error) {
	searchNamespaces, err := resolveNamespaces(kubeConfig, namespaces, timeoutSeconds)
	if err != nil {
		return result.Result{}, fmt.Errorf("failed to resolve namespaces: %w", err)
	}
	namespaceChan := make(chan []result.Namespace, len(searchNamespaces))
	var wg sync.WaitGroup
	for _, searchNamespace := range searchNamespaces {
		wg.Add(1)
		go func(searchNamespace string, wg *sync.WaitGroup) {
			defer wg.Done()
			clientSet, err := client.GetClientSet(kubeConfig)
			if err != nil {
				errs <- errors.NewFromError(err, "failed to get k8s clientset")
				return
			}
			pods, err := clientSet.CoreV1().Pods(searchNamespace).List(metav1.ListOptions{TimeoutSeconds: &timeoutSeconds})
			if err != nil {
				errs <- errors.NewFromError(err, "failed to List Pods")
				return
			}
			log.Debugf("There are %d pods in the cluster in namespace \"%s\"", len(pods.Items), searchNamespace)
			namespaceChan <- parseNamespaceImages(pods, searchNamespace)
		}(searchNamespace, &wg)
	}
	if util.WaitTimeout(&wg, time.Second*time.Duration(timeoutSeconds)) {
		return result.Result{}, fmt.Errorf("timed out waiting for results")
	}
	resolvedNamespaces := make([]result.Namespace, 0)
	for i := 0; i < len(searchNamespaces); i++ {
		log.Debugf("Waiting for results from %s", searchNamespaces[i])
		select {
		case channelNamespaceMsg := <-namespaceChan:
			log.Debugf("Got result from %s", searchNamespaces[i])
			resolvedNamespaces = append(resolvedNamespaces, channelNamespaceMsg...)
		case <-time.After(time.Second * time.Duration(timeoutSeconds)):
			log.Debugf("Timed out waiting for results from namespace: '%s'", searchNamespaces[i])
			return result.Result{}, fmt.Errorf("timed out waiting for results from namespace '%s'", searchNamespaces[i])
		}
	}
	close(namespaceChan)

	clientSet, err := client.GetClientSet(kubeConfig)
	if err != nil {
		return result.Result{}, fmt.Errorf("failed to get k8s client set: %w", err)
	}
	serverVersion, err := clientSet.Discovery().ServerVersion()
	if err != nil {
		return result.Result{}, fmt.Errorf("failed to get Cluster Server Version: %w", err)
	}

	return result.Result{
		Timestamp:             time.Now().UTC().Format(time.RFC3339),
		Results:               resolvedNamespaces,
		ServerVersionMetadata: serverVersion,
	}, nil
}

func resolveNamespaces(kubeConfig *rest.Config, namespaces []string, timeoutSeconds int64) ([]string, error) {
	if len(namespaces) == 0 {
		return GetAllNamespaces(kubeConfig, timeoutSeconds)
	}
	resolvedNamespaces := make([]string, 0)
	for _, namespaceStr := range namespaces {
		if namespaceStr == "all" {
			return GetAllNamespaces(kubeConfig, timeoutSeconds)
		}
		resolvedNamespaces = append(resolvedNamespaces, namespaceStr)
	}
	return resolvedNamespaces, nil
}

// Helper function for retrieving the namespaces in the configured cluster (see client.GetClientSet)
func GetAllNamespaces(kubeConfig *rest.Config, timeoutSeconds int64) ([]string, error) {
	clientSet, err := client.GetClientSet(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s client set: %w", err)
	}
	namespaceList, err := clientSet.CoreV1().Namespaces().List(metav1.ListOptions{TimeoutSeconds: &timeoutSeconds})
	if err != nil {
		return nil, fmt.Errorf("failed to list all namespaces: %w", err)
	}
	namespaces := make([]string, 0, len(namespaceList.Items))
	for _, namespace := range namespaceList.Items {
		namespaces = append(namespaces, namespace.ObjectMeta.Name)
	}
	return namespaces, nil
}

// Parse Pod List results into a list of Namespaces (each with unique Images)
func parseNamespaceImages(pods *v1.PodList, namespace string) []result.Namespace {
	namespaces := make([]result.Namespace, 0)
	if len(pods.Items) < 1 {
		namespaces = append(namespaces, *result.New(namespace))
		return namespaces
	}

	namespaceMap := make(map[string]*result.Namespace)
	for _, pod := range pods.Items {
		namespace := pod.ObjectMeta.Namespace
		if namespace == "" || len(pod.Status.ContainerStatuses) == 0 {
			continue
		}

		if value, ok := namespaceMap[namespace]; ok {
			value.AddImages(pod)
		} else {
			namespaceMap[namespace] = result.NewFromPod(pod)
		}
	}

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
