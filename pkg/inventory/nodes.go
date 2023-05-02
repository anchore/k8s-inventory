package inventory

import (
	"context"
	"fmt"

	"github.com/anchore/k8s-inventory/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FetchNodes(c client.Client, batchSize, timeout int64) (map[string]Node, error) {
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
			return nil, fmt.Errorf("failed to list nodes: %w", err)
		}

		for _, n := range list.Items {
			nodes[n.ObjectMeta.Name] = Node{
				Name:                    n.ObjectMeta.Name,
				UID:                     string(n.UID),
				Annotations:             n.Annotations,
				Arch:                    n.Status.NodeInfo.Architecture,
				ContainerRuntimeVersion: n.Status.NodeInfo.ContainerRuntimeVersion,
				KernelVersion:           n.Status.NodeInfo.KernelVersion,
				KubeProxyVersion:        n.Status.NodeInfo.KubeProxyVersion,
				KubeletVersion:          n.Status.NodeInfo.KubeletVersion,
				Labels:                  n.Labels,
				OperatingSystem:         n.Status.NodeInfo.OperatingSystem,
			}
		}

		cont = list.GetListMeta().GetContinue()
		if cont == "" {
			break
		}
	}

	return nodes, nil
}
