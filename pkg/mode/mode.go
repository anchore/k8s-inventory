/*
Determines the Execution Modes supported by the application.
  - adhoc: the application will poll the k8s API once and then print and report (if configured) its findings
  - periodic: the application will poll the k8s API on an interval (polling-interval-seconds) and report (if configured) its findings
*/
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

// Parse the Mode from the user specified string (should match one of modeStr - see above). If no matches, we fallback to adhoc
func ParseMode(userStr string) Mode {
	switch strings.ToLower(userStr) {
	case strings.ToLower(PeriodicPolling.String()):
		return PeriodicPolling
	default:
		return AdHoc
	}
}

// Convert the mode object to a string
func (o Mode) String() string {
	if int(o) >= len(modeStr) || o < 0 {
		return modeStr[0]
	}

	return modeStr[o]
}
