package main

import (
	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmeclient"
	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmerealtime"
	"github.com/rs/zerolog"
)

type gha struct {
	logger zerolog.Logger
}

// HandleError implements groupmeclient.HandlerAll.
func (g *gha) HandleError(err error) {
	g.logger.Error().Msgf("HandleError (error: %s)", err)
}

// HandleGroupAvatar implements groupmeclient.HandlerAll.
func (g *gha) HandleGroupAvatar(group groupmeclient.ID, newAvatar string) {
	g.logger.Debug().Msgf("HandleGroupAvatar (groupID: %s, newAvatar: %s)", group, newAvatar)
}

// HandleGroupName implements groupmeclient.HandlerAll.
func (g *gha) HandleGroupName(group groupmeclient.ID, newName string) {
	g.logger.Debug().Msgf("HandleGroupName (groupID: %s, newName: %s)", group, newName)
}

// HandleGroupTopic implements groupmeclient.HandlerAll.
func (g *gha) HandleGroupTopic(group groupmeclient.ID, newTopic string) {
	g.logger.Debug().Msgf("HandleGroupTopic (groupID: %s, newTopic: %s)", group, newTopic)
}

// HandleJoin implements groupmeclient.HandlerAll.
func (g *gha) HandleJoin(group groupmeclient.ID) {
	g.logger.Debug().Msgf("HandleJoin (groupID: %s)", group)
}

// HandleLike implements groupmeclient.HandlerAll.
func (g *gha) HandleLike(message groupmeclient.Message) {
	g.logger.Debug().Msgf("HandleLike (groupID: %s, MessageID: %s, senderID: %s)", message.GroupID, message.ID, message.SenderID)
}

// HandleLikeIcon implements groupmeclient.HandlerAll.
func (g *gha) HandleLikeIcon(group groupmeclient.ID, PackID int, PackIndex int, Type string) {
	g.logger.Debug().Msgf("HandleLikeIkon (groupID: %s, PackID: %d, PackIndex: %d, Type: %s)", group, PackID, PackIndex, Type)
}

// HandleMembers implements groupmeclient.HandlerAll.
func (g *gha) HandleMembers(group groupmeclient.ID, members []groupmeclient.Member, added bool) {
	g.logger.Debug().Msgf("HandleMembers (groupID: %s, members(len): %d, added: %t)", group, len(members), added)
}

// HandleNewAvatarInGroup implements groupmeclient.HandlerAll.
func (g *gha) HandleNewAvatarInGroup(group groupmeclient.ID, user groupmeclient.ID, avatarURL string) {
	g.logger.Debug().Msgf("HandleNewAvatarInGroup (groupID: %s, userID: %s, newName: %s)", group, user, avatarURL)
}

// HandleNewNickname implements groupmeclient.HandlerAll.
func (g *gha) HandleNewNickname(group groupmeclient.ID, user groupmeclient.ID, newName string) {
	g.logger.Debug().Msgf("HandleNewNickname (groupID: %s, userID: %s, newName: %s)", group, user, newName)
}

// HandleTextMessage implements groupmeclient.HandlerAll.
func (g *gha) HandleTextMessage(groupmeclient.Message) {
	g.logger.Debug().Msg("HandleTextMessage")
}

var _ groupmerealtime.HandlerAll = (*gha)(nil)
