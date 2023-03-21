package apps

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	slackentity "github.com/kodekoding/phastos/go/entity/slack"
	"github.com/pkg/errors"
)

type Slacks interface {
	CreateNewChannel(ctx context.Context, name string, isPrivate ...bool) (*slackentity.Response[slackentity.Channel], error)
	InviteUserToChannel(ctx context.Context, channelId string, users ...string) error
	ArchiveChannel(ctx context.Context, channelId string) error
	AddReminderToChannel(ctx context.Context, channelId, textReminder, time string) error
	PostMessageText(ctx context.Context, destId string, text string) error
	PostMessageBlocks(ctx context.Context, destId string, blocksString string) error
}

type slack struct {
	client    *resty.Client
	botToken  string
	appToken  string
	userToken string
}

const prefixURL = "https://slack.com/api"

func NewSlack(botToken string) *slack {
	client := resty.New()
	return &slack{botToken: botToken, client: client}
}

func (s *slack) newCURL(ctx context.Context) *resty.Request {
	return s.client.R().
		SetContext(ctx).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", s.botToken)).
		SetHeader("Content-Type", "application/json")
}

// CreateNewChannel - create new channel slack, with params:
// - name: channel name
// - isPrivate: is private channel ?? default is false, then should be create public channel
func (s *slack) CreateNewChannel(ctx context.Context, name string, isPrivate ...bool) (*slackentity.Response[slackentity.Channel], error) {
	isPrivateChannel := false
	if isPrivate != nil && len(isPrivate) > 0 {
		isPrivateChannel = isPrivate[0]
	}

	var result slackentity.Response[slackentity.Channel]

	if _, err := s.newCURL(ctx).
		SetResult(&result).
		SetBody(&slackentity.ChannelCreateRequest{
			Name:      name,
			IsPrivate: isPrivateChannel,
		}).
		Post(fmt.Sprintf("%s/conversation.created", prefixURL)); err != nil {
		return nil, errors.Wrap(err, "phastos.apps.slack.CreateNewChannel.Post")
	}
	return &result, nil
}

func (s *slack) InviteUserToChannel(ctx context.Context, channelId string, users ...string) error {
	if _, err := s.newCURL(ctx).
		SetBody(map[string]interface{}{
			"channel": channelId,
			"users":   users,
		}).Post(fmt.Sprintf("%s/conversation.invite", prefixURL)); err != nil {
		return errors.Wrap(err, "phastos.apps.slack.InviteUserToChannel.Post")
	}
	return nil
}

// ArchiveChannel - archive channel slack
func (s *slack) ArchiveChannel(ctx context.Context, channelId string) error {
	if _, err := s.newCURL(ctx).
		SetBody(map[string]interface{}{
			"channel": channelId,
		}).Post(fmt.Sprintf("%s/conversation.archive", prefixURL)); err != nil {
		return errors.Wrap(err, "phastos.apps.slack.ArchiveChannel.Post")
	}
	return nil
}

// AddReminderToChannel - adding reminder to channel slack by ID
func (s *slack) AddReminderToChannel(ctx context.Context, channelId, textReminder, time string) error {
	if _, err := s.newCURL(ctx).
		SetBody(map[string]interface{}{
			"text":    textReminder,
			"time":    time,
			"channel": channelId,
		}).Post(fmt.Sprintf("%s/reminders.add", prefixURL)); err != nil {
		return errors.Wrap(err, "phastos.apps.slack.AddReminderToChannel.Post")
	}
	return nil
}

// PostMessageText - Send a Message as a bot
func (s *slack) PostMessageText(ctx context.Context, destId string, text string) error {
	if _, err := s.newCURL(ctx).
		SetBody(map[string]interface{}{
			"text":    text,
			"channel": destId,
		}).Post(fmt.Sprintf("%s/chat.postMessage", prefixURL)); err != nil {
		return errors.Wrap(err, "phastos.apps.slack.PostMessageText.Post")
	}
	return nil
}

// PostMessageBlocks - Send a Message as a bot with block kit (see https://api.slack.com/tools/block-kit-builder for reference)
func (s *slack) PostMessageBlocks(ctx context.Context, destId string, blocksString string) error {
	if _, err := s.newCURL(ctx).
		SetBody(map[string]interface{}{
			"blocks":  blocksString,
			"channel": destId,
		}).Post(fmt.Sprintf("%s/chat.postMessage", prefixURL)); err != nil {
		return errors.Wrap(err, "phastos.apps.slack.PostMessageText.Post")
	}
	return nil
}
