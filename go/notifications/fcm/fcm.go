package fcm

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/pkg/errors"
	"google.golang.org/api/option"

	"github.com/kodekoding/phastos/v2/go/monitoring"
)

type (
	Service struct {
		client    *messaging.Client
		recipient string
		isActive  bool
		traceID   string
	}

	FCMConfig struct {
		IsActive           bool   `yaml:"is_active"`
		ServiceAccountPath string `yaml:"service_account_path"`
	}

	FCMAttachment struct {
		Title string
		Data  map[string]string
	}
)

func New(cfg *FCMConfig) (*Service, error) {
	opt := option.WithCredentialsFile(cfg.ServiceAccountPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, errors.Wrap(err, "fcm.init.firebase.NewApp")
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "fcm.init.app.Messaging")
	}

	return &Service{
		client:   client,
		isActive: cfg.IsActive,
	}, nil
}

func (s *Service) SetDestination(destination interface{}) {
	if dest, ok := destination.(string); ok {
		s.recipient = dest
	}
}

func (s *Service) SetTraceId(traceID string) {
	s.traceID = traceID
}

func (s *Service) IsActive() bool {
	return s.isActive
}

func (s *Service) Type() string {
	return "fcm"
}

func (s *Service) Send(ctx context.Context, text string, attachment interface{}) error {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		defer txn.StartSegment("Notification-FCM-Send").End()
	}

	if !s.isActive {
		return errors.New("fcm service is not active")
	}

	if s.recipient == "" {
		return errors.New("fcm recipient token is required")
	}

	msg := &messaging.Message{
		Token: s.recipient,
		Notification: &messaging.Notification{
			Title: "timetraq",
			Body:  text,
		},
	}

	if attachment != nil {
		fcmAtt, valid := attachment.(*FCMAttachment)
		if valid {
			if fcmAtt.Title != "" {
				msg.Notification.Title = fcmAtt.Title
			}
			msg.Data = fcmAtt.Data
		}
	}

	if s.traceID != "" {
		if msg.Data == nil {
			msg.Data = make(map[string]string)
		}
		msg.Data["trace_id"] = s.traceID
	}

	response, err := s.client.Send(ctx, msg)
	if err != nil {
		return errors.Wrap(err, "fcm.send")
	}

	_ = fmt.Sprintf("fcm message sent: %s", response)
	return nil
}
