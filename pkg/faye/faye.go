package faye

import (
	"fmt"
)

const (
	LONG_POLLING = "long-polling"
	WEBSOCKET    = "websocket"
)

var (
	MANDATORY_CONNECTION_TYPES = []string{LONG_POLLING}
	registeredTransports       = []Transport{}
)

// Logger is the interface that faye uses for it's logger
type Logger interface {
	Infof(f string, a ...interface{})
	Errorf(f string, a ...interface{})
	Debugf(f string, a ...interface{})
	Warnf(f string, a ...interface{})
}

// Extension models a faye extension
type Extension interface {
	In(Message)
	Out(Message)
}

// Subscription models a subscription, containing the channel it is subscribed
// to and the chan object used to push messages through
type Subscription struct {
	channel     string
	msgChan     chan Message
	polling     bool
	stopPolling bool
}

func StackError(callsite string, err error) error {
	return fmt.Errorf("%s error:\n\t%s", callsite, err)
}

// RegisterTransports allows for the dynamic loading of different transports
// and the most suitable one will be selected
func RegisterTransports(transports []Transport) {
	registeredTransports = transports
}
