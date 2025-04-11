package faye

import (
	"encoding/json"
	"errors"
	"slices"
)

type decoder interface {
	Decode(any) error
}

// Transport models a faye protocol transport
type Transport interface {
	isUsable(string) bool
	connectionType() string
	close()
	send(json.Marshaler) (decoder, error)
	sendOnly(json.Marshaler) error
	read() (decoder, error)
	setURL(string)
	setTimeoutSeconds(int64)
}

func selectTransport(client *FayeClient, transportTypes []string) (Transport, error) {
	for _, transport := range registeredTransports {
		if slices.Contains(transportTypes, transport.connectionType()) && transport.isUsable(client.url) {
			return transport, nil
		}
	}
	return nil, errors.New("no usable transports available")
}
