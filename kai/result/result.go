package result

type Result struct {
	Timestamp string // Should be generated using time.Now.UTC() and formatted according to RFC Y-M-DTH:M:SZ
	Results   []Namespace
}
