package ui

import (
	"bytes"
	"fmt"
	"github.com/anchore/kai/internal/config"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/internal/logger"
	"github.com/anchore/kai/internal/ui/common"
	kaiEvent "github.com/anchore/kai/kai/event"
	"github.com/anchore/kai/kai/mode"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/jotframe/pkg/frame"
	"os"
)

func LoggerUI(workerErrs <-chan error, subscription *partybus.Subscription, appConfig *config.Application) error {
	// prep the logger to not clobber the screen from now on (logrus only)
	logBuffer := bytes.NewBufferString("")
	logWrapper, ok := log.Log.(*logger.LogrusLogger)
	if ok {
		logWrapper.Logger.SetOutput(logBuffer)
	}

	fr := setupScreen()
	if fr == nil {
		return fmt.Errorf("unable to setup screen")
	}
	var isClosed bool
	defer teardownFrame(isClosed, fr, logBuffer)

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

			switch {
			case e.Type == kaiEvent.AppUpdateAvailable:
				if err := common.AppUpdateAvailableHandler(fr, e); err != nil {
					log.Errorf("unable to show %s event: %+v", e.Type, err)
				}

			case e.Type == kaiEvent.ImageResultsRetrieved:
				err := common.ImageResultsRetrievedHandler(e, appConfig)
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

func teardownFrame(isClosed bool, fr *frame.Frame, logBuffer *bytes.Buffer) {
	if !isClosed {
		_ = fr.Close()
		_ = frame.Close()
		// flush any errors to the screen before the report
		_, _ = fmt.Fprint(os.Stderr, logBuffer.String())
		isClosed = true
	}
	logWrapper, ok := log.Log.(*logger.LogrusLogger)
	if ok {
		logWrapper.Logger.SetOutput(os.Stderr)
	}
}

func setupScreen() *frame.Frame {
	frameConfig := frame.Config{
		PositionPolicy: frame.PolicyFloatForward,
		// only report output to stderr, reserve report output for stdout
		Output: os.Stderr,
	}

	fr, err := frame.New(frameConfig)
	if err != nil {
		log.Errorf("failed to create screen object: %+v", err)
		return nil
	}
	return fr
}
