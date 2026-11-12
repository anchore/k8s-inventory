package inventory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/anchore/k8s-inventory/pkg/client"
)

var crCreated = metav1.NewTime(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))

// testControllerRevision mirrors what the controllers create: named <owner>-<hash>. StatefulSet revisions carry the
// controller.kubernetes.io/hash label, DaemonSet revisions the controller-revision-hash label.
func testControllerRevision(ownerKind WorkloadKind, ownerName, ownerUID, hash string, revision int64, labels map[string]string) appsv1.ControllerRevision {
	return appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              ownerName + "-" + hash,
			Namespace:         "default",
			Labels:            labels,
			CreationTimestamp: crCreated,
			OwnerReferences:   []metav1.OwnerReference{{Kind: string(ownerKind), Name: ownerName, UID: types.UID(ownerUID)}},
		},
		Revision: revision,
	}
}

func testOwnedPod(uid string, ownerKind WorkloadKind, ownerUID, revisionLabel string, phase v1.PodPhase) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            uid,
			Namespace:       "default",
			UID:             types.UID(uid),
			Labels:          map[string]string{ControllerRevisionHashLabel: revisionLabel},
			OwnerReferences: []metav1.OwnerReference{{Kind: string(ownerKind), UID: types.UID(ownerUID)}},
		},
		Status: v1.PodStatus{Phase: phase},
	}
}

func testStatefulSet(updateRevision string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "db",
			Namespace:   "default",
			UID:         "sts-uid",
			Annotations: map[string]string{HelmReleaseAnnotation: "shop"},
			Labels:      map[string]string{AppComponentLabel: "database"},
		},
		Status: appsv1.StatefulSetStatus{UpdateRevision: updateRevision},
	}
}

func TestProcessStatefulSets(t *testing.T) {
	oldCR := testControllerRevision(KindStatefulSet, "db", "sts-uid", "aaa", 1, map[string]string{ControllerRevisionHashLabelV2: "aaa"})
	newCR := testControllerRevision(KindStatefulSet, "db", "sts-uid", "bbb", 2, map[string]string{ControllerRevisionHashLabelV2: "bbb"})
	otherCR := testControllerRevision(KindStatefulSet, "other", "other-uid", "ccc", 7, nil)
	// StatefulSet pods are labelled with the ControllerRevision name
	pods := []v1.Pod{
		testOwnedPod("db-0", KindStatefulSet, "sts-uid", "db-bbb", v1.PodRunning),
		testOwnedPod("db-1", KindStatefulSet, "sts-uid", "db-aaa", v1.PodRunning),
		testOwnedPod("other-0", KindStatefulSet, "other-uid", "other-ccc", v1.PodRunning),
	}
	containers := []Container{
		{PodUID: "db-0", Name: "redis", ImageTag: "redis:7.4", ImageDigest: "sha256:new"},
		{PodUID: "db-1", Name: "redis", ImageTag: "redis:7.2", ImageDigest: "sha256:old"},
		{PodUID: "other-0", Name: "redis", ImageTag: "redis:7.0", ImageDigest: "sha256:other"},
	}
	want := Workload{
		Kind:            KindStatefulSet,
		Name:            "db",
		Namespace:       "default",
		NamespaceUID:    "ns-uid",
		UID:             "sts-uid",
		Revision:        "2",
		RevisionName:    "db-bbb",
		RevisionHash:    "bbb",
		RevisionCreated: crCreated.UTC(),
		Relations:       Relations{HelmRelease: "shop", Component: "database"},
		Containers:      []Container{{PodUID: "db-0", Name: "redis", ImageTag: "redis:7.4", ImageDigest: "sha256:new"}},
	}

	t.Run("uses the update revision mid-rollout (partitioned) and ignores pods of other revisions", func(t *testing.T) {
		got := ProcessStatefulSets([]appsv1.StatefulSet{testStatefulSet("db-bbb")}, []appsv1.ControllerRevision{oldCR, newCR, otherCR}, pods, containers, "ns-uid")
		assert.Equal(t, []Workload{want}, got)
		assert.Equal(t, "ControllerRevision", got[0].RevisionKind())
	})

	t.Run("falls back to the newest revision before the controller reports status", func(t *testing.T) {
		got := ProcessStatefulSets([]appsv1.StatefulSet{testStatefulSet("")}, []appsv1.ControllerRevision{oldCR, newCR}, pods, containers, "ns-uid")
		assert.Equal(t, []Workload{want}, got)
	})

	t.Run("rollback reuses the old revision with a higher number", func(t *testing.T) {
		rolledBack := oldCR
		rolledBack.Revision = 3
		got := ProcessStatefulSets([]appsv1.StatefulSet{testStatefulSet("db-aaa")}, []appsv1.ControllerRevision{rolledBack, newCR}, pods, containers, "ns-uid")
		require.Len(t, got, 1)
		assert.Equal(t, "3", got[0].Revision)
		assert.Equal(t, "aaa", got[0].RevisionHash)
		assert.Equal(t, "sha256:old", got[0].Containers[0].ImageDigest)
	})

	t.Run("hash falls back to the name suffix on older clusters", func(t *testing.T) {
		unlabelled := testControllerRevision(KindStatefulSet, "db", "sts-uid", "bbb", 2, nil)
		got := ProcessStatefulSets([]appsv1.StatefulSet{testStatefulSet("db-bbb")}, []appsv1.ControllerRevision{unlabelled}, pods, containers, "ns-uid")
		require.Len(t, got, 1)
		assert.Equal(t, "bbb", got[0].RevisionHash)
	})

	t.Run("skips statefulsets without revisions or running pods", func(t *testing.T) {
		assert.Empty(t, ProcessStatefulSets([]appsv1.StatefulSet{testStatefulSet("db-bbb")}, nil, pods, containers, "ns-uid"))
		pending := []v1.Pod{testOwnedPod("db-0", KindStatefulSet, "sts-uid", "db-bbb", v1.PodPending)}
		assert.Empty(t, ProcessStatefulSets([]appsv1.StatefulSet{testStatefulSet("db-bbb")}, []appsv1.ControllerRevision{newCR}, pending, containers, "ns-uid"))
	})
}

func TestProcessDaemonSets(t *testing.T) {
	ds := appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "log-agent", Namespace: "default", UID: "ds-uid"}}
	// DaemonSet revisions and pods are labelled with the hash only
	oldCR := testControllerRevision(KindDaemonSet, "log-agent", "ds-uid", "aaa", 1, map[string]string{ControllerRevisionHashLabel: "aaa"})
	newCR := testControllerRevision(KindDaemonSet, "log-agent", "ds-uid", "bbb", 2, map[string]string{ControllerRevisionHashLabel: "bbb"})
	pods := []v1.Pod{
		testOwnedPod("node-a", KindDaemonSet, "ds-uid", "bbb", v1.PodRunning),
		testOwnedPod("node-b", KindDaemonSet, "ds-uid", "aaa", v1.PodRunning),
		// a pod with the same hash label owned by a StatefulSet must not match
		testOwnedPod("sts-0", KindStatefulSet, "ds-uid", "bbb", v1.PodRunning),
	}
	containers := []Container{
		{PodUID: "node-a", Name: "agent", ImageTag: "busybox:1.37", ImageDigest: "sha256:new"},
		{PodUID: "node-b", Name: "agent", ImageTag: "busybox:1.36", ImageDigest: "sha256:old"},
		{PodUID: "sts-0", Name: "other", ImageTag: "busybox:1.35", ImageDigest: "sha256:x"},
	}

	got := ProcessDaemonSets([]appsv1.DaemonSet{ds}, []appsv1.ControllerRevision{newCR, oldCR}, pods, containers, "ns-uid")
	assert.Equal(t, []Workload{{
		Kind:            KindDaemonSet,
		Name:            "log-agent",
		Namespace:       "default",
		NamespaceUID:    "ns-uid",
		UID:             "ds-uid",
		Revision:        "2",
		RevisionName:    "log-agent-bbb",
		RevisionHash:    "bbb",
		RevisionCreated: crCreated.UTC(),
		Containers:      []Container{{PodUID: "node-a", Name: "agent", ImageTag: "busybox:1.37", ImageDigest: "sha256:new"}},
	}}, got)

	assert.Empty(t, ProcessDaemonSets([]appsv1.DaemonSet{ds}, nil, pods, containers, "ns-uid"))
}

func TestFetchStatefulSetsDaemonSetsAndControllerRevisions(t *testing.T) {
	sts := testStatefulSet("db-bbb")
	ds := appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "log-agent", Namespace: "default"}}
	cr := testControllerRevision(KindStatefulSet, "db", "sts-uid", "bbb", 2, nil)
	otherNS := testStatefulSet("x")
	otherNS.Name, otherNS.Namespace = "elsewhere", "other-ns"

	c := client.Client{Clientset: fake.NewClientset(&sts, &ds, &cr, &otherNS)}

	stss, err := FetchStatefulSetsInNamespace(c, 100, 10, "default")
	require.NoError(t, err)
	require.Len(t, stss, 1)
	assert.Equal(t, "db", stss[0].Name)

	dss, err := FetchDaemonSetsInNamespace(c, 100, 10, "default")
	require.NoError(t, err)
	require.Len(t, dss, 1)

	crs, err := FetchControllerRevisionsInNamespace(c, 100, 10, "default")
	require.NoError(t, err)
	require.Len(t, crs, 1)
	assert.Equal(t, "db-bbb", crs[0].Name)
}
