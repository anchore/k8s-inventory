/*
Package retrieves Kubernetes In-Use Image data from the Kubernetes API. Runs adhoc and periodically, using the
k8s go SDK
*/package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/anchore/k8s-inventory/pkg/reporter"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/pkg/client"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/anchore/k8s-inventory/pkg/logger"
)

type ReportItem struct {
	Namespace  inventory.Namespace
	Pods       []inventory.Pod
	Containers []inventory.Container
}

type channels struct {
	reportItem chan ReportItem
	errors     chan error
	stopper    chan struct{}
}

func reportToStdout(report inventory.Report) error {
	enc := json.NewEncoder(os.Stdout)
	// prevent > and < from being escaped in the payload
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("unable to show inventory: %w", err)
	}
	return nil
}

func HandleReport(report inventory.Report, cfg *config.Application) error {
	if cfg.VerboseInventoryReports {
		err := reportToStdout(report)
		if err != nil {
			log.Errorf("Failed to output Inventory Report: %w", err)
		}
	}

	if cfg.AnchoreDetails.IsValid() {
		if err := reporter.Post(report, cfg.AnchoreDetails); err != nil {
			return fmt.Errorf("unable to report Inventory to Anchore: %w", err)
		}
		log.Info("Inventory report sent to Anchore")
	} else {
		log.Info("Anchore details not specified, not reporting inventory")
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

		log.Infof("Waiting %d seconds for next poll...", cfg.PollingIntervalSeconds)

		// Wait at least as long as the ticker
		log.Debugf("Start new gather: %s", <-ticker.C)
	}
}

// launchWorkerPool will create a worker pool of goroutines to grab pods/containers
// from each namespace. This should alleviate the load on the api server
func launchWorkerPool(
	cfg *config.Application,
	kubeconfig *rest.Config,
	ch channels,
	queue chan inventory.Namespace,
	nodes map[string]inventory.Node,
) {
	for i := 0; i < cfg.Kubernetes.WorkerPoolSize; i++ {
		go func() {
			// each worker needs its own clientset
			clientset, err := client.GetClientSet(kubeconfig)
			if err != nil {
				ch.errors <- err
				return
			}

			for namespace := range queue {
				select {
				case <-ch.stopper:
					return
				default:
					processNamespace(clientset, cfg, namespace, ch, nodes)
				}
			}
		}()
	}
}

// GetInventoryReport is an atomic method for getting in-use image results, in parallel for multiple namespaces
//
//nolint:funlen
func GetInventoryReport(cfg *config.Application) (inventory.Report, error) {
	log.Info("Starting image inventory collection")

	kubeconfig, err := client.GetKubeConfig(cfg)
	if err != nil {
		return inventory.Report{}, err
	}

	ch := channels{
		reportItem: make(chan ReportItem),
		errors:     make(chan error),
		stopper:    make(chan struct{}, 1),
	}

	clientset, err := client.GetClientSet(kubeconfig)
	if err != nil {
		return inventory.Report{}, fmt.Errorf("failed to get k8s client set: %w", err)
	}
	client := client.Client{
		Clientset: clientset,
	}

	namespaces, err := inventory.FetchNamespaces(client,
		cfg.Kubernetes.RequestBatchSize, cfg.Kubernetes.RequestTimeoutSeconds,
		cfg.NamespaceSelectors.Exclude, cfg.NamespaceSelectors.Include)
	if err != nil {
		return inventory.Report{}, err
	}

	queue := make(chan inventory.Namespace, len(namespaces)) // fill the queue of namespaces to process
	for _, n := range namespaces {
		queue <- n
	}
	close(queue)

	var nodeMap map[string]inventory.Node
	nodeMap, err = inventory.FetchNodes(
		client,
		cfg.Kubernetes.RequestBatchSize,
		cfg.Kubernetes.RequestTimeoutSeconds,
	)
	if err != nil {
		return inventory.Report{}, err
	}

	launchWorkerPool(cfg, kubeconfig, ch, queue, nodeMap) // get pods/containers from namespaces using a worker pool pattern

	results := make([]ReportItem, 0)
	pods := make([]inventory.Pod, 0)
	containers := make([]inventory.Container, 0)
	processedNamespaces := make([]inventory.Namespace, 0)
	for len(results) < len(namespaces) {
		select {
		case item := <-ch.reportItem:
			results = append(results, item)
			if cfg.NamespaceSelectors.IgnoreEmpty && len(item.Pods) == 0 {
				log.Debugf("Ignoring namespace \"%s\" as it has no pods", item.Namespace.Name)
				continue
			}
			processedNamespaces = append(processedNamespaces, item.Namespace)
			pods = append(pods, item.Pods...)
			containers = append(containers, item.Containers...)
		case err := <-ch.errors:
			close(ch.stopper)
			return inventory.Report{}, err
		case <-time.After(time.Second * time.Duration(cfg.Kubernetes.RequestTimeoutSeconds)):
			return inventory.Report{}, fmt.Errorf("timed out waiting for results")
		}
	}
	close(ch.reportItem)
	close(ch.errors)
	close(ch.stopper) // safe to close here since the other channel close precedes a return statement

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return inventory.Report{}, fmt.Errorf("failed to get Cluster Server Version: %w", err)
	}

	var nodes []inventory.Node
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}

	log.Infof("Got Inventory Report with %d containers running across %d namespaces", len(containers), len(processedNamespaces))
	return inventory.Report{
		Timestamp:             time.Now().UTC().Format(time.RFC3339),
		Containers:            containers,
		Pods:                  pods,
		Namespaces:            processedNamespaces,
		Nodes:                 nodes,
		ServerVersionMetadata: serverVersion,
		ClusterName:           cfg.KubeConfig.Cluster,
	}, nil
}

func processNamespace(
	clientset *kubernetes.Clientset,
	cfg *config.Application,
	ns inventory.Namespace,
	ch channels,
	nodes map[string]inventory.Node,
) {
	v1pods, err := inventory.FetchPodsInNamespace(
		client.Client{Clientset: clientset},
		cfg.Kubernetes.RequestBatchSize,
		cfg.Kubernetes.RequestTimeoutSeconds,
		ns.Name,
	)
	if err != nil {
		ch.errors <- err
		return
	}

	if len(v1pods) == 0 {
		log.Infof("No pods found in namespace \"%s\"", ns.Name)
		ch.reportItem <- ReportItem{
			Namespace: ns,
		}
		return
	}

	pods := inventory.ProcessPods(v1pods, ns.UID, nodes)
	containers := inventory.GetContainersFromPods(
		v1pods,
		cfg.IgnoreNotRunning,
		cfg.MissingRegistryOverride,
		cfg.MissingTagPolicy.Policy,
		cfg.MissingTagPolicy.Tag,
	)

	reportItem := ReportItem{
		Namespace:  ns,
		Pods:       pods,
		Containers: containers,
	}

	log.Infof("There are %d pods in namespace \"%s\"", len(pods), ns.Name)
	ch.reportItem <- reportItem
}

func SetLogger(logger logger.Logger) {
	log.Log = logger
}
