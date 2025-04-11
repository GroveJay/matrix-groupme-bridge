package faye

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// HTTPTransport models a faye protocol transport over HTTP long polling
type HTTPTransport struct {
	url string
}

func (t HTTPTransport) isUsable(clientURL string) bool {
	_, err := url.Parse("https://" + clientURL)
	return err == nil
}

func (t HTTPTransport) connectionType() string {
	return "long-polling"
}

func (t HTTPTransport) close() {
}

func (t HTTPTransport) sendOnly(msg json.Marshaler) error {
	panic("not implemented!")
}

func (t HTTPTransport) send(msg json.Marshaler) (decoder, error) {
	b, err := json.Marshal(msg)

	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBuffer(b)
	responseData, err := http.Post(t.url, "application/json", buffer)
	if err != nil {
		return nil, err
	}
	if responseData.StatusCode != 200 {
		return nil, errors.New(responseData.Status)
	}
	defer responseData.Body.Close()
	jsonData, err := io.ReadAll(responseData.Body)
	if err != nil {
		return nil, err
	}
	return json.NewDecoder(bytes.NewBuffer(jsonData)), nil
}

func (t HTTPTransport) read() (decoder, error) {
	panic("not implemented!")
}

func (t *HTTPTransport) setURL(url string) {
	t.url = "https://" + url
}

func (t *HTTPTransport) setTimeoutSeconds(timeout int64) {

}
