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

// Deployment describes the current rollout of a Kubernetes Deployment. It is used internally to create Anchore
// applications and is never serialized into the inventory report.
type Deployment struct {
	Name              string
	Namespace         string
	NamespaceUID      string
	UID               string
	Revision          string
	ReplicaSetName    string
	ReplicaSetUID     string
	PodTemplateHash   string
	ReplicaSetCreated time.Time
	// Containers running in pods owned by the current ReplicaSet, one per container name
	Containers []Container
}

func FetchDeploymentsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]appsv1.Deployment, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching deployments in namespace")
	var deployments []appsv1.Deployment

	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          batchSize,
			Continue:       cont,
			TimeoutSeconds: &timeout,
		}

		list, err := c.Clientset.AppsV1().Deployments(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list deployments in namespace %s: %w", namespace, err)
		}

		deployments = append(deployments, list.Items...)

		cont = list.GetListMeta().GetContinue()
		if cont == "" {
			break
		}
	}

	return deployments, nil
}

func FetchReplicaSetsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]appsv1.ReplicaSet, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching replicasets in namespace")
	var replicaSets []appsv1.ReplicaSet

	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          batchSize,
			Continue:       cont,
			TimeoutSeconds: &timeout,
		}

		list, err := c.Clientset.AppsV1().ReplicaSets(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list replicasets in namespace %s: %w", namespace, err)
		}

		replicaSets = append(replicaSets, list.Items...)

		cont = list.GetListMeta().GetContinue()
		if cont == "" {
			break
		}
	}

	return replicaSets, nil
}

func isOwnedBy(refs []metav1.OwnerReference, kind, uid string) bool {
	for _, ref := range refs {
		if ref.Kind == kind && string(ref.UID) == uid {
			return true
		}
	}
	return false
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

// containersForReplicaSet returns the containers of running pods owned by the ReplicaSet, de-duplicated by container
// name and preferring containers with an image digest
func containersForReplicaSet(rsUID string, pods []v1.Pod, containers []Container) []Container {
	podUIDs := make(map[string]struct{})
	for _, p := range pods {
		if p.Status.Phase == v1.PodRunning && isOwnedBy(p.OwnerReferences, "ReplicaSet", rsUID) {
			podUIDs[string(p.UID)] = struct{}{}
		}
	}

	var result []Container
	index := make(map[string]int)
	for _, c := range containers {
		if _, ok := podUIDs[c.PodUID]; !ok {
			continue
		}
		i, seen := index[c.Name]
		switch {
		case !seen:
			index[c.Name] = len(result)
			result = append(result, c)
		case result[i].ImageDigest == "" && c.ImageDigest != "":
			result[i] = c
		}
	}
	return result
}

// ProcessDeployments determines the current rollout of each deployment and the containers running for it.
// Deployments without a current ReplicaSet or without running containers are omitted.
func ProcessDeployments(deps []appsv1.Deployment, rss []appsv1.ReplicaSet, pods []v1.Pod, containers []Container, namespaceUID string) []Deployment {
	var result []Deployment
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
		depContainers := containersForReplicaSet(string(rs.UID), pods, containers)
		if len(depContainers) == 0 {
			log.Debugf("Skipping deployment %s/%s: no running containers for replicaset %s", dep.Namespace, dep.Name, rs.Name)
			continue
		}
		result = append(result, Deployment{
			Name:              dep.Name,
			Namespace:         dep.Namespace,
			NamespaceUID:      namespaceUID,
			UID:               string(dep.UID),
			Revision:          revision,
			ReplicaSetName:    rs.Name,
			ReplicaSetUID:     string(rs.UID),
			PodTemplateHash:   rs.Labels[PodTemplateHashLabel],
			ReplicaSetCreated: rs.CreationTimestamp.UTC(),
			Containers:        depContainers,
		})
	}
	return result
}
