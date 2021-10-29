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
	"github.com/anchore/kai/kai/inventory"
	"github.com/anchore/kai/kai/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type k8sNamespace struct {
	Name string
	Err  error
}

type channels struct {
	reportItem chan inventory.ReportItem
	errors     chan error
}

func HandleReport(report inventory.Report, cfg *config.Application) error {
	if cfg.AnchoreDetails.IsValid() {
		if err := reporter.Post(report, cfg.AnchoreDetails, cfg); err != nil {
			return fmt.Errorf("unable to report Inventory to Anchore: %w", err)
		}
	} else {
		log.Debug("Anchore details not specified, not reporting inventory")
	}

	if err := presenter.GetPresenter(cfg.PresenterOpt, report).Present(os.Stdout); err != nil {
		return fmt.Errorf("unable to show inventory: %w", err)
	}
	return nil
}

// PeriodicallyGetInventoryReport periodically retrieve image results and report/output them according to the configuration.
// Note: Errors do not cause the function to exit, since this is periodically running
func PeriodicallyGetInventoryReport(cfg *config.Application) {
	// Fire off a ticker that reports according to a configurable polling interval
	ticker := time.NewTicker(time.Duration(cfg.PollingIntervalSeconds) * time.Second)

	for {
		report, err := GetInventoryReport(cfg)
		if err != nil {
			log.Errorf("Failed to get Inventory Report: %w", err)
		} else {
			err := HandleReport(report, cfg)
			if err != nil {
				log.Errorf("Failed to handle Inventory Report: %w", err)
			}
		}

		// Wait at least as long as the ticker
		log.Debugf("Start new gather: %s", <-ticker.C)
	}
}

// GetInventoryReport is an atomic method for getting in-use image results, in parallel for multiple namespaces
func GetInventoryReport(cfg *config.Application) (inventory.Report, error) {
	kubeconfig, err := client.GetKubeConfig(cfg)
	if err != nil {
		return inventory.Report{}, err
	}

	ch := channels{
		reportItem: make(chan inventory.ReportItem),
		errors:     make(chan error),
	}
	nsCh := make(chan k8sNamespace)

	go fetchNamespaces(kubeconfig, cfg, nsCh)

	total := 0
	for ns := range nsCh {
		if ns.Err != nil {
			return inventory.Report{}, fmt.Errorf("failed to resolve namespace: %w", ns.Err)
		}

		// Does a "get pods" for the specified namespace and returns the unique set of images to the ch.reportItem channel
		go fetchPodsInNamespace(kubeconfig, cfg.Kubernetes, ns.Name, ch)
		total++
	}

	results := make([]inventory.ReportItem, 0)
	for len(results) < total {
		select {
		case item := <-ch.reportItem:
			results = append(results, item)

		case err := <-ch.errors:
			return inventory.Report{}, err

		case <-time.After(time.Second * time.Duration(cfg.Kubernetes.RequestTimeoutSeconds)):
			return inventory.Report{}, fmt.Errorf("timed out waiting for results")
		}
	}
	close(ch.reportItem)
	close(ch.errors)

	clientSet, err := client.GetClientSet(kubeconfig)
	if err != nil {
		return inventory.Report{}, fmt.Errorf("failed to get k8s client set: %w", err)
	}

	serverVersion, err := clientSet.Discovery().ServerVersion()
	if err != nil {
		return inventory.Report{}, fmt.Errorf("failed to get Cluster Server Version: %w", err)
	}

	return inventory.Report{
		Timestamp:             time.Now().UTC().Format(time.RFC3339),
		Results:               results,
		ServerVersionMetadata: serverVersion,
		ClusterName:           cfg.KubeConfig.Cluster,
		InventoryType:         "kubernetes",
	}, nil
}

// fetchNamespaces sends namespace strings through a channel back to the calling function. It will
// either return the namespaces detailed in the configuration OR if "all" is specified then it will
// call fetchAllNamespaces to return every namespace in the cluster.
func fetchNamespaces(kubeconfig *rest.Config, cfg *config.Application, nsCh chan k8sNamespace) {
	getAll := false
	for _, ns := range cfg.Namespaces {
		if ns == "all" {
			getAll = true
			break
		}
	}

	if len(cfg.Namespaces) == 0 || getAll {
		fetchAllNamespaces(kubeconfig, cfg.Kubernetes, nsCh)
	} else {
		for _, ns := range cfg.Namespaces {
			nsCh <- k8sNamespace{
				Name: ns,
				Err:  nil,
			}
		}
	}
	close(nsCh)
}

// fetchAllNamespaces fetches all the namespaces in a cluster and returns them in a slice
// Helper function for retrieving the namespaces in the configured cluster (see client.GetClientSet)
func fetchAllNamespaces(kubeconfig *rest.Config, kubernetes config.KubernetesAPI, nsCh chan k8sNamespace) {
	clientset, err := client.GetClientSet(kubeconfig)
	if err != nil {
		nsCh <- k8sNamespace{
			Name: "",
			Err:  fmt.Errorf("failed to get k8s client set: %w", err),
		}
		return
	}

	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          kubernetes.RequestBatchSize,
			Continue:       cont,
			TimeoutSeconds: &kubernetes.RequestTimeoutSeconds,
		}

		list, err := clientset.CoreV1().Namespaces().List(opts)
		if err != nil {
			// TODO: Handle HTTP 410 and recover
			nsCh <- k8sNamespace{
				Name: "",
				Err:  fmt.Errorf("failed to list namespaces: %w", err),
			}
			return
		}

		for _, ns := range list.Items {
			nsCh <- k8sNamespace{
				Name: ns.ObjectMeta.Name,
				Err:  nil,
			}
		}

		cont = list.GetListMeta().GetContinue()

		if cont == "" {
			break
		}
	}
}

// Atomic Function that gets all the Namespace Images for a given searchNamespace and reports them to the unbuffered results channel
func fetchPodsInNamespace(kubeconfig *rest.Config, kubernetes config.KubernetesAPI, ns string, ch channels) {
	clientSet, err := client.GetClientSet(kubeconfig)
	if err != nil {
		ch.errors <- err
		return
	}

	pods := make([]v1.Pod, 0)
	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          kubernetes.RequestBatchSize,
			Continue:       cont,
			TimeoutSeconds: &kubernetes.RequestTimeoutSeconds,
		}

		list, err := clientSet.CoreV1().Pods(ns).List(opts)
		if err != nil {
			// TODO: Handle HTTP 410 and recover
			ch.errors <- err
			return
		}

		pods = append(pods, list.Items...)

		cont = list.GetListMeta().GetContinue()

		if cont == "" {
			break
		}
	}

	log.Debugf("There are %d pods in namespace \"%s\"", len(pods), ns)
	ch.reportItem <- inventory.NewReportItem(pods, ns)
}

func SetLogger(logger logger.Logger) {
	log.Log = logger
}
