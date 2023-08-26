package notifications

import (
	"context"
	_log "log"
	"net/http"

	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/notifications/slack"
	"github.com/kodekoding/phastos/v2/go/notifications/telegram"
)

type (
	Action interface {
		Send(ctx context.Context, text string, attachment interface{}) error
		IsActive() bool
		Type() string
		SetTraceId(traceId string)
		SetDestination(destination interface{})
	}

	Platforms interface {
		Telegram() Action
		Slack() Action
		GetAllPlatform() []Action
		Handler(next http.Handler) http.Handler
	}

	Platform struct {
		telegram Action
		slack    Action
		list     []Action
	}

	Config struct {
		Telegram *telegram.TelegramConfig `yaml:"telegram"`
		Slack    *slack.SlackConfig       `yaml:"slack"`
	}
)

func New(config *Config) *Platform {
	var (
		telegramService = new(telegram.Service)
		slackService    = new(slack.Service)
		err             error
		listOfPlatform  []Action
	)

	if config.Telegram != nil && config.Telegram.IsActive {
		telegramService, err = telegram.New(config.Telegram)
		if err != nil {
			log.Error("Telegram Cannot be up, because: ", err.Error())
			return nil
		}
		listOfPlatform = append(listOfPlatform, telegramService)
		_log.Println("Telegram is up")
	}

	if config.Slack != nil && config.Slack.IsActive {
		slackService, err = slack.New(config.Slack)
		if err != nil {
			log.Error("Slack Cannot be up, because: ", err.Error())
			return nil
		}
		listOfPlatform = append(listOfPlatform, slackService)
		_log.Println("Slack is up")
	}
	return &Platform{
		telegram: telegramService,
		slack:    slackService,
		list:     listOfPlatform,
	}
}

func (this *Platform) Telegram() Action {
	return this.telegram
}

func (this *Platform) Slack() Action {
	return this.slack
}

func (this *Platform) GetAllPlatform() []Action {
	return this.list
}

func (this *Platform) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := context.WithValue(request.Context(), entity.NotifPlatformContext{}, this)
		*request = *request.WithContext(ctx)

		next.ServeHTTP(writer, request)
	})
}
