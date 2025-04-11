package main

import (
	"github.com/GroveJay/matrix-groupme-bridge/pkg/connector"
	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"
)

var (
	Tag       = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	m := mxmain.BridgeMain{
		Name:        "jrgrover-groupme",
		Description: "A Groupme matrix bridge",
		URL:         "",
		Version:     "0.1.0",
		Connector:   &connector.GroupmeConnector{},
	}
	m.InitVersion(Tag, Commit, BuildTime)
	m.Run()
}
