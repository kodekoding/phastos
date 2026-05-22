package handler

import (
	"context"
	"testing"

	slackpkg "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/assert"
)

func TestRegisterEvent(t *testing.T) {
	event := RegisterEvent(func(ctx context.Context, req SocketRequest) error {
		return nil
	})
	assert.NotNil(t, event.Handler)
	assert.Equal(t, "", event.Type)
	assert.Equal(t, "", event.Identifier)
}

func TestRegisterEventWithEventType(t *testing.T) {
	event := RegisterEvent(func(ctx context.Context, req SocketRequest) error {
		return nil
	}, WithEventType("message"))
	assert.Equal(t, "message", event.Type)
}

func TestRegisterEventWithIdentifier(t *testing.T) {
	event := RegisterEvent(func(ctx context.Context, req SocketRequest) error {
		return nil
	}, WithIdentifier("/command"))
	assert.Equal(t, "/command", event.Identifier)
}

func TestSocketEventStruct(t *testing.T) {
	event := SocketEvent{
		Handler:    func(ctx context.Context, req SocketRequest) error { return nil },
		Type:       "interaction",
		Identifier: "action_button",
	}
	assert.NotNil(t, event.Handler)
	assert.Equal(t, "interaction", event.Type)
	assert.Equal(t, "action_button", event.Identifier)
}

func TestSocketHandlerConfigStruct(t *testing.T) {
	cfg := SocketHandlerConfig{
		Handler: []SocketEvent{
			RegisterEvent(func(ctx context.Context, req SocketRequest) error { return nil }),
		},
	}
	assert.Len(t, cfg.Handler, 1)
}

func TestSocketHandlerImpl(t *testing.T) {
	impl := &SocketHandlerImpl{}
	assert.NotNil(t, impl)
}

func TestSocketRequestStruct(t *testing.T) {
	req := SocketRequest{
		GetSlashCommandData: func() (*slackpkg.SlashCommand, error) { return nil, nil },
		GetInteractionData:  func() (*slackpkg.InteractionCallback, error) { return nil, nil },
		GetEventData:        func() (*slackevents.EventsAPIEvent, error) { return nil, nil },
	}
	assert.NotNil(t, req.GetSlashCommandData)
	assert.NotNil(t, req.GetInteractionData)
	assert.NotNil(t, req.GetEventData)
}
