package inventory

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/anchore/k8s-inventory/internal/tracker"
	"github.com/anchore/k8s-inventory/pkg/client"
)

func FetchPodsInNamespace(c client.Client, batchSize, timeout int64, namespace string) ([]v1.Pod, error) {
	defer tracker.TrackFunctionTime(time.Now(), "Fetching pods in namespace")
	var podList []v1.Pod

	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          batchSize,
			Continue:       cont,
			TimeoutSeconds: &timeout,
		}

		list, err := c.Clientset.CoreV1().Pods(namespace).List(context.Background(), opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
		}

		podList = append(podList, list.Items...)

		cont = list.GetListMeta().GetContinue()
		if cont == "" {
			break
		}
	}

	return podList, nil
}

func ProcessPods(pods []v1.Pod, namespaceUID string, nodes map[string]Node, includeAnnotations, includeLabels []string, disableMetadata bool) []Pod {
	var podList []Pod

	for _, p := range pods {
		pod := Pod{
			Name:         p.Name,
			UID:          string(p.UID),
			NamespaceUID: namespaceUID,
		}
		if !disableMetadata {
			pod.Labels = processAnnotationsOrLabels(p.Labels, includeLabels)
			pod.Annotations = processAnnotationsOrLabels(p.Annotations, includeAnnotations)
		}
		node, ok := nodes[p.Spec.NodeName]
		if ok {
			pod.NodeUID = node.UID
		}
		podList = append(podList, pod)
	}

	return podList
}
