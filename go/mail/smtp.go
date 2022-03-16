package mail

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/pkg/errors"
)

type (
	SMTPs interface {
		AddRecipient(recipient string) SMTPs
		SetContent(subject, message string) SMTPs
		Send() error
	}

	SMTP struct {
		auth smtp.Auth
		SMTPConfig
		err error
	}

	SMTPConfig struct {
		Sender        string
		EmailUsername string
		EmailPassword string
		EmailFrom     string
		Host          string
		Port          int
		recipient     []string
		message       string
		address       string
	}
)

func NewSMTP(cfg *SMTPConfig) SMTPs {
	auth := smtp.PlainAuth("", cfg.EmailUsername, cfg.EmailPassword, cfg.Host)
	cfg.address = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return &SMTP{
		SMTPConfig: *cfg,
		auth:       auth,
	}
}

func (s *SMTP) AddRecipient(recipient string) SMTPs {
	s.recipient = append(s.recipient, recipient)
	return s
}

func (s *SMTP) SetContent(subject, message string) SMTPs {
	if s.Sender == "" || s.recipient == nil {
		s.err = errors.New("sender name or recipient must be filled")
		return s
	}
	s.message = fmt.Sprintf(`
		MIME-version: 1.0;
		Content-Type: text/html; charset="UTF-8";
		From: %s
		To: %s
		Subject: %s

		%s
	`, s.Sender, strings.Join(s.recipient, ","), subject, message)
	return s
}

func (s *SMTP) Send() error {
	if s.err != nil {
		return s.err
	}

	if err := smtp.SendMail(s.address, s.auth, s.EmailFrom, s.recipient, []byte(s.message)); err != nil {
		return err
	}

	return nil
}
