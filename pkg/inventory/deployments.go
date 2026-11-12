package inventory

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/client"
)

const (
	DeploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
	PodTemplateHashLabel         = "pod-template-hash"
)

// listAll pages through a Kubernetes list call, returning every item
func listAll[T any](batchSize, timeout int64, what, namespace string,
	list func(opts metav1.ListOptions) ([]T, metav1.ListInterface, error),
) ([]T, error) {
	var items []T
	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          batchSize,
			Continue:       cont,
			TimeoutSeconds: &timeout,
		}
		page, meta, err := list(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list %s in namespace %s: %w", what, namespace, err)
		}
		items = append(items, page...)
		cont = meta.GetContinue()
		if cont == "" {
			return items, nil
		}
	}
}

func FetchDeploymentsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]appsv1.Deployment, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching deployments in namespace")
	return listAll(batchSize, timeout, "deployments", namespace, func(opts metav1.ListOptions) ([]appsv1.Deployment, metav1.ListInterface, error) {
		l, err := c.Clientset.AppsV1().Deployments(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, nil, err
		}
		return l.Items, l, nil
	})
}

func FetchReplicaSetsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]appsv1.ReplicaSet, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching replicasets in namespace")
	return listAll(batchSize, timeout, "replicasets", namespace, func(opts metav1.ListOptions) ([]appsv1.ReplicaSet, metav1.ListInterface, error) {
		l, err := c.Clientset.AppsV1().ReplicaSets(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, nil, err
		}
		return l.Items, l, nil
	})
}

// currentReplicaSet returns the ReplicaSet owned by the deployment whose revision matches the deployment's revision
func currentReplicaSet(dep appsv1.Deployment, revision string, rss []appsv1.ReplicaSet) *appsv1.ReplicaSet {
	for i := range rss {
		rs := &rss[i]
		if isOwnedBy(rs.OwnerReferences, "Deployment", string(dep.UID)) &&
			rs.Annotations[DeploymentRevisionAnnotation] == revision {
			return rs
		}
	}
	return nil
}

// ProcessDeployments determines the current rollout of each deployment and the containers running for it.
// Deployments without a current ReplicaSet or without running containers are omitted.
func ProcessDeployments(deps []appsv1.Deployment, rss []appsv1.ReplicaSet, pods []v1.Pod, containers []Container, namespaceUID string) []Workload {
	var result []Workload
	for _, dep := range deps {
		revision := dep.Annotations[DeploymentRevisionAnnotation]
		if revision == "" {
			log.Debugf("Skipping deployment %s/%s: no revision annotation", dep.Namespace, dep.Name)
			continue
		}
		rs := currentReplicaSet(dep, revision, rss)
		if rs == nil {
			log.Debugf("Skipping deployment %s/%s: no replicaset found for revision %s", dep.Namespace, dep.Name, revision)
			continue
		}
		rsUID := string(rs.UID)
		depContainers := containersForPods(pods, containers, func(p v1.Pod) bool {
			return isOwnedBy(p.OwnerReferences, "ReplicaSet", rsUID)
		})
		if len(depContainers) == 0 {
			log.Debugf("Skipping deployment %s/%s: no running containers for replicaset %s", dep.Namespace, dep.Name, rs.Name)
			continue
		}
		result = append(result, Workload{
			Kind:            KindDeployment,
			Name:            dep.Name,
			Namespace:       dep.Namespace,
			NamespaceUID:    namespaceUID,
			UID:             string(dep.UID),
			Revision:        revision,
			RevisionName:    rs.Name,
			RevisionHash:    rs.Labels[PodTemplateHashLabel],
			RevisionCreated: rs.CreationTimestamp.UTC(),
			Relations:       relationsFor(dep.ObjectMeta),
			Containers:      depContainers,
		})
	}
	return result
}
