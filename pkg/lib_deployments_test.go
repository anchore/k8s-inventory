package pkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/anchore/k8s-inventory/internal/config"
	"github.com/anchore/k8s-inventory/pkg/inventory"
)

func TestGetDeploymentsInNamespace(t *testing.T) {
	cfg := &config.Application{Kubernetes: config.KubernetesAPI{RequestBatchSize: 100, RequestTimeoutSeconds: 10}}
	ns := inventory.Namespace{Name: "default", UID: "ns-uid"}
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
	pods := []v1.Pod{{
		ObjectMeta: metav1.ObjectMeta{Name: "web-abc-1", Namespace: "default", UID: "pod-uid",
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", UID: "rs-uid"}}},
		Status: v1.PodStatus{Phase: v1.PodRunning},
	}}
	containers := []inventory.Container{{PodUID: "pod-uid", Name: "nginx", ImageTag: "nginx:1.27"}}

	t.Run("returns the current rollout of each deployment", func(t *testing.T) {
		got := getDeploymentsInNamespace(fake.NewClientset(dep, rs), cfg, ns, pods, containers)
		require.Len(t, got, 1)
		assert.Equal(t, "web", got[0].Name)
		assert.Equal(t, "abc", got[0].PodTemplateHash)
		assert.Equal(t, containers, got[0].Containers)
	})

	t.Run("forbidden list returns no deployments", func(t *testing.T) {
		clientset := fake.NewClientset(dep, rs)
		clientset.PrependReactor("list", "replicasets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "replicasets"}, "", nil)
		})
		assert.Nil(t, getDeploymentsInNamespace(clientset, cfg, ns, pods, containers))
	})
}
