package mail

import (
	"github.com/keighl/mandrill"
	"github.com/pkg/errors"
)

type (
	Mandrills interface {
		AddRecipient(recipientEmail, recipientName string) Mandrills
		SetHTMLContent(subject, htmlContent string) Mandrills
		SetTextContent(subject, textContent string) Mandrills
		Send() error
	}

	Mandrill struct {
		client    *mandrill.Client
		recipient []string
		message   *mandrill.Message
		*MailConfig
	}
)

func NewMandrill(opts *MailConfig) Mandrills {
	obj := &Mandrill{MailConfig: opts}
	obj.reset()
	return obj
}

func (m *Mandrill) reset() {

	m.client = mandrill.ClientWithKey(m.SecretKey)
	msg := &mandrill.Message{}
	msg.FromEmail = m.EmailFrom
	msg.FromName = m.FromName

	msg.To = nil
	m.message = msg
}

func (m *Mandrill) AddRecipient(recipientEmail, recipientName string) Mandrills {
	m.message.AddRecipient(recipientEmail, recipientName, "to")
	return m
}

func (m *Mandrill) SetHTMLContent(subject, htmlContent string) Mandrills {
	m.message.Subject = subject
	m.message.HTML = htmlContent
	return m
}

func (m *Mandrill) SetTextContent(subject, textContent string) Mandrills {
	m.message.Subject = subject
	m.message.Text = textContent
	return m
}

func (m *Mandrill) Send() error {
	_, err := m.client.MessagesSend(m.message)
	if err != nil {
		return errors.Wrap(err, "phastos.go.mail.mandrill.SendEmail")
	}

	m.reset()
	return nil
}
