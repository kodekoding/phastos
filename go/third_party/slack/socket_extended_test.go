package slack

import (
	"context"
	"testing"

	slackpkg "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/assert"

	handler2 "github.com/kodekoding/phastos/v2/go/third_party/slack/handler"
)

func TestNewSlackApp_Valid(t *testing.T) {
	app, err := NewSlackApp("xapp-test-token", "xoxb-test-token")
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, "xapp-test-token", app.appToken)
	assert.Equal(t, "xoxb-test-token", app.botToken)
	assert.NotNil(t, app.api)
}

func TestApp_GetAPI(t *testing.T) {
	app, err := NewSlackApp("xapp-test-token", "xoxb-test-token")
	assert.NoError(t, err)
	api := app.GetAPI()
	assert.NotNil(t, api)
}

func TestWithSocketMode(t *testing.T) {
	app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
	assert.NoError(t, err)
	assert.NotNil(t, app.socket)
	assert.NotNil(t, app.socketHandler)
}

func TestWithDebugEnabled(t *testing.T) {
	t.Run("without socket mode", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithDebug(true))
		assert.NoError(t, err)
		assert.NotNil(t, app.api)
	})

	t.Run("with socket mode", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode(), WithDebug(true))
		assert.NoError(t, err)
		assert.NotNil(t, app.socket)
		assert.NotNil(t, app.socketHandler)
	})
}

func TestWrapHandler_InteractionCallback(t *testing.T) {
	t.Run("with valid interaction callback data and ack", func(t *testing.T) {
		app := &App{api: &slackpkg.Client{}}
		client := socketmode.New(&slackpkg.Client{})
		event := &socketmode.Event{
			Request: &socketmode.Request{EnvelopeID: "test-env-id"},
			Data: slackpkg.InteractionCallback{
				ActionTs: "12345.67890",
				User:     slackpkg.User{ID: "U12345", Name: "testuser"},
			},
		}

		var capturedRequest handler2.SocketRequest
		wrapped := app.wrapHandler(func(ctx context.Context, req handler2.SocketRequest) error {
			capturedRequest = req
			return nil
		})

		wrapped(event, client)

		interactionData, err := capturedRequest.GetInteractionData()
		assert.NoError(t, err)
		assert.NotNil(t, interactionData)
		assert.Equal(t, "U12345", interactionData.User.ID)

		_, err = capturedRequest.GetEventData()
		assert.Error(t, err)

		_, err = capturedRequest.GetSlashCommandData()
		assert.Error(t, err)
	})

	t.Run("with invalid interaction callback data (handler error)", func(t *testing.T) {
		app := &App{api: &slackpkg.Client{}}
		client := socketmode.New(&slackpkg.Client{})

		event := &socketmode.Event{
			Request: &socketmode.Request{EnvelopeID: "test-env-id"},
			Data:    "invalid data type",
		}

		called := false
		wrapped := app.wrapHandler(func(ctx context.Context, req handler2.SocketRequest) error {
			called = true

			_, err := req.GetInteractionData()
			assert.Error(t, err)

			_, err = req.GetEventData()
			assert.Error(t, err)

			_, err = req.GetSlashCommandData()
			assert.Error(t, err)

			return assert.AnError
		})

		wrapped(event, client)
		assert.True(t, called)
	})
}

func TestWrapHandler_EventsAPIEvent(t *testing.T) {
	t.Run("with valid events API event data and no ack", func(t *testing.T) {
		app := &App{api: &slackpkg.Client{}}
		client := socketmode.New(&slackpkg.Client{})

		event := &socketmode.Event{
			Data: slackevents.EventsAPIEvent{
				Type: string(slackevents.LinkShared),
				InnerEvent: slackevents.EventsAPIInnerEvent{
					Type: string(slackevents.LinkShared),
					Data: &slackevents.AppHomeOpenedEvent{User: "U12345"},
				},
			},
		}

		var capturedRequest handler2.SocketRequest
		wrapped := app.wrapHandler(func(ctx context.Context, req handler2.SocketRequest) error {
			capturedRequest = req
			return nil
		}, false)

		wrapped(event, client)

		eventData, err := capturedRequest.GetEventData()
		assert.NoError(t, err)
		assert.NotNil(t, eventData)

		_, err = capturedRequest.GetInteractionData()
		assert.Error(t, err)
	})
}

func TestWrapHandler_SlashCommand(t *testing.T) {
	t.Run("with valid slash command data and ack", func(t *testing.T) {
		app := &App{api: &slackpkg.Client{}}
		client := socketmode.New(&slackpkg.Client{})

		event := &socketmode.Event{
			Request: &socketmode.Request{EnvelopeID: "slash-env-id"},
			Data: slackpkg.SlashCommand{
				Command:     "/test",
				Text:        "some args",
				UserID:      "U12345",
				ChannelID:   "C12345",
				TriggerID:   "T12345.67890",
			},
		}

		var capturedRequest handler2.SocketRequest
		wrapped := app.wrapHandler(func(ctx context.Context, req handler2.SocketRequest) error {
			capturedRequest = req
			return nil
		})

		wrapped(event, client)

		slashData, err := capturedRequest.GetSlashCommandData()
		assert.NoError(t, err)
		assert.NotNil(t, slashData)
		assert.Equal(t, "/test", slashData.Command)
	})

	t.Run("with handler returning error (no ack)", func(t *testing.T) {
		app := &App{api: &slackpkg.Client{}}
		client := socketmode.New(&slackpkg.Client{})

		event := &socketmode.Event{
			Request: &socketmode.Request{EnvelopeID: "slash-env-id-2"},
			Data: slackpkg.SlashCommand{
				Command: "/fail",
			},
		}

		wrapped := app.wrapHandler(func(ctx context.Context, req handler2.SocketRequest) error {
			_, err := req.GetSlashCommandData()
			assert.NoError(t, err)
			return assert.AnError
		})

		wrapped(event, client)
	})
}

func TestApp_AddHandler(t *testing.T) {
	t.Run("with slash command event", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
		assert.NoError(t, err)

		handler := &extendedStubHandler{
			config: handler2.SocketHandlerConfig{
				Handler: []handler2.SocketEvent{
					handler2.RegisterEvent(func(ctx context.Context, req handler2.SocketRequest) error {
						return nil
					}, handler2.WithIdentifier("/test")),
				},
			},
		}
		app.AddHandler(handler)
		assert.Equal(t, 1, app.totalEvents)
	})

	t.Run("with action event", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
		assert.NoError(t, err)

		handler := &extendedStubHandler{
			config: handler2.SocketHandlerConfig{
				Handler: []handler2.SocketEvent{
					handler2.RegisterEvent(func(ctx context.Context, req handler2.SocketRequest) error {
						return nil
					}, handler2.WithIdentifier("action_button_click")),
				},
			},
		}
		app.AddHandler(handler)
		assert.Equal(t, 1, app.totalEvents)
	})

	t.Run("with default event (empty identifier)", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
		assert.NoError(t, err)

		handler := &extendedStubHandler{
			config: handler2.SocketHandlerConfig{
				Handler: []handler2.SocketEvent{
					handler2.RegisterEvent(func(ctx context.Context, req handler2.SocketRequest) error {
						return nil
					}),
				},
			},
		}
		app.AddHandler(handler)
		assert.Equal(t, 1, app.totalEvents)
	})

	t.Run("with interaction type", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
		assert.NoError(t, err)

		handler := &extendedStubHandler{
			config: handler2.SocketHandlerConfig{
				Handler: []handler2.SocketEvent{
					handler2.RegisterEvent(func(ctx context.Context, req handler2.SocketRequest) error {
						return nil
					}, 					handler2.WithEventType(slackpkg.InteractionTypeBlockActions)),
				},
			},
		}
		app.AddHandler(handler)
		assert.Equal(t, 1, app.totalEvents)
	})

	t.Run("with socketmode event type", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
		assert.NoError(t, err)

		handler := &extendedStubHandler{
			config: handler2.SocketHandlerConfig{
				Handler: []handler2.SocketEvent{
					handler2.RegisterEvent(func(ctx context.Context, req handler2.SocketRequest) error {
						return nil
					}, handler2.WithEventType(socketmode.EventTypeSlashCommand)),
				},
			},
		}
		app.AddHandler(handler)
		assert.Equal(t, 1, app.totalEvents)
	})

	t.Run("with events api type", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
		assert.NoError(t, err)

		handler := &extendedStubHandler{
			config: handler2.SocketHandlerConfig{
				Handler: []handler2.SocketEvent{
					handler2.RegisterEvent(func(ctx context.Context, req handler2.SocketRequest) error {
						return nil
					}, handler2.WithEventType(slackevents.EventsAPIType("message"))),
				},
			},
		}
		app.AddHandler(handler)
		assert.Equal(t, 1, app.totalEvents)
	})
}

func TestApp_Start(t *testing.T) {
	t.Run("without socket or http", func(t *testing.T) {
		app := &App{}
		app.Start()
	})

	t.Run("with socket only", func(t *testing.T) {
		app, err := NewSlackApp("xapp-test-token", "xoxb-test-token", WithSocketMode())
		assert.NoError(t, err)
		app.Start()
	})
}

func TestAppImplementorMethods(t *testing.T) {
	impl := &AppImplementor{}
	assert.NotNil(t, impl)
}

type extendedStubHandler struct {
	config handler2.SocketHandlerConfig
}

func (s *extendedStubHandler) GetConfig() handler2.SocketHandlerConfig {
	return s.config
}
