package notifications

import (
	"context"
	_log "log"

	"github.com/kodekoding/phastos/go/log"
	"github.com/kodekoding/phastos/go/notifications/slack"
	"github.com/kodekoding/phastos/go/notifications/telegram"
)

type (
	Action interface {
		Send(ctx context.Context, text string, attachment interface{}) error
		IsActive() bool
		Type() string
		SetTraceId(traceId string) Action
		SetDestination(destination interface{}) Action
	}

	Platforms interface {
		Telegram() Action
		Slack() Action
		GetAllPlatform() []Action
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

	if config.Telegram != nil {
		telegramService, err = telegram.New(config.Telegram)
		if err != nil {
			log.Error("Telegram Cannot be up, because: ", err.Error())
			return nil
		}
		listOfPlatform = append(listOfPlatform, telegramService)
		if config.Telegram.IsActive {
			_log.Println("Telegram is up")
		}
	}

	if config.Slack != nil {
		slackService, err = slack.New(config.Slack)
		if err != nil {
			log.Error("Slack Cannot be up, because: ", err.Error())
			return nil
		}
		listOfPlatform = append(listOfPlatform, slackService)
		if config.Slack.IsActive {
			_log.Println("Slack is up")
		}
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
