package connector

import (
	"context"
	"fmt"

	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmeclient"
	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmerealtime"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

type GroupmeLogin struct {
	User      *bridgev2.User
	Connector *GroupmeConnector
	Client    *groupmeclient.Client
	AuthToken string
	UserId    groupmeclient.ID
}

var _ bridgev2.LoginProcessUserInput = (*GroupmeLogin)(nil)

func (g *GroupmeConnector) CreateLogin(ctx context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	if flowID != "auth-token" {
		return nil, fmt.Errorf("unknown login flow ID: %s", flowID)
	}
	return &GroupmeLogin{User: user}, nil
}

func (g *GroupmeConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return []bridgev2.LoginFlow{{
		Name:        "Auth token",
		Description: "Log in with your Groupme access token from https://dev.groupmeclient.com/",
		ID:          "auth-token",
	}}
}

func (gl *GroupmeLogin) Cancel() {
	// TODO: Any teardown required?
}

func (gl *GroupmeLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       "groupmeclient.enter_api_key",
		Instructions: "",
		UserInputParams: &bridgev2.LoginUserInputParams{
			Fields: []bridgev2.LoginInputDataField{
				{
					Type:    bridgev2.LoginInputFieldTypePassword,
					ID:      "auth_token",
					Name:    "Groupme auth token",
					Pattern: "^[0-9a-zA-Z]{40}$",
				},
			},
		},
	}, nil

}

func (gl *GroupmeLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
	gl.AuthToken = input["auth_token"]
	pushSubscription := groupmerealtime.NewPushSubscription(ctx)
	userLogin, err := gl.User.NewLogin(ctx, &database.UserLogin{
		ID:         networkid.UserLoginID(gl.UserId),
		RemoteName: gl.UserId.String(),
		Metadata: &UserLoginMetadata{
			AuthToken: gl.AuthToken,
		},
	}, &bridgev2.NewLoginParams{
		LoadUserLogin: func(ctx context.Context, login *bridgev2.UserLogin) error {
			login.Client = &GroupmeClient{
				UserLogin:        login,
				PushSubscription: &pushSubscription,
				AuthToken:        gl.AuthToken,
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeComplete,
		StepID:       "groupmeclient.complete",
		Instructions: "Successfully logged in",
		CompleteParams: &bridgev2.LoginCompleteParams{
			UserLogin: userLogin,
		},
	}, nil
}
