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

var rsCreated = metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

func testDeployment(revision string) appsv1.Deployment {
	annotations := map[string]string{}
	if revision != "" {
		annotations[DeploymentRevisionAnnotation] = revision
	}
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "web",
			Namespace:   "default",
			UID:         "dep-uid",
			Annotations: annotations,
		},
	}
}

func testReplicaSet(name, uid, revision, hash string) appsv1.ReplicaSet {
	return appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			UID:               types.UID(uid),
			Annotations:       map[string]string{DeploymentRevisionAnnotation: revision},
			Labels:            map[string]string{PodTemplateHashLabel: hash},
			CreationTimestamp: rsCreated,
			OwnerReferences:   []metav1.OwnerReference{{Kind: "Deployment", UID: "dep-uid"}},
		},
	}
}

func testPod(uid, rsUID string, phase v1.PodPhase) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            uid,
			Namespace:       "default",
			UID:             types.UID(uid),
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", UID: types.UID(rsUID)}},
		},
		Status: v1.PodStatus{Phase: phase},
	}
}

func TestProcessDeployments(t *testing.T) {
	oldRS := testReplicaSet("web-old", "rs-old", "1", "aaa")
	newRS := testReplicaSet("web-new", "rs-new", "2", "bbb")

	tests := []struct {
		name       string
		deps       []appsv1.Deployment
		rss        []appsv1.ReplicaSet
		pods       []v1.Pod
		containers []Container
		want       []Deployment
	}{
		{
			name: "selects current replicaset mid-rollout and ignores old replicaset pods",
			deps: []appsv1.Deployment{testDeployment("2")},
			rss:  []appsv1.ReplicaSet{oldRS, newRS},
			pods: []v1.Pod{
				testPod("pod-old", "rs-old", v1.PodRunning),
				testPod("pod-new", "rs-new", v1.PodRunning),
			},
			containers: []Container{
				{PodUID: "pod-old", Name: "nginx", ImageTag: "nginx:1.25", ImageDigest: "sha256:old"},
				{PodUID: "pod-new", Name: "nginx", ImageTag: "nginx:1.27", ImageDigest: "sha256:new"},
			},
			want: []Deployment{{
				Name:              "web",
				Namespace:         "default",
				NamespaceUID:      "ns-uid",
				UID:               "dep-uid",
				Revision:          "2",
				ReplicaSetName:    "web-new",
				ReplicaSetUID:     "rs-new",
				PodTemplateHash:   "bbb",
				ReplicaSetCreated: rsCreated.UTC(),
				Containers: []Container{
					{PodUID: "pod-new", Name: "nginx", ImageTag: "nginx:1.27", ImageDigest: "sha256:new"},
				},
			}},
		},
		{
			name: "ignores non-running pods and prefers containers with a digest",
			deps: []appsv1.Deployment{testDeployment("2")},
			rss:  []appsv1.ReplicaSet{newRS},
			pods: []v1.Pod{
				testPod("pod-a", "rs-new", v1.PodRunning),
				testPod("pod-b", "rs-new", v1.PodRunning),
				testPod("pod-c", "rs-new", v1.PodPending),
			},
			containers: []Container{
				{PodUID: "pod-c", Name: "pending", ImageTag: "busybox:1"},
				{PodUID: "pod-a", Name: "nginx", ImageTag: "nginx:1.27"},
				{PodUID: "pod-b", Name: "nginx", ImageTag: "nginx:1.27", ImageDigest: "sha256:new"},
				{PodUID: "pod-a", Name: "sidecar", ImageTag: "busybox:1.36", ImageDigest: "sha256:bb"},
				{PodUID: "pod-b", Name: "sidecar", ImageTag: "busybox:1.36", ImageDigest: "sha256:bb"},
			},
			want: []Deployment{{
				Name:              "web",
				Namespace:         "default",
				NamespaceUID:      "ns-uid",
				UID:               "dep-uid",
				Revision:          "2",
				ReplicaSetName:    "web-new",
				ReplicaSetUID:     "rs-new",
				PodTemplateHash:   "bbb",
				ReplicaSetCreated: rsCreated.UTC(),
				Containers: []Container{
					{PodUID: "pod-b", Name: "nginx", ImageTag: "nginx:1.27", ImageDigest: "sha256:new"},
					{PodUID: "pod-a", Name: "sidecar", ImageTag: "busybox:1.36", ImageDigest: "sha256:bb"},
				},
			}},
		},
		{
			name:       "skips deployment without revision annotation",
			deps:       []appsv1.Deployment{testDeployment("")},
			rss:        []appsv1.ReplicaSet{newRS},
			pods:       []v1.Pod{testPod("pod-new", "rs-new", v1.PodRunning)},
			containers: []Container{{PodUID: "pod-new", Name: "nginx", ImageTag: "nginx:1.27"}},
		},
		{
			name: "skips deployment without a replicaset for the current revision",
			deps: []appsv1.Deployment{testDeployment("3")},
			rss:  []appsv1.ReplicaSet{oldRS, newRS},
		},
		{
			name: "skips deployment with no running pods",
			deps: []appsv1.Deployment{testDeployment("2")},
			rss:  []appsv1.ReplicaSet{newRS},
			pods: []v1.Pod{testPod("pod-new", "rs-new", v1.PodPending)},
			containers: []Container{
				{PodUID: "pod-new", Name: "nginx", ImageTag: "nginx:1.27"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProcessDeployments(tt.deps, tt.rss, tt.pods, tt.containers, "ns-uid")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchDeploymentsAndReplicaSetsInNamespace(t *testing.T) {
	dep := testDeployment("1")
	rs := testReplicaSet("web-1", "rs-1", "1", "aaa")
	other := testDeployment("1")
	other.Name = "other"
	other.Namespace = "other-ns"

	c := client.Client{Clientset: fake.NewClientset(&dep, &rs, &other)}

	deps, err := FetchDeploymentsInNamespace(c, 100, 10, "default")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "web", deps[0].Name)

	rss, err := FetchReplicaSetsInNamespace(c, 100, 10, "default")
	require.NoError(t, err)
	require.Len(t, rss, 1)
	assert.Equal(t, "web-1", rss[0].Name)
}
