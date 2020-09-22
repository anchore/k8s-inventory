// Defines the events that get sent in Kai (for asynchronous handling from normal execution)
package event

import "github.com/wagoodman/go-partybus"

const (
	AppUpdateAvailable    partybus.EventType = "kai-app-update-available"
	ImageResultsRetrieved partybus.EventType = "kai-image-results-retrieved"
)
