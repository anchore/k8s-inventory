// These are the supported Presenters for outputting In-Use-Image results
package presenter

import "strings"

const (
	UnknownPresenter Option = iota
	JSONPresenter
	TablePresenter
)

var optionStr = []string{
	"UnknownPresenter",
	"json",
	"table",
}

var Options = []Option{
	JSONPresenter,
	TablePresenter,
}

type Option int

// Parse the Presenter option from a string
func ParseOption(userStr string) Option {
	switch strings.ToLower(userStr) {
	case strings.ToLower(JSONPresenter.String()):
		return JSONPresenter
	case strings.ToLower(TablePresenter.String()):
		return TablePresenter
	default:
		return UnknownPresenter
	}
}

// Convert the Presenter option to a string
func (o Option) String() string {
	if int(o) >= len(optionStr) || o < 0 {
		return optionStr[0]
	}

	return optionStr[o]
}
