package slack

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	slackpkg "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	apppkg "github.com/kodekoding/phastos/go/app"
	log2 "github.com/kodekoding/phastos/go/log"
	handler2 "github.com/kodekoding/phastos/go/third_party/slack/handler"
)

type (
	Apps interface {
		LoadModules()
	}

	AppImplementor struct{}

	AppOptions func(*app)
	app        struct {
		Api           *slackpkg.Client
		socket        *socketmode.Client
		socketHandler *socketmode.SocketmodeHandler
		botToken      string
		appToken      string
		LoadModule    func()
		totalEvents   int
		*apppkg.API
	}
)

func NewSlackApp(appToken, botToken string, opts ...AppOptions) (*app, error) {
	if !strings.HasPrefix(appToken, "xapp-") {
		return nil, errors.Wrap(errors.New("SLACK_APP_TOKEN must have the prefix \"xapp-\"."), "phastos.third_party.slack.NewSlackSocketMode.CheckAppToken")
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		return nil, errors.Wrap(errors.New("SLACK_BOT_TOKEN must have the prefix \"xoxb-\"."), "phastos.third_party.slack.NewSlackSocketMode.CheckBotToken")
	}

	api := slackpkg.New(
		botToken,
		slackpkg.OptionDebug(true),
		slackpkg.OptionAppLevelToken(appToken),
		slackpkg.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
	)

	socketClient := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	socketHandler := socketmode.NewSocketmodeHandler(socketClient)

	slackApp := &app{
		Api:           api,
		socket:        socketClient,
		socketHandler: socketHandler,
	}
	for _, opt := range opts {
		opt(slackApp)
	}
	return slackApp, nil
}

func WithHttp(port ...int) AppOptions {
	return func(app *app) {
		servedPort := 8000
		if port != nil && len(port) > 0 {
			servedPort = port[0]
		}
		app.API = apppkg.NewAPI(apppkg.WithAppPort(servedPort))
		app.Init()
	}
}

func (app *app) wrapHandler(handler handler2.EventHandler, shouldAck ...bool) socketmode.SocketmodeHandlerFunc {
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
		if needAck {
			client.Ack(*event.Request)
		}

		if err := handler(context.Background(), request); err != nil {
			log2.Errorln("handler got error: ", err.Error())
		}
	}
}

func (app *app) AddHandler(socketHandler handler2.SocketHandler) {
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

func (app *app) Start() {
	go func() {
		log.Println("Slack Socket running, serving ", app.totalEvents, " event(s)")
		if err := app.socketHandler.RunEventLoop(); err != nil {
			log.Fatalln("cannot run socket socket: ", err.Error())
		}
	}()

	if app.API != nil {
		_ = app.API.Start()
	}
}
