package apps

import (
	"context"
	"github.com/pkg/errors"
	slackpkg "github.com/slack-go/slack"
)

type Slacks interface {
	CreateNewChannel(ctx context.Context, name string, isPrivate ...bool) (*slackpkg.Channel, error)
	InviteUserToChannel(ctx context.Context, channelId string, users ...string) error
	ArchiveChannel(ctx context.Context, channelId string) error
	AddReminderToChannel(ctx context.Context, channelId, textReminder, time string) error
}

type slack struct {
	client *slackpkg.Client
}

func NewSlack(botToken string) *slack {
	client := slackpkg.New(botToken, slackpkg.OptionDebug(true))
	return &slack{client: client}
}

// CreateNewChannel - create new channel slack, with params:
// - name: channel name
// - isPrivate: is private channel ?? default is false, then should be create public channel
func (s *slack) CreateNewChannel(ctx context.Context, name string, isPrivate ...bool) (*slackpkg.Channel, error) {
	isPrivateChannel := false
	if isPrivate != nil && len(isPrivate) > 0 {
		isPrivateChannel = isPrivate[0]
	}
	return s.client.CreateConversationContext(ctx, slackpkg.CreateConversationParams{
		ChannelName: name,
		IsPrivate:   isPrivateChannel,
	})
}

func (s *slack) InviteUserToChannel(ctx context.Context, channelId string, users ...string) error {
	if _, err := s.client.InviteUsersToConversationContext(ctx, channelId, users...); err != nil {
		return errors.Wrap(err, "phastos.go.apps.slack.InviteUserToChannel")
	}
	return nil
}

// ArchiveChannel - archive channel slack
func (s *slack) ArchiveChannel(ctx context.Context, channelId string) error {
	return s.client.ArchiveConversationContext(ctx, channelId)
}

// AddReminderToChannel - adding reminder to channel slack by ID
func (s *slack) AddReminderToChannel(ctx context.Context, channelId, textReminder, time string) error {
	if _, err := s.client.AddChannelReminderContext(ctx, channelId, textReminder, time); err != nil {
		return errors.Wrap(err, "phastos.go.apps.slack.AddReminderToChannel")
	}
	return nil
}
