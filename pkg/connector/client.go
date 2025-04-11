package connector

import (
	"context"
	"fmt"

	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmeclient"
	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmerealtime"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/status"
	"maunium.net/go/mautrix/event"
)

type GroupmeClient struct {
	UserLogin        *bridgev2.UserLogin
	PushSubscription *groupmerealtime.PushSubscription
	Client           *groupmeclient.Client
	AuthToken        string
	userId           groupmeclient.ID
}

var _ bridgev2.NetworkAPI = (*GroupmeClient)(nil)

func (groupmeClient *GroupmeClient) Connect(ctx context.Context) {
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.Connect")
	if groupmeClient.AuthToken == "" {
		groupmeClient.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "groupme-connect-authtoken",
			Message:    "Failed to find existing authtoken",
			Info:       map[string]any{},
		})
		return
	}

	groupmeClient.Client = groupmeclient.NewClient(groupmeClient.AuthToken)
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.Connect: NewClient created")

	if user, err := groupmeClient.Client.MyUser(ctx); err != nil {
		groupmeClient.UserLogin.Log.Error().Msg("Getting user information failed!")
		groupmeClient.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "groupme-get-user",
			Message:    "Failed to get user information",
			Info: map[string]any{
				"go_error": err.Error(),
			},
		})
		return
	} else {
		groupmeClient.userId = user.ID
	}

	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.Connect: got UserId")
	groupmeClient.PushSubscription.AddFullHandler(groupmeClient)
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.Connect: added handler")
	fayeZeroLogger := &groupmerealtime.FayeZeroLogger{Logger: groupmeClient.UserLogin.Log}
	if err := groupmeClient.PushSubscription.Setup(context.Background(), *groupmerealtime.NewFayeClient(*fayeZeroLogger, groupmeClient.AuthToken)); err != nil {
		groupmeClient.UserLogin.Log.Error().Msg("Setting up PushSubscription failed!")
		groupmeClient.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "groupme-setup-pushsubscription",
			Message:    "Failed to setup PushSubscription",
			Info: map[string]any{
				"go_error": err.Error(),
			},
		})
		return
	}
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.Connect: Setup succeeded")

	if err := groupmeClient.PushSubscription.SubscribeToUser(ctx, groupmeClient.userId); err != nil {
		groupmeClient.UserLogin.Log.Error().Msg("Subscription failed!")
		groupmeClient.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "groupme-subscribe-to-user",
			Message:    "Failed to subscribe to user",
			Info: map[string]any{
				"go_error": err.Error(),
			},
		})
		return
	}
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.Connect: SubscribeToUser succeeded")
	// TODO: Subscribe to all DMs and Groups
	// This seems like overkill to get real-time websocket notifications for:
	//  - Favorites / Likes / Unlikes / Like Icon Change
	//  - Typing indicators
	//  - Reactions to other's messages
	//  - Membership / role updates for other users
	// The alternative is to occasionally poll over all chats and update all messages, Members, etc
}

func (groupmeClient *GroupmeClient) Disconnect() {
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.Disconnect")
	groupmeClient.Client.Close()
}

func (groupmeClient *GroupmeClient) IsLoggedIn() bool {
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.IsLoggedIn")
	return true // groupmeClient.PushSubscription.Connected()?
}

func (groupmeClient *GroupmeClient) LogoutRemote(ctx context.Context) {
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.LogoutRemote")
	groupmeClient.Client.Close()
}

func (groupmeClient *GroupmeClient) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *event.RoomFeatures {
	groupmeClient.UserLogin.Log.Info().Msg("GroupmeClient.GetCapabilities")
	// TODO: Handle
	return &event.RoomFeatures{}
}

func (groupmeClient *GroupmeClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	groupmeClient.UserLogin.Log.Info().Msgf("GroupmeClient.IsThisUser: userID %s", userID)
	return networkid.UserID(groupmeClient.userId) == userID
}

func (groupmeClient *GroupmeClient) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	groupmeClient.UserLogin.Log.Info().Msgf("GroupmeClient.GetChatInfo: portal %s", portal.ID)
	groupID, _, err := ParsePortalId(portal.ID)
	if err != nil {
		return nil, err
	}
	group, err := groupmeClient.Client.ShowGroup(ctx, *groupID)
	if err != nil {
		groupmeClient.UserLogin.Log.Error().Msgf("GroupmeClient.GetChatInfo: Failed to get group information for groupID %s", groupID)
		return nil, err
	}
	members := &bridgev2.ChatMemberList{
		IsFull:    true,
		MemberMap: make(map[networkid.UserID]bridgev2.ChatMember, len(group.Members)),
	}
	for _, member := range group.Members {
		membership := event.MembershipJoin
		if member.AutoKicked {
			membership = event.MembershipBan
		}
		members.MemberMap[networkid.UserID(member.UserID)] = bridgev2.ChatMember{
			Nickname:   &member.Nickname,
			Membership: membership,
			UserInfo: &bridgev2.UserInfo{
				Avatar: wrapAvatar(member.ImageURL),
			},
		}
	}
	return &bridgev2.ChatInfo{
		Name:    &group.Name,
		Topic:   &group.Description,
		Avatar:  wrapAvatar(group.ImageURL),
		Members: members,
	}, nil
}

func (groupmeClient *GroupmeClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	groupmeClient.UserLogin.Log.Info().Msgf("GroupmeClient.GetUserInfo: ghostID %s", ghost.ID)
	relations, err := groupmeClient.Client.IndexAllRelations(ctx)
	if err != nil {
		return nil, err
	}
	var matchingRelation *groupmeclient.User
	for _, relation := range relations {
		if relation.ID == groupmeclient.ID(ghost.ID) {
			matchingRelation = relation
			break
		}
	}
	if matchingRelation == nil {
		groupmeClient.UserLogin.Log.Error().Msgf("GroupmeClient.GetUserInfo: unable to find user with ghostID %s", ghost.ID)
		return nil, fmt.Errorf("unable to find user with id: %s", ghost.ID)
	}
	return &bridgev2.UserInfo{
		Identifiers: []string{
			matchingRelation.Name,
			matchingRelation.PhoneNumber.String(),
			matchingRelation.Email,
		},
		Name:   &matchingRelation.Name,
		Avatar: wrapAvatar(matchingRelation.AvatarURL),
	}, nil
}

func (g *GroupmeClient) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (message *bridgev2.MatrixMessageResponse, err error) {
	groupmeclientID, _, err := ParsePortalId(msg.Portal.ID)
	if err != nil {
		return nil, err
	}
	groupmemessage, err := g.Client.CreateMessage(ctx, *groupmeclientID, &groupmeclient.Message{
		Text: msg.Content.Body,
		// TODO: Add attachments, emojis, etc
		// GetCapabilities() will need updating after so messages don't get rejected
	})
	if err != nil {
		return nil, err
	}
	return &bridgev2.MatrixMessageResponse{
		DB: &database.Message{
			ID:       networkid.MessageID(groupmemessage.ID),
			SenderID: networkid.UserID(groupmemessage.SenderID),
		},
	}, nil
}
