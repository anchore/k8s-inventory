package ui

import (
	"context"
	"sync"

	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/jotframe/pkg/frame"
)

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
}

func (r *Handler) RespondsTo(event partybus.Event) bool {
	return false
}

func (r *Handler) Handle(ctx context.Context, fr *frame.Frame, event partybus.Event, wg *sync.WaitGroup) error {
	return nil
}
