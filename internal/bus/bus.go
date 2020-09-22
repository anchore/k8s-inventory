/*
Package bus handles asynchronous communication across components in the application via a publisher/subscriber model
*/
package bus

import "github.com/wagoodman/go-partybus"

var publisher partybus.Publisher
var active bool

// SetPublisher for the given context
func SetPublisher(p partybus.Publisher) {
	publisher = p
	if p != nil {
		active = true
	}
}

// Publish an event to all subscribers for the given event type / context
func Publish(event partybus.Event) {
	if active {
		publisher.Publish(event)
	}
}
