/*
Retrieve Kubernetes In-Use Image data from the Kubernetes API. Runs adhoc and periodically, using the k8s go SDK
*/
package kai

import (
	"fmt"
	"os"
	"time"

	"github.com/anchore/kai/kai/presenter"
	"github.com/anchore/kai/kai/reporter"

	"k8s.io/client-go/rest"

	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/kai/client"
	"github.com/anchore/kai/kai/logger"
	"github.com/anchore/kai/kai/result"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ImageResult struct {
	Namespaces []result.Namespace
	Err        error
}

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

// PeriodicallyGetImageResults periodically retrieve image results and report/output them according to the configuration.
// Note: Errors do not cause the function to exit, since this is periodically running
func PeriodicallyGetImageResults(appConfig *config.Application) {

	// Fire off a ticker that reports according to a configurable polling interval
	ticker := time.NewTicker(time.Duration(appConfig.PollingIntervalSeconds) * time.Second)

	for {
		imageResult, err := GetImageResults(appConfig)
		if err != nil {
			log.Errorf("Failed to get Image Results: %w", err)
		} else {
			err := HandleResults(imageResult, appConfig)
			if err != nil {
				log.Errorf("Failed to handle Image Results: %w", err)
			}
		}

		// Wait at least as long as the ticker
		log.Debugf("Start new gather: %s", <-ticker.C)
	}
}

// Atomic method for getting in-use image results, in parallel for multiple namespaces
func GetImageResults(appConfig *config.Application) (result.Result, error) {
	kubeConfig, err := client.GetKubeConfig(appConfig)
	if err != nil {
		return result.Result{}, err
	}
	searchNamespaces, err := resolveNamespaces(kubeConfig, appConfig)
	if err != nil {
		return result.Result{}, fmt.Errorf("failed to resolve namespaces: %w", err)
	}
	results := make(chan ImageResult)
	for _, searchNamespace := range searchNamespaces {
		// Does a "get pods" for the specified namespace and returns the unique set of images to the results channel
		go getNamespaceImages(kubeConfig, appConfig, searchNamespace, results)
	}
	resolvedNamespaces := make([]result.Namespace, 0)
	for len(resolvedNamespaces) < len(searchNamespaces) {
		select {
		case imageResult := <-results:
			if imageResult.Err != nil {
				return result.Result{}, imageResult.Err
			}
			resolvedNamespaces = append(resolvedNamespaces, imageResult.Namespaces...)
		case <-time.After(time.Second * time.Duration(appConfig.KubernetesRequestTimeoutSeconds)):
			return result.Result{}, fmt.Errorf("timed out waiting for results")
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

// Atomic Function that gets all the Namespace Images for a given searchNamespace and reports them to the unbuffered results channel
func getNamespaceImages(kubeConfig *rest.Config, appConfig *config.Application, searchNamespace string, results chan ImageResult) {
	clientSet, err := client.GetClientSet(kubeConfig)
	if err != nil {
		results <- ImageResult{
			Err: err,
		}
		return
	}
	pods, err := clientSet.CoreV1().Pods(searchNamespace).List(metav1.ListOptions{TimeoutSeconds: &appConfig.KubernetesRequestTimeoutSeconds})
	if err != nil {
		results <- ImageResult{
			Err: err,
		}
		return
	}
	log.Debugf("There are %d pods in the cluster in namespace \"%s\"", len(pods.Items), searchNamespace)
	results <- parseNamespaceImages(pods, searchNamespace)
}

func resolveNamespaces(kubeConfig *rest.Config, appConfig *config.Application) ([]string, error) {
	if len(appConfig.Namespaces) == 0 {
		return GetAllNamespaces(kubeConfig, appConfig.KubernetesRequestTimeoutSeconds)
	}
	resolvedNamespaces := make([]string, 0)
	for _, namespaceStr := range appConfig.Namespaces {
		if namespaceStr == "all" {
			return GetAllNamespaces(kubeConfig, appConfig.KubernetesRequestTimeoutSeconds)
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
