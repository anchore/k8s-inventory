package inventory

import (
	"k8s.io/apimachinery/pkg/version"
)

type Report struct {
	Timestamp             string        `json:"timestamp,omitempty"` // Should be generated using time.Now.UTC() and formatted according to RFC Y-M-DTH:M:SZ
	Results               []ReportItem  `json:"results"`
	ServerVersionMetadata *version.Info `json:"serverVersionMetadata"`
	ClusterName           string        `json:"cluster_name"`
	InventoryType         string        `json:"inventory_type"`
}
