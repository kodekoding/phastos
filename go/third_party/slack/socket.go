package slack

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	log2 "github.com/rs/zerolog/log"
	slackpkg "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/kodekoding/phastos/v2/go/api"
	handler2 "github.com/kodekoding/phastos/v2/go/third_party/slack/handler"
)

type (
	AppImplementor struct{}
	AppOptions     func(*App)
	App            struct {
		api           *slackpkg.Client
		socket        *socketmode.Client
		socketHandler *socketmode.SocketmodeHandler
		botToken      string
		appToken      string
		LoadModule    func()
		totalEvents   int
		*api.App
	}
)

func NewSlackApp(appToken, botToken string, opts ...AppOptions) (*App, error) {
	if !strings.HasPrefix(appToken, "xapp-") {
		return nil, errors.Wrap(errors.New("SLACK_APP_TOKEN must have the prefix \"xapp-\"."), "phastos.third_party.slack.NewSlackSocketMode.CheckAppToken")
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		return nil, errors.Wrap(errors.New("SLACK_BOT_TOKEN must have the prefix \"xoxb-\"."), "phastos.third_party.slack.NewSlackSocketMode.CheckBotToken")
	}

	apiClient := slackpkg.New(
		botToken,
		slackpkg.OptionAppLevelToken(appToken),
		slackpkg.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
	)

	slackApp := &App{
		appToken: appToken,
		botToken: botToken,
		api:      apiClient,
	}
	for _, opt := range opts {
		opt(slackApp)
	}
	return slackApp, nil
}

func WithSocketMode() AppOptions {
	return func(app *App) {
		socketClient := socketmode.New(
			app.api,
			socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
		)

		socketHandler := socketmode.NewSocketmodeHandler(socketClient)
		app.socket = socketClient
		app.socketHandler = socketHandler
	}
}

func WithHttp(port ...int) AppOptions {
	return func(app *App) {
		servedPort := 8000
		if port != nil && len(port) > 0 {
			servedPort = port[0]
		}
		app.App = api.NewApp(api.WithAppPort(servedPort))
		app.Init()
	}
}

func WithDebug(enabled bool) AppOptions {
	return func(app *App) {
		if enabled {
			apiClient := slackpkg.New(
				app.botToken,
				slackpkg.OptionDebug(enabled),
				slackpkg.OptionAppLevelToken(app.appToken),
				slackpkg.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
			)
			app.api = apiClient

			if app.socket != nil {
				socketClient := socketmode.New(
					apiClient,
					socketmode.OptionDebug(enabled),
					socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
				)
				app.socket = socketClient

				socketHandler := socketmode.NewSocketmodeHandler(socketClient)
				app.socketHandler = socketHandler
			}
		}
	}
}

func (app *App) GetAPI() *slackpkg.Client {
	return app.api
}

func (app *App) wrapHandler(handler handler2.EventHandler, shouldAck ...bool) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {
		const notValidData = "event data not valid"
		request := handler2.SocketRequest{
			GetInteractionData: func() (*slackpkg.InteractionCallback, error) {
				data, valid := event.Data.(slackpkg.InteractionCallback)
				if !valid {
					return nil, errors.Wrap(errors.New(notValidData), "phastos.third_party.slack.socket.wrapHandler.GetInteractionData")
				}
				return &data, nil
			},
			GetEventData: func() (*slackevents.EventsAPIEvent, error) {
				data, valid := event.Data.(slackevents.EventsAPIEvent)
				if !valid {
					return nil, errors.Wrap(errors.New(notValidData), "phastos.third_party.slack.socket.wrapHandler.GetEventData")
				}
				return &data, nil
			},
			GetSlashCommandData: func() (*slackpkg.SlashCommand, error) {
				data, valid := event.Data.(slackpkg.SlashCommand)
				if !valid {
					return nil, errors.Wrap(errors.New(notValidData), "phastos.third_party.slack.socket.wrapHandler.GetSlashCommandData")
				}
				return &data, nil
			},
			Client: client,
			Event:  event,
		}

		needAck := true
		if shouldAck != nil && len(shouldAck) > 0 {
			needAck = shouldAck[0]
		}

		if err := handler(context.Background(), request); err != nil {
			log2.Error().Err(err).Msg("handler got error")
		} else if needAck {
			// if not err then ack the request
			client.Ack(*event.Request)
		}
	}
}

func (app *App) AddHandler(socketHandler handler2.SocketHandler) {
	if app.socket == nil && app.socketHandler == nil {
		return
	}
	config := socketHandler.GetConfig()
	for _, event := range config.Handler {
		switch identifier := event.Type.(type) {
		case string:
			if strings.HasPrefix(identifier, "/") {
				app.socketHandler.HandleSlashCommand(identifier, app.wrapHandler(event.Handler))
			} else if strings.HasPrefix(identifier, "action_") {
				app.socketHandler.HandleInteractionBlockAction(identifier, app.wrapHandler(event.Handler))
			} else if identifier == "" {
				app.socketHandler.HandleDefault(app.wrapHandler(event.Handler, false))
			}
		case slackpkg.InteractionType:
			app.socketHandler.HandleInteraction(identifier, app.wrapHandler(event.Handler))
		case socketmode.EventType:
			app.socketHandler.Handle(identifier, app.wrapHandler(event.Handler))
		case slackevents.EventsAPIType:
			app.socketHandler.HandleEvents(identifier, app.wrapHandler(event.Handler))
		}
	}

	app.totalEvents += len(config.Handler)
}

func (app *App) Start() {
	go func() {
		if app.socket != nil && app.socketHandler != nil {
			log.Println("Slack Socket running, serving ", app.totalEvents, " event(s)")
			if err := app.socketHandler.RunEventLoop(); err != nil {
				log.Fatalln("cannot run socket socket: ", err.Error())
			}
		}
	}()

	if app.App != nil {
		_ = app.App.Start()
	}
}
