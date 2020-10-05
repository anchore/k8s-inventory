// Event Handlers (for events received from partybus)
package common

import (
	"fmt"
	"os"

	"github.com/anchore/kai/internal/log"

	"github.com/anchore/kai/internal/config"
	kaiEventParsers "github.com/anchore/kai/kai/event/parsers"
	"github.com/anchore/kai/kai/reporter"
	"github.com/wagoodman/go-partybus"
)

// Log the Image Results to STDOUT and report to Anchore (if configured)
func ImageResultsRetrievedHandler(event partybus.Event, appConfig *config.Application) error {
	// show the report to stdout
	pres, imagesResult, err := kaiEventParsers.ParseImageResultsRetrieved(event)
	if err != nil {
		return fmt.Errorf("bad Kai event: %w", err)
	}

	if appConfig.AnchoreDetails.IsValid() {
		if err := reporter.Report(imagesResult, appConfig.AnchoreDetails); err != nil {
			return fmt.Errorf("unable to report Images to Anchore: %w", err)
		}
	} else {
		log.Debug("Anchore details not specified, not reporting in-use image data")
	}

	if err := pres.Present(os.Stdout); err != nil {
		return fmt.Errorf("unable to show Kai results: %w", err)
	}

	return nil
}
