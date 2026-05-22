package mail

import (
	"embed"
	"fmt"
	"net/smtp"
	"testing"

	"github.com/keighl/mandrill"
	"github.com/stretchr/testify/assert"
)

// --- Mandrill tests ---

func TestNewMandrill(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	assert.NotNil(t, m)

	// Verify it's a Mandrill struct
	mr, ok := m.(*Mandrill)
	assert.True(t, ok)
	assert.Equal(t, "from@example.com", mr.message.FromEmail)
	assert.Equal(t, "Test Sender", mr.message.FromName)
}

func TestMandrillAddRecipient(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	result := m.AddRecipient("user@example.com", "User Name")
	assert.NotNil(t, result)
	assert.Equal(t, m, result)
}

func TestMandrillSetEmailFrom(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "original@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	result := m.SetEmailFrom("new@example.com")
	assert.NotNil(t, result)

	mr := m.(*Mandrill)
	assert.Equal(t, "new@example.com", mr.message.FromEmail)
}

func TestMandrillSetFromName(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Original Name",
	}
	m := NewMandrill(cfg)
	result := m.SetFromName("New Name")
	assert.NotNil(t, result)

	mr := m.(*Mandrill)
	assert.Equal(t, "New Name", mr.message.FromName)
}

func TestMandrillAddAttachment(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	attachment := &mandrill.Attachment{
		Type:    "text/plain",
		Name:    "test.txt",
		Content: "dGVzdA==",
	}
	result := m.AddAttachment(attachment)
	assert.NotNil(t, result)

	mr := m.(*Mandrill)
	assert.Len(t, mr.message.Attachments, 1)
}

func TestMandrillSetHTMLContent(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	result := m.SetHTMLContent("Test Subject", "<h1>Hello</h1>")
	assert.NotNil(t, result)

	mr := m.(*Mandrill)
	assert.Equal(t, "Test Subject", mr.message.Subject)
	assert.Equal(t, "<h1>Hello</h1>", mr.message.HTML)
}

func TestMandrillSetTextContent(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	result := m.SetTextContent("Text Subject", "Plain text body")
	assert.NotNil(t, result)

	mr := m.(*Mandrill)
	assert.Equal(t, "Text Subject", mr.message.Subject)
	assert.Equal(t, "Plain text body", mr.message.Text)
}

func TestMandrillSetGlobalMergeVars(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	data := map[string]interface{}{
		"name": "John",
		"age":  30,
	}
	result := m.SetGlobalMergeVars(data)
	assert.NotNil(t, result)
}

func TestMandrillSetTemplate(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	templateContent := map[string]string{"key": "value"}
	result := m.SetTemplate("welcome-template", templateContent)
	assert.NotNil(t, result)

	mr := m.(*Mandrill)
	assert.Equal(t, "welcome-template", mr.templateName)
	assert.Equal(t, templateContent, mr.templateContent)
}

func TestMandrillSendWithoutTemplate(t *testing.T) {
	cfg := &Config{
		SecretKey: "invalid-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	m.AddRecipient("user@example.com", "User")
	m.SetHTMLContent("Test", "<p>Hello</p>")

	err := m.Send()
	// Should fail with invalid key
	assert.Error(t, err)
}

func TestMandrillSendWithTemplate(t *testing.T) {
	cfg := &Config{
		SecretKey: "invalid-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	m.AddRecipient("user@example.com", "User")
	m.SetHTMLContent("Test", "<p>Hello</p>")
	m.SetTemplate("test-template", map[string]string{"key": "value"})

	err := m.Send()
	// Should fail with invalid key
	assert.Error(t, err)
}

func TestMandrillReset(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-secret-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)

	mr := m.(*Mandrill)
	mr.SetHTMLContent("Subject", "Content")
	mr.reset()

	assert.Equal(t, "", mr.templateName)
	assert.Nil(t, mr.templateContent)
}

func TestMandrillsInterface(t *testing.T) {
	cfg := &Config{
		SecretKey: "test-key",
		EmailFrom: "from@example.com",
		FromName:  "Test",
	}
	var _ Mandrills = NewMandrill(cfg)
}

func TestSMTPsInterface(t *testing.T) {
	var _ SMTPs = &SMTP{}
}

// --- SMTP additional tests ---

//go:embed testdata
var testTemplates embed.FS

func TestSMTPSetHTMLTemplateNoSender(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
	)
	smtp.AddRecipient("user@example.com")
	smtp.SetHTMLTemplate(testTemplates, "test.html", "Test Subject", nil)
	assert.Error(t, smtp.err)
	assert.Contains(t, smtp.err.Error(), "sender name or recipient must be filled")
}

func TestSMTPSetHTMLTemplateNoRecipient(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.SetHTMLTemplate(testTemplates, "test.html", "Test Subject", nil)
	assert.Error(t, smtp.err)
}

func TestSMTPSetHTMLTemplateFromPathNoSender(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
	)
	smtp.AddRecipient("user@example.com")
	smtp.SetHTMLTemplateFromPath("nonexistent.html", "Test Subject", nil)
	assert.Error(t, smtp.err)
	assert.Contains(t, smtp.err.Error(), "sender name or recipient must be filled")
}

func TestSMTPSetHTMLTemplateFromPathNoRecipient(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.SetHTMLTemplateFromPath("nonexistent.html", "Test Subject", nil)
	assert.Error(t, smtp.err)
}

func TestSMTPSetHTMLTemplateInvalidFile(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.AddRecipient("user@example.com")
	smtp.SetHTMLTemplate(testTemplates, "nonexistent.html", "Test Subject", nil)
	assert.Error(t, smtp.err)
}

func TestSMTPSetHTMLTemplateFromPathInvalidFile(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.AddRecipient("user@example.com")
	smtp.SetHTMLTemplateFromPath("nonexistent.html", "Test Subject", nil)
	assert.Error(t, smtp.err)
}

func TestSMTPSendNoConfig(t *testing.T) {
	smtp := NewSMTP()
	smtp.AddRecipient("user@example.com")
	smtp.SetContent("Test", "Body")
	// Send will fail because there's no real SMTP server
	err := smtp.Send()
	assert.Error(t, err)
}

func TestSMTPSetHTMLTemplateSuccess(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.AddRecipient("user@example.com")
	smtp.SetHTMLTemplate(testTemplates, "testdata/test.html", "Test Subject", map[string]string{"Name": "World"})
	assert.NoError(t, smtp.err)
	assert.Contains(t, smtp.body.String(), "Test Sender")
	assert.Contains(t, smtp.body.String(), "user@example.com")
}

func TestMandrillSendSuccess_NoTemplate(t *testing.T) {
	originalSendFunc := messagesSendFunc
	defer func() { messagesSendFunc = originalSendFunc }()

	messagesSendFunc = func(c *mandrill.Client, msg *mandrill.Message) ([]*mandrill.Response, error) {
		return []*mandrill.Response{{}}, nil
	}

	cfg := &Config{
		SecretKey: "test-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	m.AddRecipient("user@example.com", "User")
	m.SetHTMLContent("Test", "<p>Hello</p>")

	err := m.Send()
	assert.NoError(t, err)
}

func TestMandrillSendSuccess_WithTemplate(t *testing.T) {
	originalSendTemplateFunc := messagesSendTemplateFunc
	defer func() { messagesSendTemplateFunc = originalSendTemplateFunc }()

	messagesSendTemplateFunc = func(c *mandrill.Client, msg *mandrill.Message, templateName string, templateContent map[string]string) ([]*mandrill.Response, error) {
		return []*mandrill.Response{{}}, nil
	}

	cfg := &Config{
		SecretKey: "test-key",
		EmailFrom: "from@example.com",
		FromName:  "Test Sender",
	}
	m := NewMandrill(cfg)
	m.AddRecipient("user@example.com", "User")
	m.SetHTMLContent("Test", "<p>Hello</p>")
	m.SetTemplate("test-template", map[string]string{"key": "value"})

	err := m.Send()
	assert.NoError(t, err)
}

func TestSMTPSendSuccess(t *testing.T) {
	originalSendMail := sendMailFunc
	defer func() { sendMailFunc = originalSendMail }()

	sendMailFunc = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		return nil
	}

	smtpSvc := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtpSvc.AddRecipient("user@example.com")
	smtpSvc.SetContent("Subject", "Body")
	err := smtpSvc.Send()
	assert.NoError(t, err)
}

func TestSMTPSendError(t *testing.T) {
	originalSendMail := sendMailFunc
	defer func() { sendMailFunc = originalSendMail }()

	sendMailFunc = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		return fmt.Errorf("smtp send error")
	}

	smtpSvc := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtpSvc.AddRecipient("user@example.com")
	smtpSvc.SetContent("Subject", "Body")
	err := smtpSvc.Send()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "smtp send error")
}

func TestSMTPChaining(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.AddRecipient("user1@example.com").AddRecipient("user2@example.com")
	assert.Len(t, smtp.recipient, 2)

	smtp.SetSingleRecipient("only@example.com")
	assert.Len(t, smtp.recipient, 1)

	smtp.SetContent("Subject", "Message")
	assert.NoError(t, smtp.err)
}
