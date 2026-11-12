package pkg

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"
)

// workloadFixtures returns a Deployment (web), a StatefulSet (web, same name) and a DaemonSet (agent) with one
// running pod each
func workloadFixtures() ([]runtime.Object, []v1.Pod, []inventory.Container) {
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: "web", Namespace: "default", UID: "dep-uid",
		Annotations: map[string]string{inventory.DeploymentRevisionAnnotation: "1"},
	}}
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Name: "web-abc", Namespace: "default", UID: "rs-uid",
		Annotations:     map[string]string{inventory.DeploymentRevisionAnnotation: "1"},
		Labels:          map[string]string{inventory.PodTemplateHashLabel: "abc"},
		OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", UID: "dep-uid"}},
	}}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "sts-uid"},
		Status:     appsv1.StatefulSetStatus{UpdateRevision: "web-def"},
	}
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default", UID: "ds-uid"}}
	stsCR := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{
		Name: "web-def", Namespace: "default",
		OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", Name: "web", UID: "sts-uid"}},
	}, Revision: 1}
	dsCR := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{
		Name: "agent-ghi", Namespace: "default", Labels: map[string]string{inventory.ControllerRevisionHashLabel: "ghi"},
		OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: "agent", UID: "ds-uid"}},
	}, Revision: 1}

	pod := func(uid, ownerKind, ownerUID string, labels map[string]string) v1.Pod {
		return v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: uid, Namespace: "default", UID: types.UID(uid), Labels: labels,
				OwnerReferences: []metav1.OwnerReference{{Kind: ownerKind, UID: types.UID(ownerUID)}}},
			Status: v1.PodStatus{Phase: v1.PodRunning},
		}
	}
	pods := []v1.Pod{
		pod("web-abc-1", "ReplicaSet", "rs-uid", nil),
		pod("web-0", "StatefulSet", "sts-uid", map[string]string{inventory.ControllerRevisionHashLabel: "web-def"}),
		pod("agent-x", "DaemonSet", "ds-uid", map[string]string{inventory.ControllerRevisionHashLabel: "ghi"}),
	}
	containers := []inventory.Container{
		{PodUID: "web-abc-1", Name: "nginx", ImageTag: "nginx:1.27"},
		{PodUID: "web-0", Name: "redis", ImageTag: "redis:7.4"},
		{PodUID: "agent-x", Name: "agent", ImageTag: "busybox:1.37"},
	}
	return []runtime.Object{dep, rs, sts, ds, stsCR, dsCR}, pods, containers
}

func kindsOf(workloads []inventory.Workload) []inventory.WorkloadKind {
	var kinds []inventory.WorkloadKind
	for _, w := range workloads {
		kinds = append(kinds, w.Kind)
	}
	return kinds
}

func TestGetWorkloadsInNamespace(t *testing.T) {
	ns := inventory.Namespace{Name: "default", UID: "ns-uid"}
	objects, pods, containers := workloadFixtures()
	cfgFor := func(kinds ...string) *config.Application {
		return &config.Application{
			Kubernetes:             config.KubernetesAPI{RequestBatchSize: 100, RequestTimeoutSeconds: 10},
			CreateApplicationsFrom: kinds,
		}
	}
	all := cfgFor(config.ApplicationsFromDeployments, config.ApplicationsFromStatefulSets, config.ApplicationsFromDaemonSets)

	t.Run("returns the current rollout of each enabled kind", func(t *testing.T) {
		got := getWorkloadsInNamespace(fake.NewClientset(objects...), all, ns, pods, containers)
		require.Len(t, got, 3)
		assert.Equal(t, []inventory.WorkloadKind{inventory.KindDeployment, inventory.KindStatefulSet, inventory.KindDaemonSet}, kindsOf(got))
		assert.Equal(t, "abc", got[0].RevisionHash)
		assert.Equal(t, "web-def", got[1].RevisionName)
		assert.Equal(t, "ghi", got[2].RevisionHash)
		assert.Equal(t, "busybox:1.37", got[2].Containers[0].ImageTag)
	})

	t.Run("only enabled kinds are collected", func(t *testing.T) {
		clientset := fake.NewClientset(objects...)
		got := getWorkloadsInNamespace(clientset, cfgFor(config.ApplicationsFromStatefulSets), ns, pods, containers)
		assert.Equal(t, []inventory.WorkloadKind{inventory.KindStatefulSet}, kindsOf(got))
		for _, a := range clientset.Actions() {
			assert.NotEqual(t, "deployments", a.GetResource().Resource)
			assert.NotEqual(t, "daemonsets", a.GetResource().Resource)
		}
	})

	t.Run("controllerrevisions are listed once for statefulsets and daemonsets", func(t *testing.T) {
		clientset := fake.NewClientset(objects...)
		getWorkloadsInNamespace(clientset, all, ns, pods, containers)
		lists := 0
		for _, a := range clientset.Actions() {
			if a.GetVerb() == "list" && a.GetResource().Resource == "controllerrevisions" {
				lists++
			}
		}
		assert.Equal(t, 1, lists)
	})

	t.Run("forbidden kind is skipped, warned once, and other kinds still collected", func(t *testing.T) {
		resetWorkloadsForbiddenWarned()
		clientset := fake.NewClientset(objects...)
		clientset.PrependReactor("list", "statefulsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "", nil)
		})
		got := getWorkloadsInNamespace(clientset, all, ns, pods, containers)
		assert.Equal(t, []inventory.WorkloadKind{inventory.KindDeployment, inventory.KindDaemonSet}, kindsOf(got))
		assert.True(t, workloadsForbiddenWarned[config.ApplicationsFromStatefulSets].Load())
		assert.False(t, workloadsForbiddenWarned[config.ApplicationsFromDeployments].Load())
	})

	t.Run("forbidden controllerrevisions skip statefulsets and daemonsets only", func(t *testing.T) {
		resetWorkloadsForbiddenWarned()
		clientset := fake.NewClientset(objects...)
		clientset.PrependReactor("list", "controllerrevisions", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "controllerrevisions"}, "", nil)
		})
		got := getWorkloadsInNamespace(clientset, all, ns, pods, containers)
		assert.Equal(t, []inventory.WorkloadKind{inventory.KindDeployment}, kindsOf(got))
		assert.True(t, workloadsForbiddenWarned[config.ApplicationsFromStatefulSets].Load())
		assert.True(t, workloadsForbiddenWarned[config.ApplicationsFromDaemonSets].Load())
	})

	t.Run("other list errors do not consume the forbidden warning", func(t *testing.T) {
		resetWorkloadsForbiddenWarned()
		clientset := fake.NewClientset(objects...)
		clientset.PrependReactor("list", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewInternalError(errors.New("boom"))
		})
		got := getWorkloadsInNamespace(clientset, cfgFor(config.ApplicationsFromDeployments), ns, pods, containers)
		assert.Empty(t, got)
		assert.False(t, workloadsForbiddenWarned[config.ApplicationsFromDeployments].Load())
	})
}

func TestApplicationSyncsAdd(t *testing.T) {
	a := inventory.Workload{Kind: inventory.KindDeployment, Name: "a"}
	b := inventory.Workload{Kind: inventory.KindStatefulSet, Name: "b"}

	disabled := ApplicationSyncs{}
	disabled.Add(&config.Application{}, "admin", []inventory.Workload{a})
	assert.Empty(t, disabled)

	cfg := &config.Application{CreateApplicationsFrom: []string{config.ApplicationsFromDeployments}}
	syncs := ApplicationSyncs{}
	syncs.Add(cfg, "admin", []inventory.Workload{a})
	// workloads from an account that fell back to the default account are merged into it
	syncs.Add(cfg, "admin", []inventory.Workload{b})
	syncs.Add(cfg, "other", nil)
	assert.Equal(t, ApplicationSyncs{"admin": {a, b}}, syncs)
}
