package watcher

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

const eventTimeout = 5 * time.Second

func newPod(name string, uid types.UID, phase v1.PodPhase) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       uid,
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubelet"},
			},
		},
		Spec: v1.PodSpec{
			NodeName:   "test-node",
			Containers: []v1.Container{{Name: name, Image: "anchore/test:latest"}},
		},
		Status: v1.PodStatus{Phase: phase},
	}
}

func newNamespace(name string) *v1.Namespace {
	return &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(name + "-uid")}}
}

func newNode(name string) *v1.Node {
	return &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(name + "-uid")}}
}

// startWatcher builds a watcher against the given clientset and runs it until
// the test finishes
func startWatcher(t *testing.T, clientset kubernetes.Interface) *Watcher {
	t.Helper()

	w, err := New(clientset, Config{CacheSyncTimeout: eventTimeout})
	require.NoError(t, err)

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	require.NoError(t, w.Start(stopCh))

	return w
}

// waitFor polls the watcher state until the condition holds. Informers deliver
// events to handlers asynchronously, so tests have to wait for the watcher to
// have observed an event rather than assuming it already has.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	require.Eventually(t, condition, eventTimeout, 5*time.Millisecond)
}

// waitForCachedPod waits until the pod informer cache holds the named pod in
// the given phase
func waitForCachedPod(t *testing.T, w *Watcher, name string, phase v1.PodPhase) {
	t.Helper()
	waitFor(t, func() bool {
		pod, err := w.podLister.Pods("default").Get(name)
		return err == nil && pod.Status.Phase == phase
	})
}

// underLock runs a read of the watcher's internal state with the mutex held,
// as the informer handler goroutines are still running during the tests
func underLock[T any](w *Watcher, read func() T) T {
	w.mu.Lock()
	defer w.mu.Unlock()
	return read()
}

// waitForBuffered waits until the given check of the watcher's buffers holds
func waitForBuffered(t *testing.T, w *Watcher, check func() bool) {
	t.Helper()
	waitFor(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return check()
	})
}

func namespaceNames(snapshot Snapshot) []string {
	names := make([]string, 0, len(snapshot.Namespaces))
	for _, ns := range snapshot.Namespaces {
		names = append(names, ns.Name)
	}
	return names
}

func podNames(snapshot Snapshot) []string {
	names := make([]string, 0, len(snapshot.Pods))
	for _, pod := range snapshot.Pods {
		names = append(names, pod.Name)
	}
	return names
}

func TestSnapshotIncludesCurrentClusterState(t *testing.T) {
	clientset := fake.NewClientset(
		newNamespace("default"),
		newNode("test-node"),
		newPod("running-pod", "pod-uid", v1.PodRunning),
	)
	w := startWatcher(t, clientset)

	snapshot, err := w.Snapshot()
	require.NoError(t, err)

	assert.Equal(t, []string{"running-pod"}, podNames(snapshot))
	require.Len(t, snapshot.Namespaces, 1)
	assert.Equal(t, "default", snapshot.Namespaces[0].Name)
	require.Len(t, snapshot.Nodes, 1)
	assert.Equal(t, "test-node", snapshot.Nodes[0].Name)
}

func TestSnapshotStripsManagedFields(t *testing.T) {
	clientset := fake.NewClientset(newPod("running-pod", "pod-uid", v1.PodRunning))
	w := startWatcher(t, clientset)

	snapshot, err := w.Snapshot()
	require.NoError(t, err)

	require.Len(t, snapshot.Pods, 1)
	assert.Nil(t, snapshot.Pods[0].ManagedFields)
}

func TestSnapshotIncludesPodDeletedBetweenSnapshots(t *testing.T) {
	clientset := fake.NewClientset(newNamespace("default"))
	w := startWatcher(t, clientset)

	// a pod that comes and goes entirely between two snapshots
	_, err := clientset.CoreV1().Pods("default").Create(
		context.Background(), newPod("transient-pod", "pod-uid", v1.PodRunning), metav1.CreateOptions{})
	require.NoError(t, err)
	waitForCachedPod(t, w, "transient-pod", v1.PodRunning)

	require.NoError(t, clientset.CoreV1().Pods("default").Delete(
		context.Background(), "transient-pod", metav1.DeleteOptions{}))
	waitForBuffered(t, w, func() bool { return len(w.deletedPods) == 1 })

	// the snapshot after the deletion still reports the pod...
	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	require.Len(t, snapshot.Pods, 1)
	assert.Equal(t, "transient-pod", snapshot.Pods[0].Name)
	assert.Equal(t, "anchore/test:latest", snapshot.Pods[0].Spec.Containers[0].Image)

	// ...but it is only reported once
	next, err := w.Snapshot()
	require.NoError(t, err)
	assert.Empty(t, next.Pods)
}

func TestSnapshotReportsDeletedPodAsItWasWhileRunning(t *testing.T) {
	clientset := fake.NewClientset(newNamespace("default"))
	w := startWatcher(t, clientset)

	pods := clientset.CoreV1().Pods("default")
	_, err := pods.Create(context.Background(), newPod("transient-pod", "pod-uid", v1.PodRunning), metav1.CreateOptions{})
	require.NoError(t, err)
	waitForBuffered(t, w, func() bool { return len(w.lastRunning) == 1 })

	// the pod completes before it is removed
	_, err = pods.Update(context.Background(), newPod("transient-pod", "pod-uid", v1.PodSucceeded), metav1.UpdateOptions{})
	require.NoError(t, err)
	waitForCachedPod(t, w, "transient-pod", v1.PodSucceeded)

	require.NoError(t, pods.Delete(context.Background(), "transient-pod", metav1.DeleteOptions{}))
	waitForBuffered(t, w, func() bool { return len(w.deletedPods) == 1 })

	// the buffered snapshot is the running one, so ignore-not-running still
	// reports the container
	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	require.Len(t, snapshot.Pods, 1)
	assert.Equal(t, v1.PodRunning, snapshot.Pods[0].Status.Phase)
}

func TestSnapshotBuffersPodThatNeverRan(t *testing.T) {
	clientset := fake.NewClientset(newNamespace("default"))
	w := startWatcher(t, clientset)

	pods := clientset.CoreV1().Pods("default")
	_, err := pods.Create(context.Background(), newPod("pending-pod", "pod-uid", v1.PodPending), metav1.CreateOptions{})
	require.NoError(t, err)
	waitForCachedPod(t, w, "pending-pod", v1.PodPending)

	require.NoError(t, pods.Delete(context.Background(), "pending-pod", metav1.DeleteOptions{}))
	waitForBuffered(t, w, func() bool { return len(w.deletedPods) == 1 })

	// the final state is buffered so that downstream filtering (rather than the
	// watcher) decides whether to report it
	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	require.Len(t, snapshot.Pods, 1)
	assert.Equal(t, v1.PodPending, snapshot.Pods[0].Status.Phase)
	assert.Empty(t, underLock(w, func() map[types.UID]*v1.Pod { return w.lastRunning }))
}

func TestSnapshotBuffersDeletedNamespacesAndNodes(t *testing.T) {
	clientset := fake.NewClientset(
		newNamespace("transient-ns"),
		newNode("transient-node"),
	)
	w := startWatcher(t, clientset)

	require.NoError(t, clientset.CoreV1().Namespaces().Delete(
		context.Background(), "transient-ns", metav1.DeleteOptions{}))
	require.NoError(t, clientset.CoreV1().Nodes().Delete(
		context.Background(), "transient-node", metav1.DeleteOptions{}))
	waitForBuffered(t, w, func() bool {
		return len(w.deletedNamespaces) == 1 && len(w.deletedNodes) == 1
	})

	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	require.Len(t, snapshot.Namespaces, 1)
	assert.Equal(t, "transient-ns", snapshot.Namespaces[0].Name)
	require.Len(t, snapshot.Nodes, 1)
	assert.Equal(t, "transient-node", snapshot.Nodes[0].Name)

	next, err := w.Snapshot()
	require.NoError(t, err)
	assert.Empty(t, next.Namespaces)
	assert.Empty(t, next.Nodes)
}

func TestSnapshotDoesNotDuplicateResourcesStillInTheCache(t *testing.T) {
	clientset := fake.NewClientset(newNamespace("default"))
	w := startWatcher(t, clientset)

	_, err := clientset.CoreV1().Pods("default").Create(
		context.Background(), newPod("recreated-pod", "pod-uid", v1.PodRunning), metav1.CreateOptions{})
	require.NoError(t, err)
	waitForCachedPod(t, w, "recreated-pod", v1.PodRunning)

	// a buffered deletion for a UID that is back in the cache (a delete and
	// recreate the watcher observed out of order) must not be reported twice
	w.onPodDelete(newPod("recreated-pod", "pod-uid", v1.PodRunning))

	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, []string{"recreated-pod"}, podNames(snapshot))
}

func TestDeleteHandlersUnwrapTombstones(t *testing.T) {
	clientset := fake.NewClientset()
	w := startWatcher(t, clientset)

	pod := newPod("tombstoned-pod", "pod-uid", v1.PodRunning)
	namespace := newNamespace("tombstoned-ns")
	node := newNode("tombstoned-node")

	w.onPodDelete(cache.DeletedFinalStateUnknown{Key: "default/tombstoned-pod", Obj: pod})
	w.onNamespaceDelete(cache.DeletedFinalStateUnknown{Key: "tombstoned-ns", Obj: namespace})
	w.onNodeDelete(cache.DeletedFinalStateUnknown{Key: "tombstoned-node", Obj: node})

	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, []string{"tombstoned-pod"}, podNames(snapshot))
	require.Len(t, snapshot.Namespaces, 1)
	assert.Equal(t, "tombstoned-ns", snapshot.Namespaces[0].Name)
	require.Len(t, snapshot.Nodes, 1)
	assert.Equal(t, "tombstoned-node", snapshot.Nodes[0].Name)
}

func TestDeleteHandlersIgnoreUnexpectedObjects(t *testing.T) {
	w, err := New(fake.NewClientset(), Config{CacheSyncTimeout: eventTimeout})
	require.NoError(t, err)

	w.onPodDelete("not-a-pod")
	w.onNamespaceDelete(cache.DeletedFinalStateUnknown{Key: "ns", Obj: "not-a-namespace"})
	w.onNodeDelete(nil)
	w.onPodUpsert("not-a-pod")

	assert.Empty(t, underLock(w, func() map[string]*v1.Pod { return w.deletedPods }))
	assert.Empty(t, underLock(w, func() map[string]*v1.Namespace { return w.deletedNamespaces }))
	assert.Empty(t, underLock(w, func() map[string]*v1.Node { return w.deletedNodes }))
	assert.Empty(t, underLock(w, func() map[types.UID]*v1.Pod { return w.lastRunning }))
}

func TestSnapshotKeepsRunningPodsWhoseDeleteEventIsStillInFlight(t *testing.T) {
	// the pod is absent from the informer cache, standing in for the window
	// between the informer dropping it and delivering the delete event - those
	// happen on different goroutines with a queue in between
	w := startWatcher(t, fake.NewClientset())
	underLock(w, func() any {
		w.lastRunning["pod-uid"] = newPod("transient-pod", "pod-uid", v1.PodRunning)
		return nil
	})

	// a snapshot taken in that window must not forget how the pod looked while
	// it was running
	_, err := w.Snapshot()
	require.NoError(t, err)

	// the delete event finally lands, carrying the pod as it completed
	w.onPodDelete(newPod("transient-pod", "pod-uid", v1.PodSucceeded))

	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	require.Len(t, snapshot.Pods, 1)
	assert.Equal(t, v1.PodRunning, snapshot.Pods[0].Status.Phase,
		"a pod whose delete event arrived after a snapshot should still be reported as it was while running")
}

func TestNodesAreSkippedWhenForbidden(t *testing.T) {
	clientset := fake.NewClientset(newNamespace("default"))
	clientset.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8sErrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "", nil)
	})

	w := startWatcher(t, clientset)
	assert.False(t, w.NodesEnabled())

	snapshot, err := w.Snapshot()
	require.NoError(t, err)
	assert.Empty(t, snapshot.Nodes)
}

func TestStartFailsWhenCachesCannotSync(t *testing.T) {
	clientset := fake.NewClientset()
	// an informer whose LIST is rejected retries forever, so Start has to give
	// up rather than block the agent indefinitely
	clientset.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8sErrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", nil)
	})

	w, err := New(clientset, Config{CacheSyncTimeout: 100 * time.Millisecond})
	require.NoError(t, err)

	stopCh := make(chan struct{})
	defer close(stopCh)

	assert.Error(t, w.Start(stopCh))
}

func TestSnapshotPrefersTheLiveNamespaceOverADeletedOneOfTheSameName(t *testing.T) {
	clientset := fake.NewClientset()
	w := startWatcher(t, clientset)

	// the namespace is deleted and recreated within one reporting interval, so
	// the buffered copy and the live copy share a name but not a UID. Pods
	// reference their namespace by name, so only one of them can be reported.
	deleted := newNamespace("churn-ns")
	deleted.UID = "old-ns-uid"
	w.onNamespaceDelete(deleted)

	recreated := newNamespace("churn-ns")
	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), recreated, metav1.CreateOptions{})
	require.NoError(t, err)
	waitFor(t, func() bool {
		ns, err := w.nsLister.Get("churn-ns")
		return err == nil && ns != nil
	})

	snapshot, err := w.Snapshot()
	require.NoError(t, err)

	assert.Equal(t, []string{"churn-ns"}, namespaceNames(snapshot))
	require.Len(t, snapshot.Namespaces, 1)
	assert.Equal(t, recreated.UID, snapshot.Namespaces[0].UID, "the namespace still in the cluster should win")
}

func TestSnapshotBuffersOneDeletedNamespacePerName(t *testing.T) {
	w := startWatcher(t, fake.NewClientset())

	// deleted, recreated and deleted again, all between two reports
	first := newNamespace("churn-ns")
	first.UID = "first-ns-uid"
	w.onNamespaceDelete(first)

	second := newNamespace("churn-ns")
	second.UID = "second-ns-uid"
	w.onNamespaceDelete(second)

	snapshot, err := w.Snapshot()
	require.NoError(t, err)

	require.Len(t, snapshot.Namespaces, 1)
	assert.Equal(t, second.UID, snapshot.Namespaces[0].UID, "the most recently deleted namespace should win")
}

func TestSnapshotBuffersOneDeletedNodePerName(t *testing.T) {
	w := startWatcher(t, fake.NewClientset())

	first := newNode("churn-node")
	first.UID = "first-node-uid"
	w.onNodeDelete(first)

	second := newNode("churn-node")
	second.UID = "second-node-uid"
	w.onNodeDelete(second)

	snapshot, err := w.Snapshot()
	require.NoError(t, err)

	require.Len(t, snapshot.Nodes, 1)
	assert.Equal(t, second.UID, snapshot.Nodes[0].UID)
}

func TestPodsAreWatchedInOneNamespaceWhenOnlyOneIsIncluded(t *testing.T) {
	included := newPod("included-pod", "included-uid", v1.PodRunning)
	included.Namespace = "watched"
	excluded := newPod("excluded-pod", "excluded-uid", v1.PodRunning)
	excluded.Namespace = "other"

	clientset := fake.NewClientset(included, excluded)
	w, err := New(clientset, Config{CacheSyncTimeout: eventTimeout, Namespaces: []string{"watched"}})
	require.NoError(t, err)

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	require.NoError(t, w.Start(stopCh))

	snapshot, err := w.Snapshot()
	require.NoError(t, err)

	// the agent does not cache pods it would only discard when building the
	// report, which matters on a large cluster
	assert.Equal(t, []string{"included-pod"}, podNames(snapshot))
}

func TestPodsAreWatchedClusterWideWhenSeveralNamespacesAreIncluded(t *testing.T) {
	first := newPod("first-pod", "first-uid", v1.PodRunning)
	first.Namespace = "a"
	second := newPod("second-pod", "second-uid", v1.PodRunning)
	second.Namespace = "b"

	clientset := fake.NewClientset(first, second)
	w, err := New(clientset, Config{CacheSyncTimeout: eventTimeout, Namespaces: []string{"a", "b"}})
	require.NoError(t, err)

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	require.NoError(t, w.Start(stopCh))

	snapshot, err := w.Snapshot()
	require.NoError(t, err)

	names := podNames(snapshot)
	sort.Strings(names)
	assert.Equal(t, []string{"first-pod", "second-pod"}, names)
}
