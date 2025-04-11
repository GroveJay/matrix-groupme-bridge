package connector

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmeclient"
	"github.com/GroveJay/matrix-groupme-bridge/pkg/util"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"
)

func MakeGroupmePortalId(group groupmeclient.ID, userLoginId networkid.UserLoginID) networkid.PortalID {
	return networkid.PortalID(fmt.Sprintf("%s:%s:%s", "GroupmeID", userLoginId, group.String()))
}

func ParsePortalId(portalID networkid.PortalID) (*groupmeclient.ID, *networkid.UserLoginID, error) {
	parts := strings.Split(string(portalID), ":")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("could not parse portalID (%s)", portalID)
	}
	userLoginID := networkid.UserLoginID(parts[1])
	groupmeID := groupmeclient.ID(parts[2])
	return &groupmeID, &userLoginID, nil
}

func wrapAvatar(avatarURL string) *bridgev2.Avatar {
	if avatarURL == "" {
		return &bridgev2.Avatar{Remove: true}
	}
	parsedURL, _ := url.Parse(avatarURL)
	avatarID := path.Base(parsedURL.Path)
	return &bridgev2.Avatar{
		ID: networkid.AvatarID(avatarID),
		Get: func(ctx context.Context) ([]byte, error) {
			_, resp, err := util.DownloadMedia(ctx, "image/*", avatarURL, 5*1024*1024, "", false)
			if err != nil {
				return nil, err
			}
			return io.ReadAll(resp)
		},
	}
}

func GroupLogContext(group groupmeclient.ID) func(c zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str("groupmeID", group.String())
	}
}

func (groupmeClient *GroupmeClient) SendSimpleEventChatInfoChange(group groupmeclient.ID, logContext func(c zerolog.Context) zerolog.Context, chatInfoChange *bridgev2.ChatInfoChange) {
	groupmeClient.UserLogin.Bridge.QueueRemoteEvent(groupmeClient.UserLogin, &simplevent.ChatInfoChange{
		EventMeta: simplevent.EventMeta{
			Type:       bridgev2.RemoteEventChatInfoChange,
			LogContext: logContext,
			PortalKey: networkid.PortalKey{
				ID:       MakeGroupmePortalId(group, groupmeClient.UserLogin.UserLogin.ID),
				Receiver: groupmeClient.UserLogin.ID,
			},
			CreatePortal: true,
			Timestamp:    time.Now(),
		},
		ChatInfoChange: chatInfoChange,
	})
}

func (groupmeClient *GroupmeClient) HandleError(err error) {
	groupmeClient.UserLogin.Log.Error().Msgf("HandleError (error: %s)", err)
}

func (groupmeClient *GroupmeClient) HandleGroupAvatar(group groupmeclient.ID, newAvatar string) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleGroupAvatar (groupID: %s, newAvatar: %s)", group, newAvatar)
	groupmeClient.SendSimpleEventChatInfoChange(
		group,
		GroupLogContext(group),
		&bridgev2.ChatInfoChange{ChatInfo: &bridgev2.ChatInfo{Avatar: wrapAvatar(newAvatar)}})
}

func (groupmeClient *GroupmeClient) HandleGroupName(group groupmeclient.ID, newName string) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleGroupName (groupID: %s, newName: %s)", group, newName)
	groupmeClient.SendSimpleEventChatInfoChange(
		group,
		GroupLogContext(group),
		&bridgev2.ChatInfoChange{ChatInfo: &bridgev2.ChatInfo{Name: &newName}})
}

func (groupmeClient *GroupmeClient) HandleGroupTopic(group groupmeclient.ID, newTopic string) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleGroupTopic (groupID: %s, newTopic: %s)", group, newTopic)
	groupmeClient.SendSimpleEventChatInfoChange(
		group,
		GroupLogContext(group),
		&bridgev2.ChatInfoChange{ChatInfo: &bridgev2.ChatInfo{Topic: &newTopic}})
}

func (groupmeClient *GroupmeClient) HandleJoin(group groupmeclient.ID) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleJoin (groupID: %s)", group)
	groupmeClient.SendSimpleEventChatInfoChange(
		group,
		GroupLogContext(group),
		&bridgev2.ChatInfoChange{ChatInfo: &bridgev2.ChatInfo{}})
}

func (groupmeClient *GroupmeClient) HandleLike(message groupmeclient.Message) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleLike (groupID: %s, MessageID: %s, senderID: %s)", message.GroupID, message.ID, message.SenderID)
	// TODO: If a portal has an emoji associated with it, use that one instead
	emoji := "❤️"
	groupmeClient.UserLogin.Bridge.QueueRemoteEvent(groupmeClient.UserLogin, &simplevent.Reaction{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventReaction,
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.
					Str("groupmeID", message.GroupID.String())
			},
			PortalKey: networkid.PortalKey{
				ID:       MakeGroupmePortalId(message.GroupID, groupmeClient.UserLogin.UserLogin.ID),
				Receiver: groupmeClient.UserLogin.ID,
			},
			CreatePortal: true,
			Timestamp:    time.Now(),
			Sender: bridgev2.EventSender{
				SenderLogin: networkid.UserLoginID(message.SenderID),
			},
		},
		TargetMessage: networkid.MessageID(message.ID),
		Emoji:         emoji,
		EmojiID:       networkid.EmojiID(emoji),
	})
}

func (groupmeClient *GroupmeClient) HandleLikeIcon(group groupmeclient.ID, PackID int, PackIndex int, Type string) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleLikeIkon (groupID: %s, PackID: %d, PackIndex: %d, Type: %s)", group, PackID, PackIndex, Type)
	// TODO: Figure out how to assign a like-icon to a portal
}

func (groupmeClient *GroupmeClient) HandleMembers(group groupmeclient.ID, members []groupmeclient.Member, added bool) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleMembers (groupID: %s, members(len): %d, added: %t)", group, len(members), added)
	memberChanges := &bridgev2.ChatMemberList{}
	membersToUpdate := []groupmeclient.Member{}
	if added {
		membersToUpdate = members
	} else {
		memberChanges.IsFull = true
		ctx := context.Context(context.Background())
		group, err := groupmeClient.Client.ShowGroup(ctx, group)
		if err != nil {
			zerolog.Ctx(context.Background()).Error().Str("groupId", group.ID.String()).Msgf("Error getting group information: %s", err)
			return
		}
		for _, member := range group.Members {
			membersToUpdate = append(membersToUpdate, *member)
		}
	}

	memberChanges.MemberMap = make(map[networkid.UserID]bridgev2.ChatMember, len(membersToUpdate))
	for _, member := range membersToUpdate {
		if _, alreadyExists := memberChanges.MemberMap[networkid.UserID(member.UserID)]; alreadyExists {
			zerolog.Ctx(context.Background()).Warn().Str("userId", member.UserID.String()).Msg("Duplicate member in list")
		}
		memberChanges.MemberMap[networkid.UserID(member.UserID)] = bridgev2.ChatMember{
			Nickname: &member.Nickname,
			UserInfo: &bridgev2.UserInfo{
				Name:   &member.Nickname,
				Avatar: wrapAvatar(member.ImageURL),
			},
		}
	}
	groupmeClient.SendSimpleEventChatInfoChange(
		group,
		GroupLogContext(group),
		&bridgev2.ChatInfoChange{
			MemberChanges: memberChanges,
		})
}

func (groupmeClient *GroupmeClient) HandleNewAvatarInGroup(group groupmeclient.ID, user groupmeclient.ID, avatarURL string) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleNewAvatar (groupID: %s, userID: %s, newName: %s)", group, user, avatarURL)
	groupmeClient.SendSimpleEventChatInfoChange(
		group,
		func(c zerolog.Context) zerolog.Context {
			return c.
				Str("groupmeID", group.String()).
				Str("userId", user.String())
		},
		&bridgev2.ChatInfoChange{
			MemberChanges: &bridgev2.ChatMemberList{
				MemberMap: map[networkid.UserID]bridgev2.ChatMember{
					networkid.UserID(user.String()): {
						UserInfo: &bridgev2.UserInfo{
							Avatar: wrapAvatar(avatarURL),
						},
					},
				},
			},
		})
}

func (groupmeClient *GroupmeClient) HandleNewNickname(group groupmeclient.ID, user groupmeclient.ID, newName string) {
	groupmeClient.UserLogin.Log.Debug().Msgf("HandleNewNickname (groupID: %s, userID: %s, newName: %s)", group, user, newName)
	groupmeClient.SendSimpleEventChatInfoChange(
		group,
		func(c zerolog.Context) zerolog.Context {
			return c.
				Str("groupmeID", group.String()).
				Str("userId", user.String())
		},
		&bridgev2.ChatInfoChange{
			MemberChanges: &bridgev2.ChatMemberList{
				MemberMap: map[networkid.UserID]bridgev2.ChatMember{
					networkid.UserID(user.String()): {
						UserInfo: &bridgev2.UserInfo{
							Name: &newName,
						},
					},
				},
			},
		})
}

func (groupmeClient *GroupmeClient) HandleTextMessage(message groupmeclient.Message) {
	groupmeClient.UserLogin.Log.Debug().Msg("HandleTextMessage")
	isFromMe := false
	if message.SenderID == groupmeClient.userId {
		isFromMe = true
	}
	groupmeClient.UserLogin.Bridge.QueueRemoteEvent(groupmeClient.UserLogin, &simplevent.Message[groupmeclient.Message]{
		EventMeta: simplevent.EventMeta{
			Sender: bridgev2.EventSender{
				Sender:   networkid.UserID(message.SenderID),
				IsFromMe: isFromMe,
			},
			Type: bridgev2.RemoteEventMessage,
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.
					Str("groupmeID", message.GroupID.String()).
					Str("userId", message.UserID.String())
			},
			PortalKey: networkid.PortalKey{
				ID:       MakeGroupmePortalId(message.GroupID, groupmeClient.UserLogin.UserLogin.ID),
				Receiver: groupmeClient.UserLogin.ID,
			},
			CreatePortal: true,
			Timestamp:    time.Now(),
		},
		Data:               message,
		ID:                 networkid.MessageID(message.ID),
		ConvertMessageFunc: groupmeClient.convertMessage,
	})
}

func (groupmeClient *GroupmeClient) convertMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, data groupmeclient.Message) (*bridgev2.ConvertedMessage, error) {
	convertedMessage := &bridgev2.ConvertedMessage{}
	parts := []*bridgev2.ConvertedMessagePart{}
	if data.Text != "" {
		parts = append(parts, &bridgev2.ConvertedMessagePart{
			Type: event.EventMessage,
			Content: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    data.Text,
			},
		})
	}
	if len(data.Attachments) > 0 {
		for _, attachment := range data.Attachments {
			if attachment.Type == groupmeclient.Reply {
				convertedMessage.ReplyTo = &networkid.MessageOptionalPartID{
					MessageID: networkid.MessageID(attachment.ReplyID),
				}
				continue
			}
			if content, err := util.ConvertAttachment(ctx, attachment, intent, portal.MXID); err != nil {
				parts = append(parts, util.ErrorToNotice(err, string(attachment.Type)))
				continue
			} else {
				parts = append(parts, &bridgev2.ConvertedMessagePart{
					Type:    event.EventMessage,
					Content: content,
				})
			}
		}
	}

	convertedMessage.Parts = parts
	// GroupMe doesn't do this so lets not
	// convertedMessage.MergeCaption()

	for i, part := range convertedMessage.Parts {
		part.ID = networkid.PartID(strconv.Itoa(i))
	}

	return convertedMessage, nil
}
