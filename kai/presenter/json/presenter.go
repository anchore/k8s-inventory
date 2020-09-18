package json

import (
	"encoding/json"
	"io"

	"github.com/anchore/kai/kai/result"
)

// Presenter is a generic struct for holding fields needed for reporting
type Presenter struct {
	result result.Result
}

// NewPresenter is a *Presenter constructor
func NewPresenter(result result.Result) *Presenter {
	return &Presenter{
		result: result,
	}
}

// Present creates a JSON-based reporting
func (pres *Presenter) Present(output io.Writer) error {
	enc := json.NewEncoder(output)
	// prevent > and < from being escaped in the payload
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	return enc.Encode(pres.result)
}
