package connector

import (
	"context"

	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmerealtime"
	"go.mau.fi/util/configupgrade"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
)

type GroupmeConnector struct {
	br *bridgev2.Bridge
}

var _ bridgev2.NetworkConnector = (*GroupmeConnector)(nil)

func (gc *GroupmeConnector) Init(bridge *bridgev2.Bridge) {
	gc.br = bridge
}

func (gc *GroupmeConnector) Start(ctx context.Context) error {
	gc.br.Log.Info().Msg("Start")
	return nil
}

func (gc *GroupmeConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{}
}

func (gc *GroupmeConnector) GetBridgeInfoVersion() (info int, capabilities int) {
	return 1, 1
}

func (gc *GroupmeConnector) GetName() bridgev2.BridgeName {
	return bridgev2.BridgeName{
		DisplayName:      "Groupme",
		NetworkURL:       "https://groupme.com",
		NetworkIcon:      "mxc://maunium.net/FYuKJHaCrSeSpvBJfHwgYylP",
		NetworkID:        "groupme",
		BeeperBridgeType: "jrgrover.com/matrix-groupme-bridge",
		DefaultPort:      29322,
	}
}

func (gc *GroupmeConnector) GetConfig() (example string, data any, upgrader configupgrade.Upgrader) {
	return "", nil, configupgrade.NoopUpgrader
}

func (gc *GroupmeConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Portal:   nil,
		Ghost:    nil,
		Message:  nil,
		Reaction: nil,
		UserLogin: func() any {
			return &UserLoginMetadata{}
		},
	}
}

type UserLoginMetadata struct {
	AuthToken string `json:"authToken"`
}

func (gc *GroupmeConnector) LoadUserLogin(ctx context.Context, login *bridgev2.UserLogin) error {
	login.Log.Info().Msgf("GroupmeConnector.LoadUserLogin")
	meta := login.Metadata.(*UserLoginMetadata)
	pushSubscription := groupmerealtime.NewPushSubscription(ctx)
	login.Log.Info().Msgf("GroupmeConnector.LoadUserLogin meta: %s", meta)
	login.Client = &GroupmeClient{
		UserLogin:        login,
		PushSubscription: &pushSubscription,
		AuthToken:        meta.AuthToken,
	}
	return nil
}
