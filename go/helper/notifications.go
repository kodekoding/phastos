package helper

import (
	"context"
	"fmt"
	sgw "github.com/ashwanthkumar/slack-go-webhook"
	context2 "github.com/kodekoding/phastos/v2/go/context"
	"github.com/pkg/errors"
	"strings"
)

type (
	SentNotifParamOptions func(param *sentNotifParam)
	sentNotifParam        struct {
		msgType  string
		data     map[string]string
		channel  string
		titleMsg string
	}
)

const (
	NotifInfoType  = "info"
	NotifWarnType  = "warn"
	NotifErrorType = "error"
)

func NotifMsgType(msgType string) SentNotifParamOptions {
	return func(param *sentNotifParam) {
		param.msgType = msgType
	}
}

func NotifData(data map[string]string) SentNotifParamOptions {
	return func(param *sentNotifParam) {
		param.data = data
	}
}

func NotifTitle(title string) SentNotifParamOptions {
	return func(param *sentNotifParam) {
		param.titleMsg = title
	}
}

func NotifChannel(channel string) SentNotifParamOptions {
	return func(param *sentNotifParam) {
		param.channel = channel
	}
}

func SendSlackNotification(ctx context.Context, options ...SentNotifParamOptions) error {
	optionalParams := new(sentNotifParam)
	for _, opt := range options {
		opt(optionalParams)
	}
	getNotifContext := context2.GetNotif(ctx)
	if getNotifContext != nil {
		notif := getNotifContext.Slack()
		if notif.IsActive() {
			slackAttachment := new(sgw.Attachment)
			color := ""
			iconTitle := ""
			switch optionalParams.msgType {
			case NotifWarnType:
				color = "#f7bf31"
				iconTitle = ":warning: "
			case NotifInfoType:
				color = "#2fe329"
				iconTitle = ":information_source: "
			case NotifErrorType:
				color = "#ff0e0a"
				iconTitle = "::broken_heart:: "
			default:
				color = "#207cf5"
				iconTitle = ":white_check_mark: "
			}
			slackAttachment.Color = &color
			for key, val := range optionalParams.data {
				shortTag := true
				if strings.HasPrefix(key, "-") {
					shortTag = false
					key = key[1:]
				}
				slackAttachment.AddField(sgw.Field{
					Title: key,
					Value: val,
					Short: shortTag,
				})
			}
			if optionalParams.channel != "" {
				notif.SetDestination(optionalParams.channel)
			}

			titleMsg := fmt.Sprintf("%s%s", iconTitle, optionalParams.titleMsg)
			if err := notif.Send(ctx, titleMsg, slackAttachment); err != nil {
				errNotif := errors.New(fmt.Sprintf("error when sent %s notifications: %s", notif.Type(), err.Error()))
				return errors.Wrap(errNotif, "phastos.helper.notifications.SendSlackNotification.SendProcess")
			}
		}
	}
	return nil
}
