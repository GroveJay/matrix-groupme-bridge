package faye

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"io"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type WebsocketTransport struct {
	url             string
	connection      *websocket.Conn
	timeoutDuration time.Duration
}

func (wt *WebsocketTransport) isUsable(clientURL string) bool {
	wt.setURL(clientURL)
	if wt.connection != nil {
		if err := wt.connection.CloseNow(); err != nil {
			log.Printf("WRN CloseNow error: %s", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wt.url, nil)
	if err != nil {
		return false
	}
	wt.connection = c
	return true
}

func (wt WebsocketTransport) close() {
	defer wt.connection.CloseNow()
}

func (wt WebsocketTransport) connectionType() string {
	return "websocket"
}

func (wt WebsocketTransport) sendOnly(msg json.Marshaler) error {
	ctx, cancel := context.WithTimeout(context.Background(), wt.timeoutDuration)
	defer cancel()

	if err := wsjson.Write(ctx, wt.connection, msg); err != nil {
		return err
	}
	return nil
}

func (wt WebsocketTransport) send(msg json.Marshaler) (decoder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wt.timeoutDuration)
	defer cancel()

	err := wsjson.Write(ctx, wt.connection, msg)
	if err != nil {
		return nil, err
	}
	_, r, err := wt.connection.Reader(ctx)
	if err != nil {
		return nil, err
	}
	jsonData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return json.NewDecoder(bytes.NewBuffer(jsonData)), nil
}

func (wt WebsocketTransport) read() (decoder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wt.timeoutDuration)
	defer cancel()

	_, r, err := wt.connection.Reader(ctx)
	if err != nil {
		return nil, err
	}
	jsonData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return json.NewDecoder(bytes.NewBuffer(jsonData)), nil
}

func (wt *WebsocketTransport) setURL(url string) {
	wt.url = "ws://" + url
}

func (wt *WebsocketTransport) setTimeoutSeconds(timeoutSeconds int64) {
	wt.timeoutDuration = time.Second * time.Duration(timeoutSeconds)
}
