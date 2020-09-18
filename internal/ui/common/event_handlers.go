package common

import (
	"fmt"
	"os"

	kaiEventParsers "github.com/anchore/kai/kai/event/parsers"
	"github.com/wagoodman/go-partybus"
)

func ImageResultsRetrievedHandler(event partybus.Event) error {
	// show the report to stdout
	pres, err := kaiEventParsers.ParseImageResultsRetrieved(event)
	if err != nil {
		return fmt.Errorf("bad Kai event: %w", err)
	}

	if err := pres.Present(os.Stdout); err != nil {
		return fmt.Errorf("unable to show Kai results: %w", err)
	}
	return nil
}
