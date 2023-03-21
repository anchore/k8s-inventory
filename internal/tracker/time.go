package tracker

import (
	"time"

	"github.com/anchore/kai/internal/log"
)

// TrackFunctionTime is a function that tracks the time it takes to execute a function
// and logs the time it took to execute the function
//
// It takes a time.Time object and a string as parameters
// The time.Time object is the time the function started executing
// The string is the name of the function that is being tracked (or any arbitrary message useful for logging)
//
// It is intended to be run at the beginning of a function and defer the function call
// for example:
//
//	func someFunction() {
//		start := time.Now()
//		defer TrackFunctionTime(start, "someFunction")
//		// do stuff
//	}
func TrackFunctionTime(start time.Time, msg string) {
	elapsed := time.Since(start)
	log.Log.Debugf("%s took %s", msg, elapsed)
}
