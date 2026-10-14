package inventory

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/pkg/client"
)

// NodeFromV1 converts a kubernetes node into its inventory representation,
// applying the configured metadata collection rules
func NodeFromV1(n *v1.Node, includeAnnotations, includeLabels []string, disableMetadata bool) Node {
	node := Node{
		Name:                    n.Name,
		UID:                     string(n.UID),
		Arch:                    n.Status.NodeInfo.Architecture,
		ContainerRuntimeVersion: n.Status.NodeInfo.ContainerRuntimeVersion,
		KernelVersion:           n.Status.NodeInfo.KernelVersion,
		KubeletVersion:          n.Status.NodeInfo.KubeletVersion,
		OperatingSystem:         n.Status.NodeInfo.OperatingSystem,
	}
	if !disableMetadata {
		node.Annotations = processAnnotationsOrLabels(n.Annotations, includeAnnotations)
		node.Labels = processAnnotationsOrLabels(n.Labels, includeLabels)
	}
	return node
}

func FetchNodes(c client.Client, batchSize, timeout int64, includeAnnotations, includeLabels []string, disableMetadata bool) (map[string]Node, error) {
	nodes := make(map[string]Node)

	cont := ""
	for {
		opts := metav1.ListOptions{
			Limit:          batchSize,
			Continue:       cont,
			TimeoutSeconds: &timeout,
		}

		list, err := c.Clientset.CoreV1().Nodes().List(context.Background(), opts)
		if err != nil {
			if k8sErrors.IsForbidden(err) {
				log.Warnf("failed to list nodes: %w", err)
				return nil, nil
			}
			return nil, fmt.Errorf("failed to list nodes: %w", err)
		}

		for i := range list.Items {
			n := &list.Items[i]
			nodes[n.Name] = NodeFromV1(n, includeAnnotations, includeLabels, disableMetadata)
		}

		cont = list.GetListMeta().GetContinue()
		if cont == "" {
			break
		}
	}

	return nodes, nil
}
