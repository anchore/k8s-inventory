// This package represents the presenters used to print Image Results to STDOut
package presenter

import (
	"io"

	"github.com/anchore/k8s-inventory/pkg/inventory"
	"github.com/anchore/k8s-inventory/pkg/presenter/json"
	"github.com/anchore/k8s-inventory/pkg/presenter/table"
)

// Presenter is the main interface other Presenters need to implement
type Presenter interface {
	Present(io.Writer) error
}

// GetPresenter retrieves a Presenter that matches a CLI option
func GetPresenter(option Option, report inventory.Report) Presenter {
	switch option {
	case JSONPresenter:
		return json.NewPresenter(report)
	case TablePresenter:
		return table.NewPresenter(report)
	default:
		return nil
	}
}
