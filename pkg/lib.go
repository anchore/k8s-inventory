/*
Package retrieves Kubernetes In-Use Image data from the Kubernetes API. Runs adhoc and periodically, using the
k8s go SDK
*/package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	jstime "github.com/anchore/k8s-inventory/internal/time"
	"github.com/anchore/k8s-inventory/pkg/integration"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/pkg/client"
	"github.com/anchore/k8s-inventory/pkg/healthreporter"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/anchore/k8s-inventory/pkg/logger"
	"github.com/anchore/k8s-inventory/pkg/reporter"
	"github.com/anchore/k8s-inventory/pkg/watcher"
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

type AccountRoutedReports map[string]inventory.Report
type BatchedReports map[string][]inventory.Report

type batchState struct {
	currNS   []inventory.Namespace
	currPods []inventory.Pod
	currCont []inventory.Container
	currNode map[string]inventory.Node
	currSize int
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

func HandleReport(report inventory.Report, reportInfo *healthreporter.InventoryReportInfo, cfg *config.Application, account string) error {
	if cfg.VerboseInventoryReports {
		err := reportToStdout(report)
		if err != nil {
			log.Errorf("Failed to output Inventory Report: %w", err)
		}
	}

	anchoreDetails := cfg.AnchoreDetails
	// Look for account credentials in the account routes first then fall back to the global anchore credentials
	if account == "" {
		return fmt.Errorf("account name is required")
	}
	anchoreDetails.Account = account
	if cfg.AccountRoutes != nil {
		if route, ok := cfg.AccountRoutes[account]; ok {
			log.Debugf("Using account details specified from account-routes config for account %s", account)
			anchoreDetails.User = route.User
			anchoreDetails.Password = route.Password
		} else {
			log.Debugf("Using default account details for account %s", account)
		}
	} else {
		log.Debugf("Using default account details for account %s", account)
	}

	if anchoreDetails.IsValid() {
		reportInfo.SentAsUser = anchoreDetails.User
		if err := reporter.Post(report, anchoreDetails); err != nil {
			if errors.Is(err, reporter.ErrAnchoreAccountDoesNotExist) {
				return err
			}
			return fmt.Errorf("unable to report Inventory to Anchore account %s: %w", account, err)
		}
		log.Infof("Inventory report sent to Anchore account %s", account)
	} else {
		log.Info("Anchore details not specified, not reporting inventory")
	}
	return nil
}

// PeriodicallyGetInventoryReport periodically retrieve image results and report/output them according to the configuration.
// Note: Errors do not cause the function to exit, since this is periodically running
func PeriodicallyGetInventoryReport(cfg *config.Application, ch integration.Channels, gatedReportInfo *healthreporter.GatedReportInfo) {
	// Wait for registration with Enterprise to be disabled or completed
	<-ch.InventoryReportingEnabled
	log.Info("Inventory reporting started")
	healthReportingEnabled := false

	// Fire off a ticker that reports according to a configurable polling interval
	ticker := time.NewTicker(time.Duration(cfg.PollingIntervalSeconds) * time.Second)

	collector := &inventoryCollector{}
	for {
		reports, err := collector.collect(cfg)
		if err != nil {
			log.Errorf("Failed to get Inventory Report: %v", err)
		} else {
			processAndSendReports(cfg, reports, ch, gatedReportInfo, &healthReportingEnabled)
		}

		log.Infof("Waiting %d seconds for next report...", cfg.PollingIntervalSeconds)

		// Wait at least as long as the ticker
		log.Debugf("Start new gather: %s", <-ticker.C)
	}
}

// How long to wait before retrying a failed attempt to start watching the
// event stream. It grows on each consecutive failure so that a cluster the
// agent is not allowed to watch settles into steady polling instead of
// re-listing on every reporting interval.
const (
	eventStreamInitialRetryDelay = time.Minute
	eventStreamMaxRetryDelay     = time.Hour
)

// eventStream is a started, synced set of informers and the client they were
// built from
type eventStream struct {
	watcher   *watcher.Watcher
	clientset kubernetes.Interface
}

// inventoryCollector gathers the inventory for one reporting interval, either
// from the Kubernetes event stream or by polling the api-server, according to
// the configured collection method.
type inventoryCollector struct {
	// guards the state shared with the goroutine that starts the informers
	mu       sync.Mutex
	stream   *eventStream
	starting bool
	retryIn  time.Duration

	// only touched while building a report
	serverVersion *version.Info
}

// collect builds the reports for a single reporting interval
func (c *inventoryCollector) collect(cfg *config.Application) (BatchedReports, error) {
	stream := c.eventStream(cfg)
	if stream == nil {
		return GetInventoryReports(cfg)
	}

	snapshot, err := stream.watcher.Snapshot()
	if err != nil {
		return BatchedReports{}, err
	}

	return GetInventoryReportsFromSnapshot(cfg, snapshot, c.refreshServerVersion(stream)), nil
}

// eventStream returns the synced event stream to collect this interval's
// inventory from, or nil if it should be collected by polling instead.
//
// Starting the informers means waiting for their initial listing of the whole
// cluster, so it happens in the background: reporting carries on by polling and
// picks the event stream up on the first interval after it has synced. The
// reporting cadence is therefore never held up by the cache sync deadline.
func (c *inventoryCollector) eventStream(cfg *config.Application) *eventStream {
	if cfg.InventoryCollection.Method != config.InventoryCollectionMethodInformer {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stream != nil {
		return c.stream
	}
	if !c.starting {
		c.starting = true
		go c.startWatching(cfg)
	}
	return nil
}

// startWatching brings up the informers in the background and publishes them
// once their caches have synced. On failure everything it started is stopped
// again and another attempt is allowed after a growing delay.
func (c *inventoryCollector) startWatching(cfg *config.Application) {
	stream, err := newEventStream(cfg)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err == nil {
		log.Info("Watching the Kubernetes event stream for inventory changes")
		c.stream = stream
		c.starting = false
		c.retryIn = 0
		return
	}

	c.retryIn = nextRetryDelay(c.retryIn)
	log.Warnf("Unable to collect inventory from the Kubernetes event stream: %v", err)
	log.Infof("Polling the api-server for inventory, the event stream will be retried in %s", c.retryIn)

	// keep this attempt marked as in progress until the delay has passed, so
	// that the reporting loop does not start another one in the meantime
	time.AfterFunc(c.retryIn, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.starting = false
	})
}

func nextRetryDelay(current time.Duration) time.Duration {
	if current <= 0 {
		return eventStreamInitialRetryDelay
	}
	if next := current * 2; next < eventStreamMaxRetryDelay {
		return next
	}
	return eventStreamMaxRetryDelay
}

// newEventStream builds the informers and blocks until their caches have synced
func newEventStream(cfg *config.Application) (*eventStream, error) {
	kubeconfig, err := client.GetKubeConfig(cfg)
	if err != nil {
		return nil, err
	}

	clientset, err := client.GetClientSet(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s client set: %w", err)
	}

	informerCfg := cfg.InventoryCollection.Informer
	w, err := watcher.New(clientset, watcher.Config{
		RequestTimeout:   time.Duration(cfg.Kubernetes.RequestTimeoutSeconds) * time.Second,
		CacheSyncTimeout: time.Duration(informerCfg.CacheSyncTimeoutSeconds) * time.Second,
		Namespaces:       cfg.NamespaceSelectors.Include,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes watcher: %w", err)
	}

	// A fresh client and factory are built for each attempt on purpose - the
	// informers of a failed attempt are stopped and cannot be restarted.
	stopCh := make(chan struct{})
	if err := w.Start(stopCh); err != nil {
		close(stopCh)
		return nil, fmt.Errorf("failed to watch the kubernetes event stream: %w", err)
	}

	return &eventStream{watcher: w, clientset: clientset}, nil
}

// refreshServerVersion re-reads the cluster server version that reports are
// stamped with, so that a cluster upgrade is picked up without restarting the
// agent. The last known version is kept if the lookup fails.
func (c *inventoryCollector) refreshServerVersion(stream *eventStream) *version.Info {
	serverVersion, err := stream.clientset.Discovery().ServerVersion()
	if err != nil {
		log.Warnf("Failed to get Cluster Server Version, reporting the last known version: %v", err)
		return c.serverVersion
	}

	c.serverVersion = serverVersion
	return c.serverVersion
}

// processAndSendReports sends every batched report to Anchore, recording the
// outcome of each batch for the health report
func processAndSendReports(
	cfg *config.Application,
	reports BatchedReports,
	ch integration.Channels,
	gatedReportInfo *healthreporter.GatedReportInfo,
	healthReportingEnabled *bool,
) {
	for account, reportsForAccount := range reports {
		reportInfo := healthreporter.InventoryReportInfo{
			Account:             account,
			BatchSize:           len(reportsForAccount),
			LastSuccessfulIndex: -1,
			Batches:             make([]healthreporter.BatchInfo, 0),
			HasErrors:           false,
		}
		for count, report := range reportsForAccount {
			log.Infof("Sending Inventory Report to Anchore Account %s, %d of %d", account, count+1, len(reportsForAccount))

			reportInfo.ReportTimestamp = report.Timestamp
			batchInfo := healthreporter.BatchInfo{
				SendTimestamp: jstime.Datetime{Time: time.Now().UTC()},
				BatchIndex:    count + 1,
			}

			err := HandleReport(report, &reportInfo, cfg, account)
			if errors.Is(err, reporter.ErrAnchoreAccountDoesNotExist) {
				// record this error for the health report even if the retry works
				batchInfo.Error = fmt.Sprintf("%s (%s) | ", err.Error(), account)
				reportInfo.HasErrors = true

				// Retry with default account
				retryAccount := cfg.AnchoreDetails.Account
				if cfg.AccountRouteByNamespaceLabel.DefaultAccount != "" {
					retryAccount = cfg.AccountRouteByNamespaceLabel.DefaultAccount
				}
				log.Warnf("Error sending to Anchore Account %s, sending to default account", account)
				err = HandleReport(report, &reportInfo, cfg, retryAccount)
			}
			if err != nil {
				log.Errorf("Failed to handle Inventory Report: %w", err)
				// append the error to any error that happened during a retry, so we record both failures
				batchInfo.Error += err.Error()
				reportInfo.HasErrors = true
			} else {
				reportInfo.LastSuccessfulIndex = count + 1
			}

			select {
			case isEnabled, isNotClosed := <-ch.HealthReportingEnabled:
				if isNotClosed {
					*healthReportingEnabled = isEnabled
				}
				log.Infof("Health reporting enabled: %t", *healthReportingEnabled)
			default:
			}
			if *healthReportingEnabled {
				reportInfo.Batches = append(reportInfo.Batches, batchInfo)
				healthreporter.SetReportInfoNoBlocking(account, count, reportInfo, gatedReportInfo)
			}
		}
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

// GetInventoryReportForNamespaces is an atomic method for getting in-use image results, in parallel for multiple namespaces
//
//nolint:funlen
func GetInventoryReportForNamespaces(
	cfg *config.Application,
	namespaces []inventory.Namespace,
) (inventory.Report, error) {
	nsNames := make([]string, 0)
	for _, ns := range namespaces {
		nsNames = append(nsNames, ns.Name)
	}
	log.Info("Starting inventory collection for namespaces: ", nsNames)

	kubeconfig, err := client.GetKubeConfig(cfg)
	if err != nil {
		return inventory.Report{}, err
	}

	clientset, err := client.GetClientSet(kubeconfig)
	if err != nil {
		return inventory.Report{}, fmt.Errorf("failed to get k8s client set: %w", err)
	}
	client := client.Client{
		Clientset: clientset,
	}

	ch := channels{
		reportItem: make(chan ReportItem),
		errors:     make(chan error),
		stopper:    make(chan struct{}, 1),
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
		cfg.MetadataCollection.Nodes.Annotations,
		cfg.MetadataCollection.Nodes.Labels,
		cfg.MetadataCollection.Nodes.Disable,
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

// snapshotInventory is the account agnostic view of a cluster snapshot, indexed
// so that it can be sliced up per Anchore account
type snapshotInventory struct {
	namespaces            []inventory.Namespace
	nodes                 []inventory.Node
	podsByNamespace       map[string][]inventory.Pod
	containersByNamespace map[string][]inventory.Container
}

// processSnapshot converts a snapshot of the cluster into inventory types,
// applying the configured namespace selectors and metadata collection rules
func processSnapshot(cfg *config.Application, snapshot watcher.Snapshot) snapshotInventory {
	nodeMap := make(map[string]inventory.Node, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		nodeMap[node.Name] = inventory.NodeFromV1(
			node,
			cfg.MetadataCollection.Nodes.Annotations,
			cfg.MetadataCollection.Nodes.Labels,
			cfg.MetadataCollection.Nodes.Disable,
		)
	}

	podsByNamespaceName := make(map[string][]v1.Pod)
	for _, pod := range snapshot.Pods {
		podsByNamespaceName[pod.Namespace] = append(podsByNamespaceName[pod.Namespace], *pod)
	}

	inv := snapshotInventory{
		nodes:                 make([]inventory.Node, 0, len(nodeMap)),
		podsByNamespace:       make(map[string][]inventory.Pod),
		containersByNamespace: make(map[string][]inventory.Container),
	}
	for _, node := range nodeMap {
		inv.nodes = append(inv.nodes, node)
	}

	// Pods reference their namespace by name, so a namespace that was deleted
	// and recreated within one interval cannot have its pods attributed to one
	// object or the other. Each name is processed once, taking the first
	// occurrence, which the watcher orders as the one still in the cluster.
	seen := make(map[string]struct{}, len(snapshot.Namespaces))
	filter := inventory.NewNamespaceFilter(cfg.NamespaceSelectors.Exclude, cfg.NamespaceSelectors.Include)
	for _, n := range snapshot.Namespaces {
		if !filter.Keep(n.Name) {
			continue
		}
		if _, duplicate := seen[n.Name]; duplicate {
			log.Debugf("Namespace \"%s\" appears more than once in the snapshot, only reporting the first", n.Name)
			continue
		}
		seen[n.Name] = struct{}{}

		v1pods := podsByNamespaceName[n.Name]
		if cfg.NamespaceSelectors.IgnoreEmpty && len(v1pods) == 0 {
			log.Debugf("Ignoring namespace \"%s\" as it has no pods", n.Name)
			continue
		}

		ns := inventory.NamespaceFromV1(
			n,
			cfg.MetadataCollection.Namespace.Annotations,
			cfg.MetadataCollection.Namespace.Labels,
			cfg.MetadataCollection.Namespace.Disable,
		)
		inv.namespaces = append(inv.namespaces, ns)
		inv.podsByNamespace[ns.UID] = inventory.ProcessPods(
			v1pods, ns.UID, nodeMap,
			cfg.MetadataCollection.Pods.Annotations,
			cfg.MetadataCollection.Pods.Labels,
			cfg.MetadataCollection.Pods.Disable,
		)
		inv.containersByNamespace[ns.UID] = inventory.GetContainersFromPods(
			v1pods,
			cfg.IgnoreNotRunning,
			cfg.MissingRegistryOverride,
			cfg.MissingTagPolicy.Policy,
			cfg.MissingTagPolicy.Tag,
		)
	}

	return inv
}

// reportForNamespaces builds a report containing the given subset of the
// snapshot's namespaces. As with the polling collection method every node is
// included in each report regardless of which namespaces it is carrying.
func (inv snapshotInventory) reportForNamespaces(
	cfg *config.Application,
	namespaces []inventory.Namespace,
	serverVersion *version.Info,
	timestamp string,
) inventory.Report {
	pods := make([]inventory.Pod, 0)
	containers := make([]inventory.Container, 0)
	for _, ns := range namespaces {
		pods = append(pods, inv.podsByNamespace[ns.UID]...)
		containers = append(containers, inv.containersByNamespace[ns.UID]...)
	}

	log.Infof("Got Inventory Report with %d containers running across %d namespaces", len(containers), len(namespaces))
	return inventory.Report{
		Timestamp:             timestamp,
		Containers:            containers,
		Pods:                  pods,
		Namespaces:            namespaces,
		Nodes:                 inv.nodes,
		ServerVersionMetadata: serverVersion,
		ClusterName:           cfg.KubeConfig.Cluster,
	}
}

// GetInventoryReportsFromSnapshot builds the account routed, batched inventory
// reports for a snapshot of the cluster taken from the Kubernetes event stream
func GetInventoryReportsFromSnapshot(cfg *config.Application, snapshot watcher.Snapshot, serverVersion *version.Info) BatchedReports {
	log.Info("Starting image inventory collection from the Kubernetes event stream")

	inv := processSnapshot(cfg, snapshot)
	timestamp := time.Now().UTC().Format(time.RFC3339)

	reports := AccountRoutedReports{}
	if len(cfg.AccountRoutes) == 0 && cfg.AccountRouteByNamespaceLabel.LabelKey == "" {
		reports[cfg.AnchoreDetails.Account] = inv.reportForNamespaces(cfg, inv.namespaces, serverVersion, timestamp)
	} else {
		accountRoutesForAllNamespaces := GetAccountRoutedNamespaces(
			cfg.AnchoreDetails.Account, inv.namespaces, cfg.AccountRoutes, cfg.AccountRouteByNamespaceLabel)

		for account, namespaces := range accountRoutesForAllNamespaces {
			reports[account] = inv.reportForNamespaces(cfg, namespaces, serverVersion, timestamp)
		}
	}

	return getBatchedInventoryReports(reports, cfg.InventoryReportLimits)
}

func GetAllNamespaces(cfg *config.Application) ([]inventory.Namespace, error) {
	kubeconfig, err := client.GetKubeConfig(cfg)
	if err != nil {
		return []inventory.Namespace{}, err
	}

	clientset, err := client.GetClientSet(kubeconfig)
	if err != nil {
		return []inventory.Namespace{}, fmt.Errorf("failed to get k8s client set: %w", err)
	}
	client := client.Client{
		Clientset: clientset,
	}

	namespaces, err := inventory.FetchNamespaces(client,
		cfg.Kubernetes.RequestBatchSize, cfg.Kubernetes.RequestTimeoutSeconds,
		cfg.NamespaceSelectors.Exclude, cfg.NamespaceSelectors.Include,
		cfg.MetadataCollection.Namespace.Annotations, cfg.MetadataCollection.Namespace.Labels,
		cfg.MetadataCollection.Namespace.Disable)
	if err != nil {
		return []inventory.Namespace{}, err
	}

	log.Infof("Found %d namespaces", len(namespaces))

	return namespaces, nil
}

func GetAccountRoutedNamespaces(defaultAccount string, namespaces []inventory.Namespace,
	accountRoutes config.AccountRoutes, namespaceLabelRouting config.AccountRouteByNamespaceLabel) map[string][]inventory.Namespace {
	accountRoutesForAllNamespaces := make(map[string][]inventory.Namespace)

	if namespaceLabelRouting.DefaultAccount != "" {
		defaultAccount = namespaceLabelRouting.DefaultAccount
	}

	accountNamespaces := make(map[string]struct{})
	for routeNS, route := range accountRoutes {
		for _, ns := range namespaces {
			for _, namespaceRegex := range route.Namespaces {
				if regexp.MustCompile(namespaceRegex).MatchString(ns.Name) {
					log.Debugf("Namespace %s matched route from config %s", ns.Name, routeNS)
					accountNamespaces[ns.Name] = struct{}{}
					accountRoutesForAllNamespaces[routeNS] = append(accountRoutesForAllNamespaces[routeNS], ns)
				}
			}
		}
	}

	// If there is a namespace label routing, add namespaces to the account routes based on the label,
	// if the namespace has not already been added to an account route set via explicit configuration in
	// accountRoutes config. (This overrides the label routing for the case where the label cannot be changed).
	// Otherwise, add namespaces that are not in any account route to the default account unless disabled.
	for _, ns := range namespaces {
		_, namespaceRouted := accountNamespaces[ns.Name]
		if namespaceLabelRouting.LabelKey != "" && !namespaceRouted {
			if account, ok := ns.Labels[namespaceLabelRouting.LabelKey]; ok {
				log.Debugf("Namespace %s matched route from label %s", ns.Name, account)
				accountRoutesForAllNamespaces[account] = append(accountRoutesForAllNamespaces[account], ns)
			} else if !namespaceLabelRouting.IgnoreMissingLabel {
				accountRoutesForAllNamespaces[defaultAccount] = append(accountRoutesForAllNamespaces[defaultAccount], ns)
			} else {
				log.Infof("Ignoring namespace %s because it does not have the label %s", ns.Name, namespaceLabelRouting.LabelKey)
			}
		} else if !namespaceRouted {
			accountRoutesForAllNamespaces[defaultAccount] = append(accountRoutesForAllNamespaces[defaultAccount], ns)
			log.Debugf("Namespace %s added to default account %s", ns.Name, defaultAccount)
		}
	}

	return accountRoutesForAllNamespaces
}

func GetNamespacesBatches(namespaces []inventory.Namespace, batchSize int) [][]inventory.Namespace {
	batches := make([][]inventory.Namespace, 0)
	if batchSize <= 0 {
		return append(batches, namespaces)
	}
	for i := 0; i < len(namespaces); i += batchSize {
		end := i + batchSize
		if end > len(namespaces) {
			end = len(namespaces)
		}
		batches = append(batches, namespaces[i:end])
	}
	return batches
}

func GetInventoryReports(cfg *config.Application) (BatchedReports, error) {
	log.Info("Starting image inventory collection")

	reports := AccountRoutedReports{}
	namespaces, _ := GetAllNamespaces(cfg)

	if len(cfg.AccountRoutes) == 0 && cfg.AccountRouteByNamespaceLabel.LabelKey == "" {
		allNamespacesReport, err := GetInventoryReportForNamespaces(cfg, namespaces)
		if err != nil {
			return BatchedReports{}, err
		}
		reports[cfg.AnchoreDetails.Account] = allNamespacesReport
	} else {
		accountRoutesForAllNamespaces := GetAccountRoutedNamespaces(cfg.AnchoreDetails.Account, namespaces, cfg.AccountRoutes, cfg.AccountRouteByNamespaceLabel)

		for account, namespaces := range accountRoutesForAllNamespaces {
			nsNames := make([]string, 0)
			for _, ns := range namespaces {
				nsNames = append(nsNames, ns.Name)
			}
			log.Infof("Namespaces for account %s : %s", account, nsNames)
		}

		// Get inventory reports for each account
		for account, namespaces := range accountRoutesForAllNamespaces {
			accountReport, err := GetInventoryReportForNamespaces(cfg, namespaces)
			if err != nil {
				return BatchedReports{}, err
			}
			reports[account] = accountReport
		}
	}

	return getBatchedInventoryReports(reports, cfg.InventoryReportLimits), nil
}

func (state *batchState) createReportBatch(accountReport inventory.Report) *inventory.Report {
	if len(state.currNS) == 0 {
		return nil
	}

	// Flatten map[string]inventory.Node → []inventory.Node
	nodes := make([]inventory.Node, 0, len(state.currNode))
	for _, n := range state.currNode {
		nodes = append(nodes, n)
	}

	// Build the new inventory report
	rpt := inventory.Report{
		Timestamp:             accountReport.Timestamp,
		Namespaces:            state.currNS,
		Pods:                  state.currPods,
		Containers:            state.currCont,
		Nodes:                 nodes,
		ServerVersionMetadata: accountReport.ServerVersionMetadata,
		ClusterName:           accountReport.ClusterName,
	}

	// Reset batch state
	*state = batchState{
		currNode: make(map[string]inventory.Node),
	}

	return &rpt
}

// Lookup tables are used to retrieve all the pods, containers, and nodes
// associated with each namespace when created batched reports
type inventoryLookups struct {
	nodeMap               map[string]inventory.Node
	podMap                map[string]inventory.Pod
	podsByNamespace       map[string][]inventory.Pod
	containersByNamespace map[string][]inventory.Container
}

func buildLookups(accountReport inventory.Report) inventoryLookups {
	// nodeMap: UID -> Node
	nodeMap := make(map[string]inventory.Node, len(accountReport.Nodes))
	for _, node := range accountReport.Nodes {
		nodeMap[node.UID] = node
	}
	// podMap: UID -> Pod
	podMap := make(map[string]inventory.Pod, len(accountReport.Pods))
	for _, pod := range accountReport.Pods {
		podMap[pod.UID] = pod
	}
	// podsByNamespace: namespaceUID -> []pods
	podsByNamespace := make(map[string][]inventory.Pod)
	for _, pod := range accountReport.Pods {
		podsByNamespace[pod.NamespaceUID] = append(podsByNamespace[pod.NamespaceUID], pod)
	}
	// containersByNamespace: namespaceUID -> []containers
	containersByNamespace := make(map[string][]inventory.Container)
	for _, ctr := range accountReport.Containers {
		nsUID := podMap[ctr.PodUID].NamespaceUID
		containersByNamespace[nsUID] = append(containersByNamespace[nsUID], ctr)
	}

	return inventoryLookups{
		nodeMap:               nodeMap,
		podMap:                podMap,
		podsByNamespace:       podsByNamespace,
		containersByNamespace: containersByNamespace,
	}
}

//nolint:gocognit
func getBatchedInventoryReports(reports AccountRoutedReports, limits config.InventoryReportLimits) BatchedReports {
	batchCount := 0
	batched := BatchedReports{}
	for account, accountReport := range reports {
		// Check if batching is enabled
		if limits.PayloadThresholdBytes <= 0 && limits.Namespaces <= 0 {
			batched[account] = append(batched[account], accountReport)
			continue
		}

		// We are batching - build lookup tables and init state tracking
		lookups := buildLookups(accountReport)
		state := batchState{
			currNode: make(map[string]inventory.Node),
			currSize: 0,
		}

		// Iterate over all namespaces, watching for when we exceed our max payload threshold
		for _, ns := range accountReport.Namespaces {
			// Calculate the set of Nodes referenced by all Pods in the Namespace
			newNodes := make(map[string]inventory.Node)
			for _, p := range lookups.podsByNamespace[ns.UID] {
				if _, exists := state.currNode[p.NodeUID]; !exists {
					newNodes[p.NodeUID] = lookups.nodeMap[p.NodeUID]
				}
			}

			var payloadLength = 0
			if limits.PayloadThresholdBytes > 0 {
				// Flatten to a list - this is just used for sizing up the additional payload bytes
				nodesArr := make([]inventory.Node, 0, len(newNodes))
				for _, node := range newNodes {
					nodesArr = append(nodesArr, node)
				}

				// Size up a report with just this new info, not the entire 'state' of the batch
				// NOTE: This isn't going to find the precise incremental size of adding this namespace,
				//       but it's a close enough approximation for batching purposes
				nextRecord := inventory.Report{
					Namespaces: []inventory.Namespace{ns},
					Pods:       lookups.podsByNamespace[ns.UID],
					Containers: lookups.containersByNamespace[ns.UID],
					Nodes:      nodesArr,
				}
				sizeNext, _ := json.Marshal(nextRecord)
				payloadLength = len(sizeNext)
			}

			// Now we can add this namespace into the batch
			//  - Namespaces, Containers, and Pods are appended
			//  - Nodes are merged into the currNode map
			state.currSize += payloadLength
			state.currNS = append(state.currNS, ns)
			state.currPods = append(state.currPods, lookups.podsByNamespace[ns.UID]...)
			state.currCont = append(state.currCont, lookups.containersByNamespace[ns.UID]...)
			for k, v := range newNodes {
				state.currNode[k] = v
			}

			// Check if the batch is full after having added this namespace
			if (limits.PayloadThresholdBytes > 0 && state.currSize >= limits.PayloadThresholdBytes) ||
				(limits.Namespaces > 0 && len(state.currNS) >= limits.Namespaces) {
				if rpt := state.createReportBatch(accountReport); rpt != nil {
					batched[account] = append(batched[account], *rpt)
					batchCount++
				}
			}
		}

		// Emit tail batch (if any).
		if rpt := state.createReportBatch(accountReport); rpt != nil {
			batched[account] = append(batched[account], *rpt)
			batchCount++
		}
	}

	log.Infof("Finished batching %d inventory reports (threshold = %d namespaces, %d bytes)", batchCount, limits.Namespaces, limits.PayloadThresholdBytes)
	return batched
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

	pods := inventory.ProcessPods(v1pods, ns.UID, nodes, cfg.MetadataCollection.Pods.Annotations, cfg.MetadataCollection.Pods.Labels, cfg.MetadataCollection.Pods.Disable)
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
