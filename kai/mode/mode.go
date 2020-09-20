package mode

import "strings"

const (
	AdHoc Mode = iota
	PeriodicPolling
)

var modeStr = []string{
	"adhoc",
	"periodic",
}

var Modes = []Mode{
	AdHoc,
	PeriodicPolling,
}

type Mode int

func ParseMode(userStr string) Mode {
	switch strings.ToLower(userStr) {
	case strings.ToLower(PeriodicPolling.String()):
		return PeriodicPolling
	default:
		return AdHoc
	}
}

func (o Mode) String() string {
	if int(o) >= len(modeStr) || o < 0 {
		return modeStr[0]
	}

	return modeStr[o]
}
