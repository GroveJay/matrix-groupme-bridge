package faye

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// State enums
	UNCONNECTED = 1
	CONNECTING  = 2
	CONNECTED   = 3
	// This is not documented? https://docs.cometd.org/current/reference/index.html#_client_state_table
	// But seems logical based on the advice.reconnect docs: https://docs.cometd.org/current/reference/index.html#_reconnect_advice_field
	DISCONNECTED = 4

	// Advice strings
	HANDSHAKE = "handshake"
	RETRY     = "retry"
	NONE      = "none"

	CONNECTION_TIMEOUT_SECONDS = 3.0 * 60
	WEBSOCKET_POLL_INITERVAL   = 30.0
	// DEFAULT_RETRY              = 5.0
	// MAX_REQUEST_SIZE           = 2048
)

// FayeClient models a faye client
type FayeClient struct {
	state              int
	url                string
	handshakeChannel   string
	connectChannel     string
	subscribeChannel   string
	unsubscribeChannel string
	subscriptions      []*Subscription
	transport          Transport
	log                Logger
	clientID           string
	nextHandshake      int64
	mutex              *sync.RWMutex // protects instance vars across goroutines
	extns              []Extension
	message_id         int
}

// NewFayeClient returns a new client for interfacing to a faye server
func NewFayeClient(
	url string,
	handshakeChannel string,
	connectChannel string,
	subscribeChannel string,
	unsubscribeChannel string) *FayeClient {

	fayeClient := &FayeClient{
		url:                url,
		handshakeChannel:   handshakeChannel,
		connectChannel:     connectChannel,
		subscribeChannel:   subscribeChannel,
		unsubscribeChannel: unsubscribeChannel,
		state:              UNCONNECTED,
		mutex:              &sync.RWMutex{},
		log:                fayeDefaultLogger{},
		message_id:         1,
	}

	return fayeClient
}

// Out spies on outgoing messages and prints them to the log, when the client
// is added to itself as an extension
func (faye *FayeClient) Out(msg Message) {
	switch v := msg.(type) {
	case msgWrapper:
		line := "→ "
		clientID := string([]rune(v.msg.ClientID)[:4])
		messageID := v.msg.ID
		switch channel := v.msg.Channel; channel {
		case "/meta/handshake":
			line = line + fmt.Sprintf("Handshake (%s)", messageID)
		case "/meta/connect":
			line = line + fmt.Sprintf("[%s] Connect (%s)", clientID, messageID)
		case "/meta/subscribe":
			line = line + fmt.Sprintf("[%s] %s Subscribe (%s)", clientID, v.msg.Subscription, messageID)
		default:
			if v.msg.Data["type"] != nil {
				line = line + fmt.Sprintf("[%s] %s - %s (%s)", clientID, channel, v.msg.Data["type"], messageID)
			} else {
				if strings.HasPrefix(channel, "/user/") {
					line = line + fmt.Sprintf("[%s] %s (%s)", clientID, channel, messageID)
				} else {
					b, _ := v.MarshalJSON()
					line = line + string(b)
				}
			}
		}
		faye.log.Debugf(line)
	}
}

// In spies on outgoing messages and prints them to the log, when the client
// is added to itself as an extension
func (faye *FayeClient) In(msg Message) {
	switch v := msg.(type) {
	case msgWrapper:
		line := "← "
		clientID := string([]rune(v.msg.ClientID)[:4])
		messageID := v.msg.ID

		switch channel := v.msg.Channel; channel {
		case "/meta/handshake":
			line = line + fmt.Sprintf("[%s] Handshake (%s)", clientID, messageID)
		case "/meta/connect":
			line = line + fmt.Sprintf("[%s] Connect (%s)", clientID, messageID)
		case "/meta/subscribe":
			line = line + fmt.Sprintf("[%s] Subscribe - %s (%s)", clientID, v.msg.Subscription, messageID)
		default:
			data_type := v.msg.Data["type"]
			if data_type != nil {
				line = line + fmt.Sprintf("[%s] %s - ", clientID, v.msg.Channel)
				if data_type == "ping" {
					line = line + fmt.Sprintf("ping (%s)", messageID)
				} else {
					b, _ := v.MarshalJSON()
					line = line + string(b)
				}
			} else {
				if strings.HasPrefix(channel, "/user/") {
					line = line + fmt.Sprintf("[%s] %s (%s)", clientID, channel, messageID)
				} else {
					b, _ := v.MarshalJSON()
					line = line + string(b)
				}
			}
		}

		faye.log.Debugf(line)
	}
}

// SetLogger attaches a Logger to the faye client, and replaces the default
// logger which just puts to stdout
func (faye *FayeClient) SetLogger(log Logger) {
	faye.log = log
}

func (faye *FayeClient) Connected() bool {
	return faye.state == CONNECTED
}

func (faye *FayeClient) HandshakeAndConnect() error {
	if err := faye.handshake(); err != nil {
		return StackError("handshake", err)
	}
	if err := faye.connect(); err != nil {
		return StackError("connect", err)
	}
	if faye.transport.connectionType() == WEBSOCKET {
		go faye.websocketReadPoll()
	}
	return nil
}

// AddExtension adds an extension to the Faye Client
func (faye *FayeClient) AddExtension(extn Extension) {
	faye.extns = append(faye.extns, extn)
}

// WaitSubscribe will send a subscribe request and block until the connection was successful
func (faye *FayeClient) WaitSubscribe(channel string, optionalMsgChan ...chan Message) {
	msgChan := make(chan Message)
	if len(optionalMsgChan) > 0 {
		msgChan = optionalMsgChan[0]
	}
	subscription := &Subscription{
		channel:     channel,
		msgChan:     msgChan,
		stopPolling: false,
	}

	for {
		if err := faye.requestSubscription(subscription); err != nil {
			faye.log.Errorf("requestSubscription error: %s", err)
			time.Sleep(1 * time.Second)
			continue
		}
		faye.log.Debugf("requestSubscription succeeded")
		break
	}

	faye.subscriptions = append(faye.subscriptions, subscription)
}

// resubscribe all of the subscriptions
func (faye *FayeClient) ResubscribeAll() {
	existingSubscriptions := faye.subscriptions
	faye.subscriptions = []*Subscription{}

	faye.log.Debugf("Attempting to resubscribe %d existing subscription(s)", len(existingSubscriptions))
	for _, existingSubscription := range existingSubscriptions {
		existingSubscription.stopPolling = false
		// fork off all the resubscribe requests
		go faye.resubscribe(existingSubscription)
	}
}

// Publish a message to the given channel
func (faye *FayeClient) Publish(channel string, data map[string]interface{}) error {
	msg := NewMessage(faye.clientID, channel)
	msg.Data = data
	response, _, err := faye.send(msg)
	if err != nil {
		return err
	}

	go faye.handleAdvice(response.Advice())

	if !response.OK() {
		return fmt.Errorf("Response was not successful")
	}

	return nil
}

func (faye *FayeClient) websocketReadPoll() {
	for {
		if err := faye.websocketRead(); err != nil {
			faye.log.Debugf("Got error from websocket, breaking: %s", err)
			break
		}
	}
	faye.transport.close()
	for _, subscription := range faye.subscriptions {
		// close(subscription.msgChan)
		subscription.stopPolling = true
	}
	faye.state = UNCONNECTED
	//faye.clientID = ""
}

func (faye *FayeClient) websocketRead() error {
	// faye.log.Debugf("Awaiting read from websocket")
	decoder, err := faye.transport.read()
	// faye.log.Debugf("Read from websocket")
	if err != nil {
		return StackError("transport.read", err)
	} else {
		r, m, err := decodeResponse(decoder)
		if err != nil {
			return StackError("decodeResponse", err)
		}
		/*
			if r.ClientID() != faye.clientID {
				// faye.log.Warnf("Ignoring message for channel %s with different clientID %s", r.Channel(), r.ClientID())
				// faye.requestUnsubscribe(r.Channel(), r.ClientID())
				return nil
			}
		*/
		go faye.handleAdvice(r.Advice())
		if len(m) > 0 {
			// faye.log.Debugf("Got %d messages", len(m))
			faye.handleMessages(m)
		}
	}
	// faye.log.Debugf("Done reading from websocket")
	return nil
}

func (faye *FayeClient) websocketPingPoll(subscription *Subscription) {
	subscription.polling = true
	for {
		if faye.state != CONNECTED {
			faye.log.Debugf("Faye was disconnected, stopping websocket ping polling")
			break
		}
		if subscription.stopPolling {
			faye.log.Debugf("Subscription stopPolling true, stopping websocket ping polling")
			break
		}
		if err := faye.websocketPing(subscription.channel); err != nil {
			faye.log.Errorf("%s", StackError("websocketPing", err))
			break
		}
		time.Sleep(time.Duration(WEBSOCKET_POLL_INITERVAL) * time.Second)
	}
	subscription.polling = false
}

func (faye *FayeClient) websocketPing(channel string) error {
	msg := NewMessage(faye.clientID, channel)
	msg.Data = map[string]any{"type": "ping"}
	if err := faye.sendOnly(msg); err != nil {
		return StackError("sendOnly", err)
	}
	return nil
}

func (faye *FayeClient) handshake() error {
	// uh oh spaghettios!
	if faye.state == DISCONNECTED {
		return fmt.Errorf("GTFO: Server told us not to reconnect :(")
	}

	// check if we need to wait before handshaking again
	if faye.nextHandshake > time.Now().Unix() {
		sleepFor := time.Now().Unix() - faye.nextHandshake

		// wait for the duration the server told us
		if sleepFor > 0 {
			faye.log.Debugf("Waiting for", sleepFor, "seconds before next handshake")
			time.Sleep(time.Duration(sleepFor) * time.Second)
		}
	}

	t, err := selectTransport(faye, MANDATORY_CONNECTION_TYPES)
	if err != nil {
		return fmt.Errorf("no usable transports available")
	}

	faye.mutex.Lock()
	faye.transport = t
	faye.transport.setURL(faye.url)
	faye.state = CONNECTING
	faye.mutex.Unlock()

	var response Response

	for {
		msg := NewMessage(faye.clientID, faye.handshakeChannel)
		msg.Version = "1.0"
		msg.SupportedConnectionTypes = []string{LONG_POLLING}
		response, _, err = faye.send(msg)

		if err != nil {
			faye.mutex.Lock()
			faye.state = UNCONNECTED
			faye.mutex.Unlock()

			faye.log.Warnf("Handshake failed. Retry in 10 seconds")
			time.Sleep(10 * time.Second)
			continue
		}
		faye.log.Debugf("Handshake successful")
		break
	}

	faye.mutex.Lock()
	oldClientID := faye.clientID
	faye.clientID = response.ClientID()
	faye.state = CONNECTED
	faye.transport, err = selectTransport(faye, response.SupportedConnectionTypes())
	faye.transport.setTimeoutSeconds(CONNECTION_TIMEOUT_SECONDS)
	faye.mutex.Unlock()

	if err != nil {
		return fmt.Errorf("Server does not support any available transports. Supported transports: " + strings.Join(response.SupportedConnectionTypes(), ","))
	}

	if oldClientID != faye.clientID && len(faye.subscriptions) > 0 {
		faye.log.Warnf("Client ID changed (%s => %s), %d invalid subscriptions", oldClientID, faye.clientID, len(faye.subscriptions))
		faye.ResubscribeAll()
	}

	return nil
}

func (faye *FayeClient) resubscribe(subscription *Subscription) {
	for {
		if err := faye.requestSubscription(subscription); err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		// if it worked add it back to the list
		faye.mutex.Lock()
		defer faye.mutex.Unlock()
		faye.subscriptions = append(faye.subscriptions, subscription)
		faye.log.Debugf("Resubscribed to %s", subscription.channel)
		return
	}
}

// requests a subscription from the server and returns error if the request failed
func (faye *FayeClient) requestSubscription(subscription *Subscription) error {
	msg := NewMessage(faye.clientID, faye.subscribeChannel)
	msg.Subscription = subscription.channel

	if faye.transport.connectionType() == WEBSOCKET {
		if err := faye.sendOnly(msg); err != nil {
			return StackError("sendOnly", err)
		}
		if !subscription.polling {
			faye.log.Debugf("Starting subscription polling for %s", subscription.channel)
			go faye.websocketPingPoll(subscription)
		} else {
			faye.log.Debugf("Skipping subscription polling for %s", subscription.channel)
		}
		return nil
	}
	// TODO: check if the protocol allows a subscribe during an active connect request
	response, _, err := faye.send(msg)
	if err != nil {
		return err
	}

	go faye.handleAdvice(response.Advice())

	if !response.OK() {
		// TODO: put more information in the error message about why it failed
		errmsg := "Response was unsuccessful: "

		if response.HasError() {
			errmsg += " / " + response.Error()
		}
		reserr := errors.New(errmsg)
		return reserr
	}

	return nil
}

/*
func (faye *FayeClient) requestUnsubscribe(channel string, clientId string) error {
	if err := faye.connectIfNotConnected(); err != nil {
		return err
	}

	msg := NewMessage(faye.clientID, faye.unsubscribeChannel)
	msg.ClientID = clientId
	msg.Subscription = channel

	if faye.transport.connectionType() == WEBSOCKET {
		if err := faye.sendOnly(msg); err != nil {
			return err
		}
		return nil
	}

	response, _, err := faye.send(msg)
	if err != nil {
		return err
	}

	go faye.handleAdvice(response.Advice())

	if !response.OK() {
		// TODO: put more information in the error message about why it failed
		errmsg := "Response was unsuccessful: "

		if response.HasError() {
			errmsg += " / " + response.Error()
		}
		reserr := errors.New(errmsg)
		return reserr
	}
	return nil
}
*/

// handles a response from the server
func (faye *FayeClient) handleMessages(msgs []Message) {
	for _, message := range msgs {
		faye.runExtensions("in", message)
		for _, subscription := range faye.subscriptions {
			matched, _ := filepath.Match(subscription.channel, message.Channel())
			if matched {
				go func() { subscription.msgChan <- message }()
				return
			}
		}
		faye.log.Warnf("Unable to find subscription for channel %s", message.Channel())
	}
}

// handles advice from the server
func (faye *FayeClient) handleAdvice(advice Advice) {
	faye.mutex.Lock()
	defer faye.mutex.Unlock()

	if advice.Reconnect() != "" {
		interval := advice.Interval()

		switch advice.Reconnect() {
		case RETRY:
			if err := faye.connect(); err != nil {
				faye.log.Errorf("Error connecting while handling advice: %s", StackError("connect", err))
			}
		case HANDSHAKE:
			faye.state = UNCONNECTED // force a handshake on the next request
			if interval > 0 {
				faye.nextHandshake = int64(time.Duration(time.Now().Unix()) + (time.Duration(interval) * time.Millisecond))
			}
		case NONE:
			faye.state = DISCONNECTED
			faye.log.Errorf("GTFO: Server advised not to reconnect :(")
		}
	}
}

// Connects to the server. Waits for a response if HTTPTransport
func (faye *FayeClient) connect() error {
	msg := NewMessage(faye.clientID, faye.connectChannel)
	msg.ConnectionType = faye.transport.connectionType()

	if msg.ConnectionType == WEBSOCKET {
		return faye.sendOnly(msg)
	}

	response, messages, err := faye.send(msg)
	if err != nil {
		faye.log.Errorf("Error while sending connect request: %s", err)
		return err
	}

	go faye.handleAdvice(response.Advice())

	if response.OK() {
		go faye.handleMessages(messages)
	} else {
		faye.log.Errorf("Error in response to connect request: %s", response.Error())
		return errors.New(response.Error())
	}
	return nil
}

func (faye *FayeClient) setupSend(msg *message) (Message, error) {
	if msg.ClientID == "" && msg.Channel != faye.handshakeChannel && faye.clientID != "" {
		msg.ClientID = faye.clientID
	}

	msg.ID = strconv.Itoa(faye.message_id)
	faye.message_id = faye.message_id + 1
	message := Message(msgWrapper{msg})
	faye.runExtensions("out", message)

	if message.HasError() {
		return nil, message // Message has Error() so can be returned as an error
	}
	return message, nil
}

// wraps the call to transport.send()
func (faye *FayeClient) send(msg *message) (Response, []Message, error) {
	message, err := faye.setupSend(msg)
	if err != nil || message.HasError() {
		return nil, []Message{}, err // Message has Error() so can be returned as an error
	}

	dec, err := faye.transport.send(message)
	if err != nil {
		err = StackError("transport.send", err)
		faye.log.Errorf("%s", err)
		return nil, []Message{}, err
	}

	r, m, err := decodeResponse(dec)
	if err != nil {
		faye.log.Errorf("Failed to decode response: %s", err)
	}

	if r != nil {
		faye.runExtensions("in", r.(Message))
	}

	return r, m, err
}

func (faye *FayeClient) sendOnly(msg *message) error {
	if message, err := faye.setupSend(msg); err != nil {
		return StackError("setupSend", err)
	} else {
		if err = faye.transport.sendOnly(message); err != nil {
			return StackError("transport.sendOnly", err)
		}
	}
	return nil
}

func (faye *FayeClient) runExtensions(direction string, msg Message) {
	for _, extn := range faye.extns {
		// faye.log.Debugf("Running extension %T %s", extn, direction)
		switch direction {
		case "out":
			extn.Out(msg)
		case "in":
			extn.In(msg)
		}
	}
}
