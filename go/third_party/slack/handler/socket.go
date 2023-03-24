package handler

import (
	"context"
	slackpkg "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type (
	SocketRequest struct {
		GetSlashCommandData func() (*slackpkg.SlashCommand, error)
		GetInteractionData  func() (*slackpkg.InteractionCallback, error)
		GetEventData        func() (*slackevents.EventsAPIEvent, error)
		Client              *socketmode.Client
		Event               *socketmode.Event
	}

	EventHandler func(ctx context.Context, request SocketRequest) error

	EventOptions func(event *SocketEvent)
	SocketEvent  struct {
		Handler    EventHandler
		Type       any
		Identifier string
	}
	SocketHandlerConfig struct {
		Handler []SocketEvent
	}

	SocketHandler interface {
		GetConfig() SocketHandlerConfig
	}

	SocketHandlerImpl struct{}
)

func RegisterEvent(handler EventHandler, opts ...EventOptions) SocketEvent {
	socketEvent := SocketEvent{
		Handler: handler,
		Type:    "",
	}
	for _, opt := range opts {
		opt(&socketEvent)
	}
	return socketEvent
}

func WithEventType(eventType any) EventOptions {
	return func(r *SocketEvent) {
		r.Type = eventType
	}
}

func WithIdentifier(identifier string) EventOptions {
	return func(r *SocketEvent) {
		r.Identifier = identifier
	}
}
