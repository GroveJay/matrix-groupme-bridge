package groupmerealtime

import (
	"github.com/GroveJay/matrix-groupme-bridge/pkg/faye"
)

type AuthExt struct {
	token string
}

var _ faye.Extension = (*AuthExt)(nil)

func (a *AuthExt) In(m faye.Message) {}

func (a *AuthExt) Out(msg faye.Message) {
	if msg.Channel() == subscribeChannel || msg.Data()["type"] == "ping" {
		ext := msg.Ext()
		ext["access_token"] = a.token
		// ext["timestamp"] = time.Now().Unix()
	}
}
