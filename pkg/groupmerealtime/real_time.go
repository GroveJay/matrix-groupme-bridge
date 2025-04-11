package groupmerealtime

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/GroveJay/matrix-groupme-bridge/pkg/faye"
	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmeclient"
)

const (
	PushServer         = "push.groupme.com/faye"
	userChannel        = "/user/"
	groupChannel       = "/group/"
	dmChannel          = "/direct_message/"
	handshakeChannel   = "/meta/handshake"
	connectChannel     = "/meta/connect"
	subscribeChannel   = "/meta/subscribe"
	unsubscribeChannel = "/meta/unsubscribe"
)

var (
	ErrHandlerNotFound    = errors.New("Handler not found")
	ErrListenerNotStarted = errors.New("GroupMe listener not started")
)

var concur = sync.Mutex{}

func init() {
	faye.RegisterTransports([]faye.Transport{
		&faye.WebsocketTransport{},
		&faye.HTTPTransport{},
	})
}

type HandlerAll interface {
	Handler

	//of self
	HandlerText
	HandlerLike
	HandlerMembership

	//of group
	HandleGroupTopic
	HandleGroupAvatar
	HandleGroupName
	HandleGroupLikeIcon

	//of group members
	HandleMemberNewNickname
	HandleMemberNewAvatar
	HandleMembers
}
type Handler interface {
	HandleError(error)
}
type HandlerText interface {
	HandleTextMessage(groupmeclient.Message)
}
type HandlerLike interface {
	HandleLike(groupmeclient.Message)
}
type HandlerMembership interface {
	HandleJoin(groupmeclient.ID)
}

// Group Handlers
type HandleGroupTopic interface {
	HandleGroupTopic(group groupmeclient.ID, newTopic string)
}

type HandleGroupName interface {
	HandleGroupName(group groupmeclient.ID, newName string)
}
type HandleGroupAvatar interface {
	HandleGroupAvatar(group groupmeclient.ID, newAvatar string)
}
type HandleGroupLikeIcon interface {
	HandleLikeIcon(group groupmeclient.ID, PackID, PackIndex int, Type string)
}

// Group member handlers
type HandleMemberNewNickname interface {
	HandleNewNickname(group groupmeclient.ID, user groupmeclient.ID, newName string)
}

type HandleMemberNewAvatar interface {
	HandleNewAvatarInGroup(group groupmeclient.ID, user groupmeclient.ID, avatarURL string)
}
type HandleMembers interface {
	//HandleNewMembers returns only partial member with id and nickname; added is false if removing
	HandleMembers(group groupmeclient.ID, members []groupmeclient.Member, added bool)
}

type PushMessage interface {
	Channel() string
	Data() map[string]interface{}
	Ext() map[string]interface{}
	Error() string
}

// PushSubscription manages real time subscription
type PushSubscription struct {
	channel           chan PushMessage
	fayeClient        *faye.FayeClient
	handlers          []Handler
	connectionTimeout int64
	timeoutMinutes    int64
}

func NewFayeClient(logger faye.Logger, authToken string) *faye.FayeClient {
	fc := faye.NewFayeClient(
		PushServer,
		handshakeChannel,
		connectChannel,
		subscribeChannel,
		unsubscribeChannel)
	fc.SetLogger(logger)
	fc.AddExtension(&AuthExt{
		token: authToken,
	})
	fc.AddExtension(fc)

	return fc
}

// NewPushSubscription creates and returns a push subscription object
func NewPushSubscription(context context.Context) PushSubscription {
	return PushSubscription{
		channel:        make(chan PushMessage),
		timeoutMinutes: 3,
	}
}

func (r *PushSubscription) AddHandler(h Handler) {
	r.handlers = append(r.handlers, h)
}

// AddFullHandler is the same as AddHandler except it ensures the interface implements everything
func (r *PushSubscription) AddFullHandler(h HandlerAll) {
	r.handlers = append(r.handlers, h)
}

var RealTimeHandlers map[string]func(r *PushSubscription, channel string, data ...interface{})
var RealTimeSystemHandlers map[string]func(r *PushSubscription, channel string, id groupmeclient.ID, rawData []byte)

func (r *PushSubscription) HandleMessageLoop() {
	for msg := range r.channel {
		r.connectionTimeout = time.Now().Unix() + (60 * r.timeoutMinutes)
		data := msg.Data()
		content := data["subject"]
		dataType := data["type"]
		if dataType == nil {
			continue
		}
		contentType := dataType.(string)
		channel := msg.Channel()

		// log.Printf("finding handler for contentType %s\n", contentType)
		handler, ok := RealTimeHandlers[contentType]
		if !ok {
			if contentType == "ping" ||
				len(contentType) == 0 ||
				content == nil {
				continue
			}
			log.Println("Unable to handle GroupMe message type", contentType)
		}

		handler(r, channel, content)
	}
}

func (r *PushSubscription) Setup(context context.Context, client faye.FayeClient) error {
	r.fayeClient = &client
	if err := r.fayeClient.HandshakeAndConnect(); err != nil {
		return err
	}

	go r.HandleMessageLoop()
	go r.StayConnectedLoop()
	return nil
}

func (r *PushSubscription) StayConnectedLoop() {
	time.Sleep(5 * time.Second)
	for {
		if !r.fayeClient.Connected() {
			retries := 3
			retry_wait_seconds := 5
			reconnected := false
			for i := range retries {
				log.Println("PushSubscription reconnecting")
				if err := r.fayeClient.HandshakeAndConnect(); err != nil {
					log.Printf("PushSubscription could not reconnect on try %d (%s), retrying in %d seconds", i+1, err, retry_wait_seconds)
					time.Sleep(time.Duration(retry_wait_seconds) * time.Second)
					continue
				}
				reconnected = true
				break
			}
			if !reconnected {
				log.Printf("PushSubscription could not reconnect after %d retries\n", retries)
				break
			}
			// r.fayeClient.Resubscribe()
		}
		time.Sleep(5 * time.Second)
	}
}

// SubscribeToUser to users
func (r *PushSubscription) SubscribeToUser(context context.Context, id groupmeclient.ID) error {
	return r.subscribeWithPrefix(userChannel, id)
}

// SubscribeToGroup to groups for typing notification
func (r *PushSubscription) SubscribeToGroup(context context.Context, id groupmeclient.ID) error {
	return r.subscribeWithPrefix(groupChannel, id)
}

// SubscribeToDM to users
func (r *PushSubscription) SubscribeToDM(context context.Context, id groupmeclient.ID) error {
	id = groupmeclient.ID(strings.Replace(id.String(), "+", "_", 1))
	return r.subscribeWithPrefix(dmChannel, id)
}

func (r *PushSubscription) subscribeWithPrefix(prefix string, groupID groupmeclient.ID) error {
	concur.Lock()
	defer concur.Unlock()
	if r.fayeClient == nil {
		return ErrListenerNotStarted
	}

	channel := prefix + groupID.String()
	c_new := make(chan faye.Message)
	r.fayeClient.WaitSubscribe(channel, c_new)
	//converting between types because channels don't support interfaces well
	go func() {
		for i := range c_new {
			r.channel <- i
		}
	}()

	return nil
}
