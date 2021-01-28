/*
Retrieve Kubernetes In-Use Image data from the Kubernetes API. Runs adhoc and periodically, using the k8s go SDK
*/
package kai

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/anchore/kai/kai/presenter"
	"github.com/anchore/kai/kai/reporter"

	"github.com/anchore/kai/internal/util"

	"k8s.io/client-go/rest"

	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/kai/client"
	"github.com/anchore/kai/kai/logger"
	"github.com/anchore/kai/kai/result"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HandleResults(imageResult result.Result, appConfig *config.Application) error {
	if appConfig.AnchoreDetails.IsValid() {
		if err := reporter.Report(imageResult, appConfig.AnchoreDetails, appConfig); err != nil {
			return fmt.Errorf("unable to report Images to Anchore: %w", err)
		}
	} else {
		log.Debug("Anchore details not specified, not reporting in-use image data")
	}

	if err := presenter.GetPresenter(appConfig.PresenterOpt, imageResult).Present(os.Stdout); err != nil {
		return fmt.Errorf("unable to show Kai results: %w", err)
	}
	return nil
}

// According to configuration, periodically retrieve image results and report/output them.
// Note: Errors do not cause the function to exit, since this is periodically running
func PeriodicallyGetImageResults(appConfig *config.Application) {
	// Report one result right away
	imageResult, err := GetAndPublishImageResults(appConfig)
	if err != nil {
		log.Errorf("Failed to get Image Results: %w", err)
	} else {
		err := HandleResults(imageResult, appConfig)
		if err != nil {
			log.Errorf("Failed to handle Image Results: %w", err)
		}
	}

	// Then fire off a ticker that reports according to a configurable polling interval
	ticker := time.NewTicker(time.Duration(appConfig.PollingIntervalSeconds) * time.Second)
	for range ticker.C {
		imageResult, err := GetAndPublishImageResults(appConfig)
		if err != nil {
			log.Errorf("Failed to get Image Results: %w", err)
		} else {
			err := HandleResults(imageResult, appConfig)
			if err != nil {
				log.Errorf("Failed to handle Image Results: %w", err)
			}
		}
	}
}

// According to configuration, retrieve image results and return them
// If the config comes from Anchore, there may be multiple clusters (which is not supported from direct configuration)
func GetAndPublishImageResults(appConfig *config.Application) (result.Result, error) {
	kubeConfig, err := client.GetKubeConfig(appConfig)
	if err != nil {
		return result.Result{}, err
	}
	imagesResult, err := GetImageResults(kubeConfig, appConfig.Namespaces, appConfig.KubernetesRequestTimeoutSeconds)
	if err != nil {
		return result.Result{}, err
	}
	return imagesResult, nil
}

type ImageResult struct {
	Namespaces []result.Namespace
	Err        error
}

// Atomic method for getting in-use image results, in parallel for multiple namespaces
func GetImageResults(kubeConfig *rest.Config, namespaces []string, timeoutSeconds int64) (result.Result, error) {
	searchNamespaces, err := resolveNamespaces(kubeConfig, namespaces, timeoutSeconds)
	if err != nil {
		return result.Result{}, fmt.Errorf("failed to resolve namespaces: %w", err)
	}
	results := make(chan ImageResult, len(searchNamespaces))
	var wg sync.WaitGroup
	for _, searchNamespace := range searchNamespaces {
		wg.Add(1)
		go func(searchNamespace string, wg *sync.WaitGroup) {
			defer wg.Done()
			clientSet, err := client.GetClientSet(kubeConfig)
			if err != nil {
				results <- ImageResult{
					Err: err,
				}
				return
			}
			pods, err := clientSet.CoreV1().Pods(searchNamespace).List(metav1.ListOptions{TimeoutSeconds: &timeoutSeconds})
			if err != nil {
				results <- ImageResult{
					Err: err,
				}
				return
			}
			log.Debugf("There are %d pods in the cluster in namespace \"%s\"", len(pods.Items), searchNamespace)
			results <- parseNamespaceImages(pods, searchNamespace)
		}(searchNamespace, &wg)
	}
	if util.WaitTimeout(&wg, time.Second*time.Duration(timeoutSeconds)) {
		return result.Result{}, fmt.Errorf("timed out waiting for results")
	}
	resolvedNamespaces := make([]result.Namespace, 0)
	for i := 0; i < len(searchNamespaces); i++ {
		select {
		case imageResult := <-results:
			if imageResult.Err != nil {
				return result.Result{}, imageResult.Err
			}
			resolvedNamespaces = append(resolvedNamespaces, imageResult.Namespaces...)
		case <-time.After(time.Second * time.Duration(timeoutSeconds)):
			return result.Result{}, fmt.Errorf("timed out waiting for results from namespace '%s'", searchNamespaces[i])
		}
	}
	close(results)

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
func parseNamespaceImages(pods *v1.PodList, namespace string) ImageResult {
	namespaces := make([]result.Namespace, 0)
	if len(pods.Items) < 1 {
		namespaces = append(namespaces, *result.New(namespace))
		return ImageResult{
			Namespaces: namespaces,
			Err:        nil,
		}
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
	return ImageResult{
		Namespaces: namespaces,
		Err:        nil,
	}
}

func SetLogger(logger logger.Logger) {
	log.Log = logger
}
