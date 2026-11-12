package inventory

import (
	"context"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/client"
)

func FetchStatefulSetsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]appsv1.StatefulSet, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching statefulsets in namespace")
	return listAll(batchSize, timeout, "statefulsets", namespace, func(opts metav1.ListOptions) ([]appsv1.StatefulSet, metav1.ListInterface, error) {
		l, err := c.Clientset.AppsV1().StatefulSets(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, nil, err
		}
		return l.Items, l, nil
	})
}

func FetchDaemonSetsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]appsv1.DaemonSet, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching daemonsets in namespace")
	return listAll(batchSize, timeout, "daemonsets", namespace, func(opts metav1.ListOptions) ([]appsv1.DaemonSet, metav1.ListInterface, error) {
		l, err := c.Clientset.AppsV1().DaemonSets(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, nil, err
		}
		return l.Items, l, nil
	})
}

func FetchControllerRevisionsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]appsv1.ControllerRevision, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching controllerrevisions in namespace")
	return listAll(batchSize, timeout, "controllerrevisions", namespace, func(opts metav1.ListOptions) ([]appsv1.ControllerRevision, metav1.ListInterface, error) {
		l, err := c.Clientset.AppsV1().ControllerRevisions(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, nil, err
		}
		return l.Items, l, nil
	})
}

// controllerRevisionWorkload builds the Workload for a StatefulSet or DaemonSet from its current ControllerRevision,
// returning false when no running pod of that revision has containers
func controllerRevisionWorkload(kind WorkloadKind, meta metav1.ObjectMeta, cr *appsv1.ControllerRevision,
	pods []v1.Pod, containers []Container, namespaceUID string,
) (Workload, bool) {
	hash := controllerRevisionHash(cr, meta.Name)
	uid := string(meta.UID)
	wlContainers := containersForPods(pods, containers, func(p v1.Pod) bool {
		return isOwnedBy(p.OwnerReferences, string(kind), uid) && podInControllerRevision(p, cr, hash)
	})
	if len(wlContainers) == 0 {
		log.Debugf("Skipping %s %s/%s: no running containers for controllerrevision %s", kind, meta.Namespace, meta.Name, cr.Name)
		return Workload{}, false
	}
	return Workload{
		Kind:            kind,
		Name:            meta.Name,
		Namespace:       meta.Namespace,
		NamespaceUID:    namespaceUID,
		UID:             uid,
		Revision:        strconv.FormatInt(cr.Revision, 10),
		RevisionName:    cr.Name,
		RevisionHash:    hash,
		RevisionCreated: cr.CreationTimestamp.UTC(),
		Relations:       relationsFor(meta),
		Containers:      wlContainers,
	}, true
}

// ProcessStatefulSets determines the current rollout of each StatefulSet (its update revision) and the containers
// running for it. StatefulSets without a current ControllerRevision or without running containers are omitted.
func ProcessStatefulSets(stss []appsv1.StatefulSet, crs []appsv1.ControllerRevision, pods []v1.Pod, containers []Container, namespaceUID string) []Workload {
	var result []Workload
	for _, sts := range stss {
		owned := ownedControllerRevisions(KindStatefulSet, string(sts.UID), crs)
		var current *appsv1.ControllerRevision
		for _, cr := range owned {
			if cr.Name == sts.Status.UpdateRevision {
				current = cr
				break
			}
		}
		if current == nil {
			// status not yet reported by the controller: fall back to the newest revision
			current = latestControllerRevision(owned)
		}
		if current == nil {
			log.Debugf("Skipping statefulset %s/%s: no controllerrevision found", sts.Namespace, sts.Name)
			continue
		}
		if wl, ok := controllerRevisionWorkload(KindStatefulSet, sts.ObjectMeta, current, pods, containers, namespaceUID); ok {
			result = append(result, wl)
		}
	}
	return result
}

// ProcessDaemonSets determines the current rollout of each DaemonSet and the containers running for it. DaemonSets
// report no update revision in their status, so the ControllerRevision with the highest revision is used.
func ProcessDaemonSets(dss []appsv1.DaemonSet, crs []appsv1.ControllerRevision, pods []v1.Pod, containers []Container, namespaceUID string) []Workload {
	var result []Workload
	for _, ds := range dss {
		current := latestControllerRevision(ownedControllerRevisions(KindDaemonSet, string(ds.UID), crs))
		if current == nil {
			log.Debugf("Skipping daemonset %s/%s: no controllerrevision found", ds.Namespace, ds.Name)
			continue
		}
		if wl, ok := controllerRevisionWorkload(KindDaemonSet, ds.ObjectMeta, current, pods, containers, namespaceUID); ok {
			result = append(result, wl)
		}
	}
	return result
}
