/*
Package watcher maintains a live view of the cluster by watching the Kubernetes
event stream with informers rather than listing every resource on each polling
interval.

As well as removing the periodic LIST load from the api-server, watching the
event stream means that resources which are created and destroyed *between* two
inventory reports are still observed. Those resources are buffered until the
next report is generated so that short lived pods (and the containers within
them) are no longer missed.
*/
package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/anchore/k8s-inventory/internal/log"
)

// The informers are created without a resync period. A client-go resync does
// not contact the api-server or reconcile the cache against it - it only
// redelivers the objects already cached to the event handlers. The handlers
// here just re-store a pointer they already hold, so a resync would cost a
// redelivery of every cached pod and buy nothing.
const noResync = time.Duration(0)

// Config holds the tuning options for a Watcher
type Config struct {
	// RequestTimeout bounds the one-off requests the watcher makes itself,
	// outside of the informers
	RequestTimeout time.Duration
	// CacheSyncTimeout bounds how long Start waits for the informer caches to
	// fill. Zero waits for as long as it takes.
	CacheSyncTimeout time.Duration
	// Namespaces is the configured namespace allow-list. When it names exactly
	// one namespace the pod informer is scoped to it, so that the agent does
	// not cache pods it is going to discard.
	Namespaces []string
}

// Snapshot is a point in time view of the cluster. It contains everything the
// informer caches currently hold plus anything that was deleted since the
// previous snapshot was taken.
//
// The objects it references are owned by the informer caches and must not be
// modified.
type Snapshot struct {
	Namespaces []*v1.Namespace
	Pods       []*v1.Pod
	Nodes      []*v1.Node
}

// Watcher owns the informers for the resources that make up an inventory
// report and buffers deletions between snapshots.
type Watcher struct {
	factory          informers.SharedInformerFactory
	podLister        listersv1.PodLister
	nsLister         listersv1.NamespaceLister
	nodeLister       listersv1.NodeLister
	nodesEnabled     bool
	cacheSyncTimeout time.Duration

	mu sync.Mutex
	// lastRunning holds the most recent Running snapshot of each pod so that a
	// pod which has already completed by the time it is deleted is still
	// reported as it was while running
	lastRunning map[types.UID]*v1.Pod
	// deletedPods is keyed by UID, as a pod that is deleted and recreated under
	// the same name is genuinely two different pods and both should be reported
	deletedPods map[string]*v1.Pod
	// deletedNamespaces and deletedNodes are keyed by name, because pods
	// reference their namespace and node by name. Two objects sharing a name
	// could not be told apart when attributing pods to them, so the most
	// recently deleted one wins and anything still in the cache wins over that.
	deletedNamespaces map[string]*v1.Namespace
	deletedNodes      map[string]*v1.Node
}

// New builds a Watcher for the given clientset
func New(clientset kubernetes.Interface, cfg Config) (*Watcher, error) {
	w := &Watcher{
		factory:           informers.NewSharedInformerFactoryWithOptions(clientset, noResync, factoryOptions(cfg)...),
		cacheSyncTimeout:  cfg.CacheSyncTimeout,
		lastRunning:       make(map[types.UID]*v1.Pod),
		deletedPods:       make(map[string]*v1.Pod),
		deletedNamespaces: make(map[string]*v1.Namespace),
		deletedNodes:      make(map[string]*v1.Node),
	}

	podInformer := w.factory.Core().V1().Pods()
	// managed fields are never reported and are a significant part of the
	// memory footprint of a cluster wide pod cache
	if err := podInformer.Informer().SetTransform(stripManagedFields); err != nil {
		return nil, fmt.Errorf("failed to set pod informer transform: %w", err)
	}
	if _, err := podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.onPodUpsert(obj) },
		UpdateFunc: func(_, obj interface{}) { w.onPodUpsert(obj) },
		DeleteFunc: w.onPodDelete,
	}); err != nil {
		return nil, fmt.Errorf("failed to add pod event handler: %w", err)
	}
	w.podLister = podInformer.Lister()

	nsInformer := w.factory.Core().V1().Namespaces()
	if _, err := nsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: w.onNamespaceDelete,
	}); err != nil {
		return nil, fmt.Errorf("failed to add namespace event handler: %w", err)
	}
	w.nsLister = nsInformer.Lister()

	// Nodes are optional - the agent tolerates not being allowed to read them
	if canListNodes(clientset, cfg.RequestTimeout) {
		nodeInformer := w.factory.Core().V1().Nodes()
		if _, err := nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			DeleteFunc: w.onNodeDelete,
		}); err != nil {
			return nil, fmt.Errorf("failed to add node event handler: %w", err)
		}
		w.nodeLister = nodeInformer.Lister()
		w.nodesEnabled = true
	}

	return w, nil
}

// factoryOptions scopes the pod informer to a single namespace when that is all
// the configuration asks for. Namespace and node informers are cluster scoped
// and are unaffected by this.
func factoryOptions(cfg Config) []informers.SharedInformerOption {
	if len(cfg.Namespaces) != 1 {
		return nil
	}
	log.Infof("Watching pods in namespace %s only", cfg.Namespaces[0])
	return []informers.SharedInformerOption{informers.WithNamespace(cfg.Namespaces[0])}
}

// Start begins watching and blocks until every informer cache has synced.
//
// Syncing is bounded by the configured cache sync timeout because informers
// retry a rejected LIST indefinitely, which would otherwise leave the agent
// waiting forever on a cluster it is not permitted to watch rather than
// reporting an error.
func (w *Watcher) Start(stopCh <-chan struct{}) error {
	w.factory.Start(stopCh)

	syncCh := stopCh
	if w.cacheSyncTimeout > 0 {
		deadlineCh := make(chan struct{})
		synced := make(chan struct{})
		defer close(synced)
		go func() {
			timer := time.NewTimer(w.cacheSyncTimeout)
			defer timer.Stop()
			select {
			case <-stopCh:
			case <-timer.C:
			case <-synced:
			}
			close(deadlineCh)
		}()
		syncCh = deadlineCh
	}

	for informerType, hasSynced := range w.factory.WaitForCacheSync(syncCh) {
		if !hasSynced {
			return fmt.Errorf("failed to sync informer cache for %s within %s", informerType, w.cacheSyncTimeout)
		}
	}
	return nil
}

// Snapshot returns the current contents of the informer caches merged with
// everything deleted since the last call. The deletion buffers are reset so
// that each deleted resource is reported exactly once.
func (w *Watcher) Snapshot() (Snapshot, error) {
	// held across the lister reads so that the deletion buffers and the caches
	// they are merged with cannot move relative to each other
	w.mu.Lock()
	defer w.mu.Unlock()

	pods, err := w.podLister.List(labels.Everything())
	if err != nil {
		return Snapshot{}, fmt.Errorf("failed to list pods from informer cache: %w", err)
	}
	namespaces, err := w.nsLister.List(labels.Everything())
	if err != nil {
		return Snapshot{}, fmt.Errorf("failed to list namespaces from informer cache: %w", err)
	}
	var nodes []*v1.Node
	if w.nodesEnabled {
		nodes, err = w.nodeLister.List(labels.Everything())
		if err != nil {
			return Snapshot{}, fmt.Errorf("failed to list nodes from informer cache: %w", err)
		}
	}

	snapshot := Snapshot{
		Pods:       mergeDeleted(pods, w.deletedPods, uidKey),
		Namespaces: mergeDeleted(namespaces, w.deletedNamespaces, nameKey),
		Nodes:      mergeDeleted(nodes, w.deletedNodes, nameKey),
	}

	log.Debugf(
		"Snapshot taken from informer caches: %d namespaces, %d pods, %d nodes (including %d deleted pods)",
		len(snapshot.Namespaces), len(snapshot.Pods), len(snapshot.Nodes), len(w.deletedPods),
	)

	w.deletedPods = make(map[string]*v1.Pod)
	w.deletedNamespaces = make(map[string]*v1.Namespace)
	w.deletedNodes = make(map[string]*v1.Node)

	// lastRunning is deliberately not pruned against the pods just listed. The
	// informer removes an object from its cache on one goroutine and delivers
	// the delete event on another, so a pod can be absent from the listing while
	// its delete event is still queued. Evicting it here would leave nothing for
	// onPodDelete to buffer but the pod's final, possibly completed, state,
	// which ignore-not-running would then discard - losing exactly the transient
	// container this exists to capture. Every removal from the cache delivers a
	// delete event, including the tombstones a relist synthesises after a watch
	// gap, and onPodDelete always drops the entry, so the map stays bounded.

	return snapshot, nil
}

// NodesEnabled reports whether the agent is able to watch nodes
func (w *Watcher) NodesEnabled() bool {
	return w.nodesEnabled
}

func (w *Watcher) onPodUpsert(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}
	// Only running pods are remembered. A pod that comes and goes between two
	// reports is reported as it was while running, which keeps the behaviour of
	// the ignore-not-running option intact.
	if pod.Status.Phase != v1.PodRunning {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastRunning[pod.UID] = pod
}

func (w *Watcher) onPodDelete(obj interface{}) {
	pod, ok := fromTombstone[*v1.Pod](obj)
	if !ok {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if lastRunning, ok := w.lastRunning[pod.UID]; ok {
		// prefer the running snapshot, it is the one that carries the container
		// statuses (and therefore the image digests) we want to report
		w.deletedPods[string(pod.UID)] = lastRunning
		delete(w.lastRunning, pod.UID)
	} else {
		w.deletedPods[string(pod.UID)] = pod
	}
	log.Debugf("Buffered deleted pod %s/%s for the next inventory report", pod.Namespace, pod.Name)
}

func (w *Watcher) onNamespaceDelete(obj interface{}) {
	namespace, ok := fromTombstone[*v1.Namespace](obj)
	if !ok {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// buffered so that pods deleted along with their namespace still have a
	// parent namespace in the report
	w.deletedNamespaces[namespace.Name] = namespace
}

func (w *Watcher) onNodeDelete(obj interface{}) {
	node, ok := fromTombstone[*v1.Node](obj)
	if !ok {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.deletedNodes[node.Name] = node
}

func uidKey(obj metav1.Object) string  { return string(obj.GetUID()) }
func nameKey(obj metav1.Object) string { return obj.GetName() }

// mergeDeleted combines the objects currently held in an informer cache with
// the buffered deleted objects, ignoring any deleted object whose key is back
// in the cache
func mergeDeleted[PT metav1.Object](current []PT, deleted map[string]PT, keyOf func(metav1.Object) string) []PT {
	merged := make([]PT, 0, len(current)+len(deleted))
	present := make(map[string]struct{}, len(current))
	for _, obj := range current {
		present[keyOf(obj)] = struct{}{}
		merged = append(merged, obj)
	}
	for key, obj := range deleted {
		if _, ok := present[key]; ok {
			continue
		}
		merged = append(merged, obj)
	}
	return merged
}

// fromTombstone extracts the deleted object from a delete event, unwrapping the
// tombstone the informer delivers when it missed the delete itself
func fromTombstone[T any](obj interface{}) (T, bool) {
	if typed, ok := obj.(T); ok {
		return typed, true
	}
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if typed, ok := tombstone.Obj.(T); ok {
			return typed, true
		}
	}
	log.Debugf("Ignoring delete event for unexpected object type %T", obj)
	var zero T
	return zero, false
}

// stripManagedFields drops the managedFields metadata before an object is put
// into an informer cache. It is never reported and is a large part of the
// memory footprint of a cluster wide cache.
func stripManagedFields(obj interface{}) (interface{}, error) {
	if accessor, ok := obj.(metav1.ObjectMetaAccessor); ok {
		accessor.GetObjectMeta().SetManagedFields(nil)
	}
	return obj, nil
}

// canListNodes probes whether the agent is permitted to read nodes. Listing
// nodes is optional - the poll based collection tolerates being forbidden from
// doing so and the informer based collection does the same.
//
// Only being forbidden disables the node informer. Any other failure is treated
// as transient and left to the informer to retry.
func canListNodes(clientset kubernetes.Interface, requestTimeout time.Duration) bool {
	ctx := context.Background()
	if requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, requestTimeout)
		defer cancel()
	}

	limit := int64(1)
	_, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil && k8sErrors.IsForbidden(err) {
		log.Warnf("not permitted to watch nodes, node inventory will not be collected: %v", err)
		return false
	}
	return true
}
