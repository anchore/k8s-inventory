package presenter

import (
	"github.com/anchore/kai/kai/presenter/json"
	"github.com/anchore/kai/kai/presenter/table"
	"github.com/anchore/kai/kai/result"
	"io"
)

// Presenter is the main interface other Presenters need to implement
type Presenter interface {
	Present(io.Writer) error
}

// GetPresenter retrieves a Presenter that matches a CLI option
func GetPresenter(option Option, result result.Result) Presenter {
	switch option {
	case JSONPresenter:
		return json.NewPresenter(result)
	case TablePresenter:
		return table.NewPresenter(result)
	default:
		return nil
	}
}