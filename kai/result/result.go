// The structs here define the result format which is parsed from K8s Client requests
package result

import (
	"k8s.io/apimachinery/pkg/version"
)

type Result struct {
	Timestamp             string        `json:"timestamp,omitempty"` // Should be generated using time.Now.UTC() and formatted according to RFC Y-M-DTH:M:SZ
	Results               []Namespace   `json:"results"`
	ServerVersionMetadata *version.Info `json:"serverVersionMetadata"`
}
