package ui

import (
	"github.com/anchore/kai/internal/config"
	"github.com/wagoodman/go-partybus"
)

type UI func(<-chan error, *partybus.Subscription, *config.Application) error
