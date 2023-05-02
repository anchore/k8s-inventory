package inventory

import (
	"k8s.io/apimachinery/pkg/version"
)

type Namespace struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Name        string            `json:"name"`
	UID         string            `json:"uid"`
}

type Container struct {
	ID          string `json:"id"`
	ImageDigest string `json:"image_digest"`
	ImageTag    string `json:"image_tag"`
	Name        string `json:"name"`
	PodUID      string `json:"pod_uid"`
}

type Node struct {
	Annotations             map[string]string `json:"annotations,omitempty"`
	Arch                    string            `json:"arch,omitempty"`
	ContainerRuntimeVersion string            `json:"container_runtime_version,omitempty"`
	KernelVersion           string            `json:"kernel_version,omitempty"`
	KubeProxyVersion        string            `json:"kube_proxy_version,omitempty"`
	KubeletVersion          string            `json:"kubelet_version,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty"`
	Name                    string            `json:"name"`
	OperatingSystem         string            `json:"operating_system,omitempty"`
	UID                     string            `json:"uid"`
}

type Pod struct {
	Annotations  map[string]string `json:"annotations,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Name         string            `json:"name"`
	NamespaceUID string            `json:"namespace_uid"`
	NodeUID      string            `json:"node_uid,omitempty"`
	UID          string            `json:"uid"`
}

type Report struct {
	ClusterName           string        `json:"cluster_name"`
	Containers            []Container   `json:"containers"`
	Namespaces            []Namespace   `json:"namespaces,omitempty"`
	Nodes                 []Node        `json:"nodes,omitempty"`
	Pods                  []Pod         `json:"pods,omitempty"`
	ServerVersionMetadata *version.Info `json:"serverVersionMetadata"`
	Timestamp             string        `json:"timestamp,omitempty"` // Should be generated using time.Now.UTC() and formatted according to RFC Y-M-DTH:M:SZ
}
