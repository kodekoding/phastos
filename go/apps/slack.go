package apps

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	slackentity "github.com/kodekoding/phastos/go/entity/slack"
	"github.com/pkg/errors"
)

type (
	Slacks interface {
		channels
		messages
		oauths
		users
	}

	channels interface {
		CreateNewChannel(ctx context.Context, name string, isPrivate ...bool) (*slackentity.Response[slackentity.Channel], error)
		InviteUserToChannel(ctx context.Context, channelId string, users ...string) error
		ArchiveChannel(ctx context.Context, channelId string) error
		AddReminderToChannel(ctx context.Context, channelId, textReminder, time string) error
	}

	messages interface {
		PostMessageText(ctx context.Context, destId string, text string) error
		PostMessageBlocks(ctx context.Context, destId string, blocksString string) error
	}

	oauths interface {
		GetOauthAccess(ctx context.Context, codeCallback string) (*slackentity.Response[slackentity.OauthAccess], error)
	}

	users interface {
	}

	slack struct {
		client       *resty.Client
		botToken     string
		appToken     string
		userToken    string
		clientID     string
		clientSecret string
	}
)

const prefixURL = "https://slack.com/api"

func NewSlack(botToken, clientId, clientSecret string) *slack {
	client := resty.New()
	return &slack{botToken: botToken, client: client, clientID: clientId, clientSecret: clientSecret}
}

func (s *slack) newCURL(ctx context.Context, contentType ...string) *resty.Request {
	cType := "application/json"
	if len(contentType) > 1 && contentType != nil {
		cType = contentType[0]
	}
	return s.client.R().
		SetContext(ctx).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", s.botToken)).
		SetHeader("Content-Type", cType)
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

// GetOauthAccess - Get Oauth Access Data
func (s *slack) GetOauthAccess(ctx context.Context, codeCallback string) (*slackentity.Response[slackentity.OauthAccess], error) {
	var result slackentity.Response[slackentity.OauthAccess]
	if _, err := s.newCURL(ctx, "application/x-www-form-urlencoded").
		SetContentLength(true).
		SetHeader("Cache-Control", "no-cache").
		SetResult(&result).
		SetFormData(map[string]string{
			"code":          codeCallback,
			"client_id":     s.clientID,
			"client_secret": s.clientSecret,
		}).Post(fmt.Sprintf("%s/oauth.v2.access", prefixURL)); err != nil {
		return nil, errors.Wrap(err, "phastos.apps.slack.GetOauthAccess.Post")
	}
	return &result, nil
}
