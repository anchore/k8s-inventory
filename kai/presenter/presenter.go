// This package represents the presenters used to print Image Results to STDOut
package presenter

import (
	"io"

	"github.com/anchore/kai/kai/inventory"
	"github.com/anchore/kai/kai/presenter/json"
	"github.com/anchore/kai/kai/presenter/table"
)

// Presenter is the main interface other Presenters need to implement
type Presenter interface {
	Present(io.Writer) error
}

// GetPresenter retrieves a Presenter that matches a CLI option
func GetPresenter(option Option, result inventory.Result) Presenter {
	switch option {
	case JSONPresenter:
		return json.NewPresenter(result)
	case TablePresenter:
		return table.NewPresenter(result)
	default:
		return nil
	}
}
