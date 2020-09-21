package common

import (
	"fmt"
	"io"
	"os"

	"github.com/anchore/kai/internal"
	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/kai/reporter"
	"github.com/gookit/color"
	"github.com/wagoodman/jotframe/pkg/frame"

	kaiEventParsers "github.com/anchore/kai/kai/event/parsers"
	"github.com/wagoodman/go-partybus"
)

func ImageResultsRetrievedHandler(event partybus.Event, appConfig *config.Application) error {
	// show the report to stdout
	pres, imagesResult, err := kaiEventParsers.ParseImageResultsRetrieved(event)
	if err != nil {
		return fmt.Errorf("bad Kai event: %w", err)
	}

	if appConfig.HasAnchoreDetails() {
		if err := reporter.Report(imagesResult, appConfig.AnchoreDetails); err != nil {
			return fmt.Errorf("unable to report Images to Anchore: %w", err)
		}
	} else {
		fmt.Println("Anchore details not specified, not reporting in-use image data")
	}

	if err := pres.Present(os.Stdout); err != nil {
		return fmt.Errorf("unable to show Kai results: %w", err)
	}

	return nil
}

func AppUpdateAvailableHandler(fr *frame.Frame, event partybus.Event) error {
	newVersion, err := kaiEventParsers.ParseAppUpdateAvailable(event)
	if err != nil {
		return fmt.Errorf("bad %s event: %w", event.Type, err)
	}

	line, err := fr.Prepend()
	if err != nil {
		return err
	}

	message := color.Magenta.Sprintf("New version of %s is available: %s", internal.ApplicationName, newVersion)
	_, _ = io.WriteString(line, message)

	return nil
}
