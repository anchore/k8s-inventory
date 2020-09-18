package result

type Result struct {
	Timestamp string      `json:"timestamp"` // Should be generated using time.Now.UTC() and formatted according to RFC Y-M-DTH:M:SZ
	Results   []Namespace `json:"results"`
}
