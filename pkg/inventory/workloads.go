package inventory

import (
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkloadKind is the kind of Kubernetes workload that Anchore applications are created from
type WorkloadKind string

const (
	KindDeployment  WorkloadKind = "Deployment"
	KindStatefulSet WorkloadKind = "StatefulSet"
	KindDaemonSet   WorkloadKind = "DaemonSet"
)

const (
	// ControllerRevisionHashLabel is set on StatefulSet pods (to the ControllerRevision name) and on DaemonSet pods
	// and ControllerRevisions (to the pod template hash)
	ControllerRevisionHashLabel = appsv1.ControllerRevisionHashLabelKey
	// ControllerRevisionHashLabelV2 is set by Kubernetes on ControllerRevisions it creates, to the pod template hash
	ControllerRevisionHashLabelV2 = "controller.kubernetes.io/hash"

	// Conventions that relate workloads to each other. Kubernetes itself has no relationship between workloads of
	// different kinds; these are set by Helm and by charts following the recommended labels.
	HelmReleaseAnnotation = "meta.helm.sh/release-name"
	AppInstanceLabel      = "app.kubernetes.io/instance"
	AppPartOfLabel        = "app.kubernetes.io/part-of"
	AppComponentLabel     = "app.kubernetes.io/component"
)

// Relations records how a workload relates to other workloads by convention (Helm release, recommended labels) and
// to the controller that owns it (e.g. an operator custom resource). Any field may be empty.
type Relations struct {
	HelmRelease string
	Instance    string
	PartOf      string
	Component   string
	OwnerKind   string
	OwnerName   string
}

// Workload describes the current rollout of a Kubernetes Deployment, StatefulSet or DaemonSet. It is used internally
// to create Anchore applications and is never serialized into the inventory report.
type Workload struct {
	Kind         WorkloadKind
	Name         string
	Namespace    string
	NamespaceUID string
	UID          string
	// Revision is the rollout number: the Deployment revision, or the ControllerRevision revision for StatefulSets
	// and DaemonSets. A rollback is a new, higher revision.
	Revision string
	// RevisionName is the object holding the current pod template: a ReplicaSet for Deployments, a
	// ControllerRevision for StatefulSets and DaemonSets
	RevisionName string
	// RevisionHash is the hash of the current pod template
	RevisionHash    string
	RevisionCreated time.Time
	Relations       Relations
	// Containers running in pods of the current revision, one per container name
	Containers []Container
}

// RevisionKind is the kind of object named by RevisionName
func (w Workload) RevisionKind() string {
	if w.Kind == KindDeployment {
		return "ReplicaSet"
	}
	return "ControllerRevision"
}

func relationsFor(meta metav1.ObjectMeta) Relations {
	r := Relations{
		HelmRelease: meta.Annotations[HelmReleaseAnnotation],
		Instance:    meta.Labels[AppInstanceLabel],
		PartOf:      meta.Labels[AppPartOfLabel],
		Component:   meta.Labels[AppComponentLabel],
	}
	if owner := metav1.GetControllerOfNoCopy(&meta); owner != nil {
		r.OwnerKind = owner.Kind
		r.OwnerName = owner.Name
	}
	return r
}

func isOwnedBy(refs []metav1.OwnerReference, kind, uid string) bool {
	for _, ref := range refs {
		if ref.Kind == kind && string(ref.UID) == uid {
			return true
		}
	}
	return false
}

// containersForPods returns the containers of running pods matched by isCurrent, de-duplicated by container name and
// preferring containers with an image digest
func containersForPods(pods []v1.Pod, containers []Container, isCurrent func(v1.Pod) bool) []Container {
	podUIDs := make(map[string]struct{})
	for _, p := range pods {
		if p.Status.Phase == v1.PodRunning && isCurrent(p) {
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

// controllerRevisionHash returns the pod template hash of a ControllerRevision owned by the named workload
func controllerRevisionHash(cr *appsv1.ControllerRevision, ownerName string) string {
	if h := cr.Labels[ControllerRevisionHashLabelV2]; h != "" {
		return h
	}
	if h := cr.Labels[ControllerRevisionHashLabel]; h != "" {
		return h
	}
	return strings.TrimPrefix(cr.Name, ownerName+"-")
}

// ownedControllerRevisions returns the ControllerRevisions owned by the workload
func ownedControllerRevisions(kind WorkloadKind, uid string, crs []appsv1.ControllerRevision) []*appsv1.ControllerRevision {
	var owned []*appsv1.ControllerRevision
	for i := range crs {
		if isOwnedBy(crs[i].OwnerReferences, string(kind), uid) {
			owned = append(owned, &crs[i])
		}
	}
	return owned
}

// latestControllerRevision returns the revision with the highest revision number. The controllers give the revision
// matching the current pod template the highest number, including after a rollback.
func latestControllerRevision(owned []*appsv1.ControllerRevision) *appsv1.ControllerRevision {
	var latest *appsv1.ControllerRevision
	for _, cr := range owned {
		if latest == nil || cr.Revision > latest.Revision {
			latest = cr
		}
	}
	return latest
}

// podInControllerRevision reports whether the pod runs the ControllerRevision: StatefulSet pods are labelled with the
// ControllerRevision name, DaemonSet pods with its hash
func podInControllerRevision(p v1.Pod, cr *appsv1.ControllerRevision, hash string) bool {
	v := p.Labels[ControllerRevisionHashLabel]
	return v != "" && (v == cr.Name || v == hash)
}
