package kai

import (
	"github.com/anchore/kai/internal/bus"
	"github.com/anchore/kai/internal/log"
	"github.com/anchore/kai/kai/logger"
	"github.com/wagoodman/go-partybus"
)

func SetLogger(logger logger.Logger) {
	log.Log = logger
}

func SetBus(b *partybus.Bus) {
	bus.SetPublisher(b)
}
