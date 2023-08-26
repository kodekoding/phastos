package mail

import (
	"bytes"
	"embed"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/helper"
)

type (
	SMTPs interface {
		AddRecipient(recipient ...string) *SMTP
		SetContent(subject, message string) *SMTP
		SetHTMLTemplate(fs embed.FS, tplFile, subject string, args interface{}) *SMTP
		SetSingleRecipient(recipient string) *SMTP
		Send() error
	}

	SMTP struct {
		auth smtp.Auth
		SMTPConfig
		err error
	}

	Config struct {
		Sender        string `yaml:"sender"`
		EmailUsername string `yaml:"username"`
		EmailPassword string `yaml:"password"`
		SecretKey     string `yaml:"secret_key"`
		EmailFrom     string `yaml:"from"`
		FromName      string `yaml:"from_name"`
		Host          string `yaml:"host"`
		Port          int    `yaml:"port"`
	}

	SMTPConfig struct {
		Config
		recipient []string
		address   string
		body      bytes.Buffer
	}
)

func NewSMTP(cfg *SMTPConfig) *SMTP {
	auth := smtp.PlainAuth("", cfg.EmailUsername, cfg.EmailPassword, cfg.Host)
	cfg.address = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return &SMTP{
		SMTPConfig: *cfg,
		auth:       auth,
	}
}

func (s *SMTP) reset() {
	s.recipient = nil
	s.body.Reset()
}

func (s *SMTP) AddRecipient(recipient ...string) *SMTP {
	s.recipient = append(s.recipient, recipient...)
	return s
}

func (s *SMTP) SetSingleRecipient(recipient string) *SMTP {
	s.recipient = []string{recipient}
	return s
}

func (s *SMTP) SetContent(subject, message string) *SMTP {
	if s.Sender == "" || s.recipient == nil {
		s.err = errors.New("sender name or recipient must be filled")
		return s
	}
	s.body.Write([]byte(fmt.Sprintf(`
		MIME-version: 1.0;
		Content-Type: text/html; charset="UTF-8";
		From: %s
		To: %s
		Subject: %s

		%s
	`, s.Sender, strings.Join(s.recipient, ","), subject, message)))
	return s
}

func (s *SMTP) SetHTMLTemplate(fs embed.FS, tplFile, subject string, args interface{}) *SMTP {
	if s.Sender == "" || s.recipient == nil {
		s.err = errors.New("sender name or recipient must be filled")
		return s
	}

	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	additionalBody := fmt.Sprintf("From: %s\nTo:%s\nSubject:%s \n%s\n\n", s.Sender, strings.Join(s.recipient, ","), subject, mimeHeaders)

	s.body, _ = helper.ParseTemplate(fs, tplFile, args, additionalBody)

	return s
}

func (s *SMTP) Send() error {
	defer s.reset()
	if s.err != nil {
		return s.err
	}

	if err := smtp.SendMail(s.address, s.auth, s.EmailFrom, s.recipient, s.body.Bytes()); err != nil {
		return err
	}

	return nil
}
