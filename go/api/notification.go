package api

import (
	"os"

	"github.com/kodekoding/phastos/v2/go/notifications"
)

func (app *App) loadNotification() {
	slackWebhookURL := os.Getenv("NOTIFICATIONS_SLACK_WEBHOOK_URL")
	var notifOptions []notifications.Options
	if slackWebhookURL != "" {
		notifOptions = append(notifOptions, notifications.ActivateSlack(slackWebhookURL))
	}

	telegramBotToken := os.Getenv("NOTIFICATIONS_TELEGRAM_TOKEN")
	if telegramBotToken != "" {
		notifOptions = append(notifOptions, notifications.ActivateTelegram(telegramBotToken))
	}

	if notifOptions != nil {
		notif := notifications.New(notifOptions...)
		app.WrapToApp(notif)
	}
}
