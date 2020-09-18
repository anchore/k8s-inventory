package ui

import (
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/internal/ui/common"
	kaiEvent "github.com/anchore/kai/kai/event"
	"github.com/wagoodman/go-partybus"
)

func LoggerUI(workerErrs <-chan error, subscription *partybus.Subscription) error {
	events := subscription.Events()
eventLoop:
	for {
		select {
		case err := <-workerErrs:
			if err != nil {
				return err
			}
		case e, ok := <-events:
			if !ok {
				// event bus closed...
				break eventLoop
			}

			// ignore all events except for the final event
			if e.Type == kaiEvent.ImageResultsRetrieved {
				err := common.ImageResultsRetrievedHandler(e)
				if err != nil {
					log.Errorf("unable to show %s event: %+v", e.Type, err)
				}

				// this is the last expected event
				break eventLoop
			}
		}
	}

	return nil
}
