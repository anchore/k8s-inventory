package ui

import (
	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/internal/ui/common"
	kaiEvent "github.com/anchore/kai/kai/event"
	"github.com/anchore/kai/kai/mode"
	"github.com/wagoodman/go-partybus"
)

func LoggerUI(workerErrs <-chan error, subscription *partybus.Subscription, appConfig *config.Application) error {
	events := subscription.Events()
	var errResult error
	for {
		select {
		case err, ok := <-workerErrs:
			if err != nil {
				return err
			}
			if !ok {
				workerErrs = nil
			}
		case e, ok := <-events:
			if !ok {
				// event bus closed...
				events = nil
			}

			// ignore all events except for the final event
			if e.Type == kaiEvent.ImageResultsRetrieved {
				err := common.ImageResultsRetrievedHandler(e)
				if err != nil {
					log.Errorf("unable to show %s event: %+v", e.Type, err)
				}

				// this is the last expected event (if we're not running periodically)
				if appConfig.RunMode != mode.PeriodicPolling {
					events = nil
				}
			}
		}
		if events == nil && workerErrs == nil {
			break
		}
	}

	return errResult
}
