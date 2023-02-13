package apps

import (
	"github.com/pkg/errors"
	"log"
	"os"

	slackpkg "github.com/slack-go/slack"
)

type Slacks interface {
	CreateNewChannel(name string, isPrivate ...bool) (*slackpkg.Channel, error)
	InviteUserToChannel(channelId string, users ...string) error
	ArchiveChannel(channelId string) error
}

type slack struct {
	client *slackpkg.Client
}

func NewSlack(botToken string) *slack {
	client := slackpkg.New(
		botToken,
		slackpkg.OptionDebug(true),
		slackpkg.OptionLog(log.New(os.Stdout, "sertif-api: ", log.Lshortfile|log.LstdFlags)),
	)

	return &slack{client: client}
}

// CreateNewChannel - create new channel slack, with params:
// - name: channel name
// - isPrivate: is private channel ?? default is false, then should be create public channel
func (s *slack) CreateNewChannel(name string, isPrivate ...bool) (*slackpkg.Channel, error) {
	isPrivateChannel := false
	if isPrivate != nil && len(isPrivate) > 0 {
		isPrivateChannel = isPrivate[0]
	}
	return s.client.CreateConversation(slackpkg.CreateConversationParams{
		ChannelName: name,
		IsPrivate:   isPrivateChannel,
	})
}

func (s *slack) InviteUserToChannel(channelId string, users ...string) error {
	if _, err := s.client.InviteUsersToConversation(channelId, users...); err != nil {
		return errors.Wrap(err, "phastos.go.apps.slack.InviteUserToChannel")
	}
	return nil
}

// ArchiveChannel - archive channel slack
func (s *slack) ArchiveChannel(channelId string) error {
	return s.client.ArchiveConversation(channelId)
}
