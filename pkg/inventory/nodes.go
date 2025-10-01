package inventory

import (
	"context"
	"fmt"

	"github.com/anchore/k8s-inventory/internal/log"
	"github.com/anchore/k8s-inventory/pkg/client"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

		for _, n := range list.Items {
			if !disableMetadata {
				annotations := processAnnotationsOrLabels(n.Annotations, includeAnnotations)
				labels := processAnnotationsOrLabels(n.Labels, includeLabels)
				nodes[n.Name] = Node{
					Name:                    n.Name,
					UID:                     string(n.UID),
					Annotations:             annotations,
					Arch:                    n.Status.NodeInfo.Architecture,
					ContainerRuntimeVersion: n.Status.NodeInfo.ContainerRuntimeVersion,
					KernelVersion:           n.Status.NodeInfo.KernelVersion,
					KubeletVersion:          n.Status.NodeInfo.KubeletVersion,
					Labels:                  labels,
					OperatingSystem:         n.Status.NodeInfo.OperatingSystem,
				}
			} else {
				nodes[n.Name] = Node{
					Name:                    n.Name,
					UID:                     string(n.UID),
					Arch:                    n.Status.NodeInfo.Architecture,
					ContainerRuntimeVersion: n.Status.NodeInfo.ContainerRuntimeVersion,
					KernelVersion:           n.Status.NodeInfo.KernelVersion,
					KubeletVersion:          n.Status.NodeInfo.KubeletVersion,
					OperatingSystem:         n.Status.NodeInfo.OperatingSystem,
				}
			}
		}

		cont = list.GetListMeta().GetContinue()
		if cont == "" {
			break
		}
	}

	return nodes, nil
}
