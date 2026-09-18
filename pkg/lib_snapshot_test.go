package pkg

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/anchore/k8s-inventory/pkg/watcher"
)

var testServerVersion = &version.Info{Major: "1", Minor: "30"}

func snapshotNamespace(name string, uid types.UID, labels map[string]string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			UID:         uid,
			Labels:      labels,
			Annotations: map[string]string{"anchore.io/annotation": "annotation-value"},
		},
	}
}

func snapshotPod(name, namespace string, uid types.UID, phase v1.PodPhase) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			UID:         uid,
			Labels:      map[string]string{"anchore.io/label": "label-value"},
			Annotations: map[string]string{"anchore.io/annotation": "annotation-value"},
		},
		Spec: v1.PodSpec{
			NodeName:   "test-node",
			Containers: []v1.Container{{Name: name, Image: "anchore/test:latest"}},
		},
		Status: v1.PodStatus{
			Phase: phase,
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:        name,
					ContainerID: "containerd://" + string(uid),
					Image:       "anchore/test:latest",
					ImageID:     "anchore/test@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				},
			},
		},
	}
}

func testSnapshot() watcher.Snapshot {
	return watcher.Snapshot{
		Namespaces: []*v1.Namespace{
			snapshotNamespace("ns1", "ns1_UID", map[string]string{"anchore.io/account": "account1"}),
			snapshotNamespace("ns2", "ns2_UID", map[string]string{"anchore.io/account": "account2"}),
			snapshotNamespace("empty-ns", "empty_ns_UID", nil),
		},
		Pods: []*v1.Pod{
			snapshotPod("pod1", "ns1", "pod1_UID", v1.PodRunning),
			snapshotPod("pod2", "ns2", "pod2_UID", v1.PodRunning),
		},
		Nodes: []*v1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "test-node", UID: "node_UID"}},
		},
	}
}

func baseConfig() *config.Application {
	return &config.Application{
		AnchoreDetails:   config.AnchoreInfo{Account: "admin"},
		KubeConfig:       config.KubeConf{Cluster: "test-cluster"},
		IgnoreNotRunning: true,
		MissingTagPolicy: config.MissingTagConf{Policy: "digest"},
	}
}

func namespaceNames(report inventory.Report) []string {
	names := make([]string, 0, len(report.Namespaces))
	for _, ns := range report.Namespaces {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names
}

func podNames(report inventory.Report) []string {
	names := make([]string, 0, len(report.Pods))
	for _, pod := range report.Pods {
		names = append(names, pod.Name)
	}
	sort.Strings(names)
	return names
}

func TestGetInventoryReportsFromSnapshot(t *testing.T) {
	cfg := baseConfig()

	batched := GetInventoryReportsFromSnapshot(cfg, testSnapshot(), testServerVersion)

	require.Len(t, batched, 1)
	require.Len(t, batched["admin"], 1)
	report := batched["admin"][0]

	assert.Equal(t, "test-cluster", report.ClusterName)
	assert.Equal(t, testServerVersion, report.ServerVersionMetadata)
	assert.NotEmpty(t, report.Timestamp)
	assert.Equal(t, []string{"empty-ns", "ns1", "ns2"}, namespaceNames(report))
	assert.Equal(t, []string{"pod1", "pod2"}, podNames(report))
	require.Len(t, report.Nodes, 1)
	assert.Equal(t, "test-node", report.Nodes[0].Name)

	require.Len(t, report.Containers, 2)
	for _, container := range report.Containers {
		assert.Equal(t, "anchore/test:latest", container.ImageTag)
	}

	// pods are linked back to their namespace and node
	for _, pod := range report.Pods {
		assert.Equal(t, "node_UID", pod.NodeUID)
		assert.NotEmpty(t, pod.NamespaceUID)
	}
}

func TestGetInventoryReportsFromSnapshotIncludesDeletedPods(t *testing.T) {
	cfg := baseConfig()

	snapshot := testSnapshot()
	// a pod the watcher buffered because it was deleted between two reports -
	// its namespace was deleted along with it, so it is buffered too
	snapshot.Namespaces = append(snapshot.Namespaces, snapshotNamespace("flash-ns", "flash_ns_UID", nil))
	snapshot.Pods = append(snapshot.Pods, snapshotPod("transient-pod", "flash-ns", "transient_UID", v1.PodRunning))

	batched := GetInventoryReportsFromSnapshot(cfg, snapshot, testServerVersion)
	report := batched["admin"][0]

	assert.Contains(t, namespaceNames(report), "flash-ns")
	assert.Contains(t, podNames(report), "transient-pod")

	containerIDs := make([]string, 0, len(report.Containers))
	for _, container := range report.Containers {
		containerIDs = append(containerIDs, container.ID)
	}
	assert.Contains(t, containerIDs, "containerd://transient_UID")
}

func TestGetInventoryReportsFromSnapshotIgnoreNotRunning(t *testing.T) {
	snapshot := testSnapshot()
	snapshot.Pods = append(snapshot.Pods, snapshotPod("pending-pod", "ns1", "pending_UID", v1.PodPending))

	tests := []struct {
		name             string
		ignoreNotRunning bool
		wantContainers   int
	}{
		{name: "not running pods are dropped", ignoreNotRunning: true, wantContainers: 2},
		{name: "not running pods are reported", ignoreNotRunning: false, wantContainers: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.IgnoreNotRunning = tt.ignoreNotRunning

			report := GetInventoryReportsFromSnapshot(cfg, snapshot, testServerVersion)["admin"][0]

			assert.Len(t, report.Containers, tt.wantContainers)
			// the pod itself is always reported, only its containers are filtered
			assert.Contains(t, podNames(report), "pending-pod")
		})
	}
}

func TestGetInventoryReportsFromSnapshotNamespaceSelectors(t *testing.T) {
	tests := []struct {
		name      string
		selectors config.NamespaceSelector
		want      []string
	}{
		{
			name: "namespaces can be excluded",
			selectors: config.NamespaceSelector{
				Exclude: []string{"^empty-.*"},
			},
			want: []string{"ns1", "ns2"},
		},
		{
			name: "includes act as an allow-list",
			selectors: config.NamespaceSelector{
				Include: []string{"ns1"},
			},
			want: []string{"ns1"},
		},
		{
			name: "empty namespaces can be ignored",
			selectors: config.NamespaceSelector{
				IgnoreEmpty: true,
			},
			want: []string{"ns1", "ns2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.NamespaceSelectors = tt.selectors

			report := GetInventoryReportsFromSnapshot(cfg, testSnapshot(), testServerVersion)["admin"][0]

			assert.Equal(t, tt.want, namespaceNames(report))
		})
	}
}

func TestGetInventoryReportsFromSnapshotAccountRouting(t *testing.T) {
	tests := []struct {
		name         string
		configure    func(*config.Application)
		wantAccounts map[string][]string
	}{
		{
			name: "routing by namespace label",
			configure: func(cfg *config.Application) {
				cfg.AccountRouteByNamespaceLabel = config.AccountRouteByNamespaceLabel{
					LabelKey: "anchore.io/account",
				}
			},
			wantAccounts: map[string][]string{
				"account1": {"ns1"},
				"account2": {"ns2"},
				"admin":    {"empty-ns"},
			},
		},
		{
			name: "routing by explicit account routes",
			configure: func(cfg *config.Application) {
				cfg.AccountRoutes = config.AccountRoutes{
					"routed-account": config.AccountRouteDetails{Namespaces: []string{"^ns.*"}},
				}
			},
			wantAccounts: map[string][]string{
				"routed-account": {"ns1", "ns2"},
				"admin":          {"empty-ns"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			tt.configure(cfg)

			batched := GetInventoryReportsFromSnapshot(cfg, testSnapshot(), testServerVersion)

			require.Len(t, batched, len(tt.wantAccounts))
			for account, namespaces := range tt.wantAccounts {
				require.Len(t, batched[account], 1, "expected a single report for account %s", account)
				assert.Equal(t, namespaces, namespaceNames(batched[account][0]))
				// every report carries the full node list, as with polling
				assert.Len(t, batched[account][0].Nodes, 1)
			}
		})
	}
}

func TestGetInventoryReportsFromSnapshotBatches(t *testing.T) {
	cfg := baseConfig()
	cfg.InventoryReportLimits = config.InventoryReportLimits{Namespaces: 1}

	batched := GetInventoryReportsFromSnapshot(cfg, testSnapshot(), testServerVersion)

	require.Len(t, batched["admin"], 3)
	for _, report := range batched["admin"] {
		assert.Len(t, report.Namespaces, 1)
	}
}

func TestGetInventoryReportsFromSnapshotMetadataCollection(t *testing.T) {
	cfg := baseConfig()
	cfg.MetadataCollection = config.MetadataCollection{
		Namespace: config.ResourceMetadata{Disable: true},
		Pods:      config.ResourceMetadata{Disable: true},
		Nodes:     config.ResourceMetadata{Disable: true},
	}

	report := GetInventoryReportsFromSnapshot(cfg, testSnapshot(), testServerVersion)["admin"][0]

	for _, ns := range report.Namespaces {
		assert.Empty(t, ns.Labels)
		assert.Empty(t, ns.Annotations)
	}
	for _, pod := range report.Pods {
		assert.Empty(t, pod.Labels)
		assert.Empty(t, pod.Annotations)
	}
	for _, node := range report.Nodes {
		assert.Empty(t, node.Labels)
		assert.Empty(t, node.Annotations)
	}
}

func TestGetInventoryReportsFromSnapshotEmptyCluster(t *testing.T) {
	cfg := baseConfig()

	batched := GetInventoryReportsFromSnapshot(cfg, watcher.Snapshot{}, testServerVersion)

	require.Len(t, batched["admin"], 1)
	report := batched["admin"][0]
	assert.Empty(t, report.Namespaces)
	assert.Empty(t, report.Pods)
	assert.Empty(t, report.Containers)
	assert.Empty(t, report.Nodes)
}

func TestGetInventoryReportsFromSnapshotCollapsesDuplicateNamespaceNames(t *testing.T) {
	cfg := baseConfig()

	// a namespace deleted and recreated within one interval appears twice in a
	// snapshot, once from the deletion buffer and once from the informer cache.
	// Pods reference their namespace by name, so reporting both would emit
	// every pod and container in it twice.
	snapshot := watcher.Snapshot{
		Namespaces: []*v1.Namespace{
			snapshotNamespace("ns1", "ns1_UID", nil),
			snapshotNamespace("ns1", "ns1_UID_v2", nil),
		},
		Pods: []*v1.Pod{
			snapshotPod("pod1", "ns1", "pod1_UID", v1.PodRunning),
			snapshotPod("pod2", "ns1", "pod2_UID", v1.PodRunning),
		},
	}

	report := GetInventoryReportsFromSnapshot(cfg, snapshot, testServerVersion)["admin"][0]

	require.Len(t, report.Namespaces, 1)
	assert.Equal(t, "ns1_UID", report.Namespaces[0].UID, "the first namespace, which the watcher orders as the live one, should win")
	assert.Equal(t, []string{"pod1", "pod2"}, podNames(report))
	assert.Len(t, report.Containers, 2)

	// every pod is attributed to the single reported namespace
	for _, pod := range report.Pods {
		assert.Equal(t, "ns1_UID", pod.NamespaceUID)
	}
}

func TestGetInventoryReportsFromSnapshotRefreshesNothingWithoutAServerVersion(t *testing.T) {
	// the collector keeps the last known version when a lookup fails, which can
	// be nil if the very first lookup failed
	report := GetInventoryReportsFromSnapshot(baseConfig(), testSnapshot(), nil)["admin"][0]

	assert.Nil(t, report.ServerVersionMetadata)
	assert.NotEmpty(t, report.Namespaces)
}

func TestEventStreamIsNotUsedWhenPollingIsConfigured(t *testing.T) {
	cfg := baseConfig()
	cfg.InventoryCollection.Method = config.InventoryCollectionMethodPoll

	collector := &inventoryCollector{}

	// the revert switch keeps the agent on the legacy collection path, without
	// so much as trying to start the informers
	assert.Nil(t, collector.eventStream(cfg))
	assert.Nil(t, collector.stream)
	assert.False(t, collector.starting)
}

func TestNextRetryDelay(t *testing.T) {
	tests := []struct {
		name    string
		current time.Duration
		want    time.Duration
	}{
		{name: "the first failure waits the initial delay", current: 0, want: eventStreamInitialRetryDelay},
		{name: "each consecutive failure waits longer", current: time.Minute, want: 2 * time.Minute},
		{name: "the delay is capped", current: 45 * time.Minute, want: eventStreamMaxRetryDelay},
		{name: "the cap is not exceeded once reached", current: eventStreamMaxRetryDelay, want: eventStreamMaxRetryDelay},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nextRetryDelay(tt.current))
		})
	}
}
