package parsers

import (
	"fmt"

	"github.com/anchore/kai/kai/result"

	"github.com/anchore/kai/kai/event"
	"github.com/anchore/kai/kai/presenter"
	"github.com/wagoodman/go-partybus"
)

type ErrBadPayload struct {
	Type  partybus.EventType
	Field string
	Value interface{}
}

func (e *ErrBadPayload) Error() string {
	return fmt.Sprintf("event='%s' has bad event payload field='%v': '%+v'", string(e.Type), e.Field, e.Value)
}

func newPayloadErr(t partybus.EventType, field string, value interface{}) error {
	return &ErrBadPayload{
		Type:  t,
		Field: field,
		Value: value,
	}
}

func checkEventType(actual, expected partybus.EventType) error {
	if actual != expected {
		return newPayloadErr(expected, "Type", actual)
	}
	return nil
}

func ParseAppUpdateAvailable(e partybus.Event) (string, error) {
	if err := checkEventType(e.Type, event.AppUpdateAvailable); err != nil {
		return "", err
	}

	newVersion, ok := e.Value.(string)
	if !ok {
		return "", newPayloadErr(e.Type, "Value", e.Value)
	}

	return newVersion, nil
}

func ParseImageResultsRetrieved(e partybus.Event) (presenter.Presenter, result.Result, error) {
	if err := checkEventType(e.Type, event.ImageResultsRetrieved); err != nil {
		return nil, result.Result{}, err
	}

	pres, ok := e.Value.(presenter.Presenter)
	if !ok {
		return nil, result.Result{}, newPayloadErr(e.Type, "Value", e.Value)
	}

	imagesResult, ok := e.Source.(result.Result)
	if !ok {
		return nil, result.Result{}, newPayloadErr(e.Type, "Source", e.Source)
	}

	return pres, imagesResult, nil
}
