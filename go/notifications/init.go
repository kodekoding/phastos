package notifications

import (
	"context"
	"github.com/rs/zerolog/log"
	"net/http"

	"github.com/kodekoding/phastos/v2/go/entity"
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

	Options func(platform *Platform)

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

func New(opt ...Options) *Platform {
	var (
		telegramService = new(telegram.Service)
		slackService    = new(slack.Service)
		listOfPlatform  []Action
	)

	notifPlatform := &Platform{
		telegram: telegramService,
		slack:    slackService,
		list:     listOfPlatform,
	}

	for _, options := range opt {
		options(notifPlatform)
	}

	return notifPlatform
}

func ActivateSlack(webhookURL string) Options {
	return func(platform *Platform) {
		var err error
		platform.slack, err = slack.New(&slack.SlackConfig{URL: webhookURL, IsActive: true})
		if err != nil {
			log.Error().Msgf("slack cannot initialized: %s", err)
			return
		}
		log.Info().Msg("slack notification initialized")
	}
}

func ActivateTelegram(botToken string) Options {
	return func(platform *Platform) {
		var err error
		platform.telegram, err = telegram.New(&telegram.TelegramConfig{BotToken: botToken, IsActive: true})
		if err != nil {
			log.Error().Msgf("telegram cannot initialized: %s", err)
			return
		}
		log.Info().Msg("telegram notification initialized")
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

func (this *Platform) WrapToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, entity.NotifPlatformContext{}, this)
}
