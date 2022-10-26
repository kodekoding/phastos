package mail

import (
	"github.com/keighl/mandrill"
	"github.com/pkg/errors"
)

type (
	Mandrills interface {
		AddRecipient(recipientEmail, recipientName string) Mandrills
		AddAttachment(attachment *mandrill.Attachment) Mandrills
		SetHTMLContent(subject, htmlContent string) Mandrills
		SetTextContent(subject, textContent string) Mandrills
		SetGlobalMergeVars(data map[string]interface{}) Mandrills
		SetTemplate(templateName string, templateContent map[string]string) Mandrills
		SetEmailFrom(emailFrom string) Mandrills
		SetFromName(fromName string) Mandrills
		Send() error
	}

	Mandrill struct {
		client           *mandrill.Client
		recipient        []string
		message          *mandrill.Message
		templateName     string
		templateContent  map[string]string
		defaultEmailFrom string
		defaultFromName  string
		*MailConfig
	}
)

func NewMandrill(opts *MailConfig) Mandrills {
	obj := &Mandrill{MailConfig: opts}
	obj.defaultEmailFrom = opts.EmailFrom
	obj.defaultFromName = opts.FromName
	obj.reset()
	return obj
}

func (m *Mandrill) reset() {

	m.client = mandrill.ClientWithKey(m.SecretKey)
	msg := &mandrill.Message{}
	msg.FromEmail = m.defaultEmailFrom
	msg.FromName = m.defaultFromName
	msg.To = nil
	m.templateName = ""
	m.templateContent = nil
	m.message = msg
}

func (m *Mandrill) AddRecipient(recipientEmail, recipientName string) Mandrills {
	m.message.AddRecipient(recipientEmail, recipientName, "to")
	return m
}

func (m *Mandrill) SetEmailFrom(emailFrom string) Mandrills {
	m.message.FromEmail = emailFrom
	return m
}

func (m *Mandrill) SetFromName(fromName string) Mandrills {
	m.message.FromName = fromName
	return m
}

func (m *Mandrill) AddAttachment(attachment *mandrill.Attachment) Mandrills {
	m.message.Attachments = append(m.message.Attachments, attachment)
	return m
}

func (m *Mandrill) SetHTMLContent(subject, htmlContent string) Mandrills {
	m.message.Subject = subject
	m.message.HTML = htmlContent
	return m
}

func (m *Mandrill) SetGlobalMergeVars(data map[string]interface{}) Mandrills {
	m.message.GlobalMergeVars = mandrill.MapToVars(data)
	return m
}

func (m *Mandrill) SetTemplate(templateName string, templateContent map[string]string) Mandrills {
	m.templateContent = templateContent
	m.templateName = templateName
	return m
}

func (m *Mandrill) SetTextContent(subject, textContent string) Mandrills {
	m.message.Subject = subject
	m.message.Text = textContent
	return m
}

func (m *Mandrill) Send() error {
	var err error
	if m.templateName == "" {
		_, err = m.client.MessagesSend(m.message)
	} else {
		_, err = m.client.MessagesSendTemplate(m.message, m.templateName, m.templateContent)
	}

	if err != nil {
		return errors.Wrap(err, "phastos.go.mail.mandrill.SendEmail")
	}

	m.reset()
	return nil
}
