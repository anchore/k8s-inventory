package inventory

type Report struct {
	Result
	ClusterName   string `json:"cluster_name"`
	InventoryType string `json:"inventory_type"`
}

func NewReport(result Result, clusterName string) *Report {
	return &Report{
		result,
		clusterName,
		"kubernetes",
	}
}
