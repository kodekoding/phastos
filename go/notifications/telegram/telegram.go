package telegram

import (
	"context"

	tbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/monitoring"
)

type (
	Service struct {
		chatId        int64
		defaultChatId int64
		bot           *tbot.BotAPI
		isActive      bool
		traceId       string
	}

	TelegramConfig struct {
		IsActive bool   `yaml:"is_active"`
		BotToken string `yaml:"bot_token"`
		ChatId   int64  `yaml:"chat_id"`
	}
)

func (s *Service) SetDestination(destination interface{}) {
	s.chatId = destination.(int64)
}

func (s *Service) resetChatId() {
	s.chatId = s.defaultChatId
}

func (s *Service) Type() string {
	return "telegram"
}

func (s *Service) SetTraceId(traceId string) {
	s.traceId = traceId
}

func New(cfg *TelegramConfig) (*Service, error) {
	bot, err := tbot.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, errors.Wrap(err, "pkg.notificications.telegram.NewBot")
	}
	return &Service{
		chatId:        cfg.ChatId,
		defaultChatId: cfg.ChatId,
		bot:           bot,
		isActive:      cfg.IsActive,
	}, nil
}

func (s *Service) Send(ctx context.Context, text string, attachment interface{}) error {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		defer txn.StartSegment("Notification-Slack-Telegram").End()
	}
	defer s.resetChatId()
	newMessage := tbot.NewMessage(s.chatId, text)
	if _, err := s.bot.Send(newMessage); err != nil {
		return errors.Wrap(err, "pkg.notifications.telegram.Send")
	}
	return nil
}

func (s *Service) IsActive() bool {
	return s.isActive
}
