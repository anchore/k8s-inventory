package etui

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/anchore/kai/internal"
	kaiEventParsers "github.com/anchore/kai/kai/event/parsers"

	"github.com/gookit/color"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/jotframe/pkg/frame"
)

func appUpdateAvailableHandler(_ context.Context, fr *frame.Frame, event partybus.Event, _ *sync.WaitGroup) error {
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
