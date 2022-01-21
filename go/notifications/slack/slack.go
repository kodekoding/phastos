package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	sgw "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/kodekoding/phastos/go/helper"
)

const strErrDesc = "Error Desciption"

// Service structure
type (
	Service struct {
		url        string
		attachment *sgw.Attachment
		message    string
		traceID    string
		recipient  string
		isActive   bool
	}

	SlackConfig struct {
		IsActive bool   `yaml:"is_active"`
		URL      string `yaml:"webhook_url"`
	}
)

var sendSlack = sgw.Send

// New instance of Slack Service
func New(cfg *SlackConfig) (*Service, error) {
	return &Service{
		url:      cfg.URL,
		isActive: cfg.IsActive,
	}, nil
}

// Send - Post to Slack
func (p *Service) Send(_ context.Context, text string, attachment interface{}) error {
	slackAttachment, valid := attachment.(*sgw.Attachment)
	if !valid {
		return errors.New("attachment must be slack-go-webhook attachment struct")
	}
	p.attachment = slackAttachment

	users := "<!here>"

	slackMessage := "there is an error:"
	if text != "" {
		slackMessage = text
	}

	if p.traceID == "" {
		p.traceID = helper.GenerateUUIDV4()
	}

	if p.recipient != "" {
		users = p.recipient
	}

	payload := sgw.Payload{
		Text: fmt.Sprintf("Hallo %s %s", users, slackMessage),
	}
	if p.attachment != nil {
		p.attachment.AddField(sgw.Field{
			Title: "Slack Trace ID",
			Value: p.traceID,
			Short: true,
		})
		attachments := []sgw.Attachment{}

		attachments = append(attachments, *p.attachment)
		payload.Attachments = attachments
	}
	err := sendSlack(p.url, "", payload)
	if len(err) > 0 {
		errStr := []string{}
		for _, errVal := range err {
			errStr = append(errStr, errVal.Error())
		}
		errStrJoin := strings.Join(errStr, ",")
		errNewStr := errors.New(errStrJoin)
		return errors.Wrap(errNewStr, "error when send slack")
	}
	return nil
}

func (s *Service) IsActive() bool {
	return s.isActive
}
