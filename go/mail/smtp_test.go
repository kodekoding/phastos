package mail

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSMTPDefault(t *testing.T) {
	smtp := NewSMTP()
	assert.NotNil(t, smtp)
	assert.Equal(t, "", smtp.EmailUsername)
	assert.Equal(t, "", smtp.EmailPassword)
	assert.Equal(t, "", smtp.Host)
	assert.Equal(t, 0, smtp.Port)
}

func TestNewSMTPWithOption(t *testing.T) {
	smtp := NewSMTP(
		WithEmail("test@example.com"),
		WithEmailPassword("password123"),
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Sender Name"),
		WithEmailFrom("from@example.com"),
	)
	assert.NotNil(t, smtp)
	assert.Equal(t, "test@example.com", smtp.EmailUsername)
	assert.Equal(t, "password123", smtp.EmailPassword)
	assert.Equal(t, "smtp.example.com", smtp.Host)
	assert.Equal(t, 587, smtp.Port)
	assert.Equal(t, "Sender Name", smtp.Sender)
	assert.Equal(t, "from@example.com", smtp.EmailFrom)
}

func TestNewSMTPDefaultsEmailFrom(t *testing.T) {
	smtp := NewSMTP(
		WithEmail("user@example.com"),
		WithHost("smtp.example.com"),
		WithPort(587),
	)
	// EmailFrom should default to EmailUsername when not explicitly set
	assert.Equal(t, "user@example.com", smtp.EmailFrom)
}

func TestSMTPAddRecipient(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
	)
	smtp.AddRecipient("user1@example.com", "user2@example.com")
	assert.Len(t, smtp.recipient, 2)
}

func TestSMTPSetSingleRecipient(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
	)
	smtp.AddRecipient("user1@example.com")
	smtp.SetSingleRecipient("only@example.com")
	assert.Len(t, smtp.recipient, 1)
	assert.Equal(t, "only@example.com", smtp.recipient[0])
}

func TestSMTPSetContentNoSender(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
	)
	smtp.SetContent("Test Subject", "Test Body")
	assert.Error(t, smtp.err)
	assert.Contains(t, smtp.err.Error(), "sender name or recipient must be filled")
}

func TestSMTPSetContentNoRecipient(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.SetContent("Test Subject", "Test Body")
	assert.Error(t, smtp.err)
}

func TestSMTPSetContentSuccess(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.AddRecipient("user@example.com")
	smtp.SetContent("Test Subject", "<p>Hello</p>")
	assert.NoError(t, smtp.err)
	assert.Contains(t, smtp.body.String(), "Test Subject")
	assert.Contains(t, smtp.body.String(), "<p>Hello</p>")
}

func TestSMTPReset(t *testing.T) {
	smtp := NewSMTP(
		WithHost("smtp.example.com"),
		WithPort(587),
		WithSender("Test Sender"),
	)
	smtp.AddRecipient("user@example.com")
	smtp.SetContent("Subject", "Body")
	smtp.reset()
	assert.Nil(t, smtp.recipient)
	assert.Equal(t, 0, smtp.body.Len())
}

func TestSMTPSendWithPreviousError(t *testing.T) {
	smtp := NewSMTP()
	smtp.err = assert.AnError
	err := smtp.Send()
	assert.Equal(t, assert.AnError, err)
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Sender:        "Test Sender",
		EmailUsername:  "user@example.com",
		EmailPassword:  "password",
		SecretKey:     "secret",
		EmailFrom:     "from@example.com",
		FromName:      "From Name",
		Host:          "smtp.example.com",
		Port:          587,
	}
	assert.Equal(t, "Test Sender", cfg.Sender)
	assert.Equal(t, "user@example.com", cfg.EmailUsername)
	assert.Equal(t, 587, cfg.Port)
}

func TestSMTPConfigStruct(t *testing.T) {
	cfg := SMTPConfig{
		Config: Config{
			Host: "smtp.example.com",
			Port: 587,
		},
	}
	assert.Equal(t, "smtp.example.com", cfg.Host)
	assert.Equal(t, 587, cfg.Port)
}
