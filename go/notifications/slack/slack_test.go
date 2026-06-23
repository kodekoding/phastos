package slack

import (
	"context"
	"testing"

	sgw "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewSlackService(t *testing.T) {
	cfg := &SlackConfig{
		URL:      "https://hooks.slack.com/test",
		IsActive: true,
	}
	svc, err := New(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, svc)
	assert.True(t, svc.IsActive())
	assert.Equal(t, "slack", svc.Type())
}

func TestSlackServiceNotActive(t *testing.T) {
	cfg := &SlackConfig{
		URL:      "",
		IsActive: false,
	}
	svc, err := New(cfg)
	assert.NoError(t, err)
	assert.False(t, svc.IsActive())
}

func TestSlackServiceSetTraceId(t *testing.T) {
	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)
	svc.SetTraceId("trace-123")
	assert.Equal(t, "trace-123", svc.traceID)
}

func TestSlackServiceSetDestination(t *testing.T) {
	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)
	svc.SetDestination("https://hooks.slack.com/other")
	assert.Equal(t, "https://hooks.slack.com/other", svc.url)
}

func TestSlackServiceType(t *testing.T) {
	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)
	assert.Equal(t, "slack", svc.Type())
}

func TestSlackConfigStruct(t *testing.T) {
	cfg := SlackConfig{
		IsActive: true,
		URL:      "https://hooks.slack.com/test",
	}
	assert.True(t, cfg.IsActive)
	assert.Equal(t, "https://hooks.slack.com/test", cfg.URL)
}

func TestSlackServiceResetURL(t *testing.T) {
	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)
	svc.url = "https://different-url.com"
	svc.resetURL()
	assert.Equal(t, "https://hooks.slack.com/test", svc.url)
}

func TestSlackServiceSendWithInvalidAttachment(t *testing.T) {
	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	// Send with an attachment that is not sgw.Attachment
	err := svc.Send(context.Background(), "test", "not-an-attachment")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attachment must be slack-go-webhook attachment struct")
}

func TestSlackServiceSendWithValidAttachment(t *testing.T) {
	// Override the sendSlack variable to prevent actual HTTP calls
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil // success
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	attachment := &sgw.Attachment{}
	err := svc.Send(context.Background(), "test message", attachment)
	assert.NoError(t, err)
}

func TestSlackServiceSendNoAttachment(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	err := svc.Send(context.Background(), "test message", nil)
	assert.NoError(t, err)
}

func TestSlackServiceSendWithEmptyText(t *testing.T) {
	originalSend := sendSlack
	var capturedPayload sgw.Payload
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		capturedPayload = payload
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	err := svc.Send(context.Background(), "", nil)
	assert.NoError(t, err)
	assert.Contains(t, capturedPayload.Text, "there is an error:")
}

func TestSlackServiceSendWithCustomText(t *testing.T) {
	originalSend := sendSlack
	var capturedPayload sgw.Payload
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		capturedPayload = payload
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	err := svc.Send(context.Background(), "custom message", nil)
	assert.NoError(t, err)
	assert.Contains(t, capturedPayload.Text, "custom message")
}

func TestSlackServiceSendWithTraceIdFromContext(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	ctx := context.WithValue(context.Background(), "requestId", "ctx-trace-123")
	err := svc.Send(ctx, "test", nil)
	assert.NoError(t, err)
	assert.Equal(t, "ctx-trace-123", svc.traceID)
}

func TestSlackServiceSendWithTraceIdFromContextEmpty(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	// Empty string in context should still generate UUID
	ctx := context.WithValue(context.Background(), "requestId", "")
	err := svc.Send(ctx, "test", nil)
	assert.NoError(t, err)
	// traceID should be a UUID since context value was empty
	assert.NotEmpty(t, svc.traceID)
}

func TestSlackServiceSendWithNonStringRequestId(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	// Non-string value in context should generate UUID
	ctx := context.WithValue(context.Background(), "requestId", 12345)
	err := svc.Send(ctx, "test", nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, svc.traceID)
}

func TestSlackServiceSendWithExistingTraceId(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)
	svc.SetTraceId("pre-set-trace-id")

	err := svc.Send(context.Background(), "test", nil)
	assert.NoError(t, err)
	// Should use the pre-set trace ID, not generate a new one
	assert.Equal(t, "pre-set-trace-id", svc.traceID)
}

func TestSlackServiceSendWithRecipient(t *testing.T) {
	originalSend := sendSlack
	var capturedPayload sgw.Payload
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		capturedPayload = payload
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)
	svc.recipient = "@john"

	err := svc.Send(context.Background(), "test", nil)
	assert.NoError(t, err)
	assert.Contains(t, capturedPayload.Text, "@john")
}

func TestSlackServiceSendMultipleErrors(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return []error{assert.AnError, errors.New("second error")}
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	err := svc.Send(context.Background(), "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error when send slack")
}

func TestSlackServiceSendResetsURLAfterSend(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)
	svc.SetDestination("https://hooks.slack.com/other")
	assert.Equal(t, "https://hooks.slack.com/other", svc.url)

	err := svc.Send(context.Background(), "test", nil)
	assert.NoError(t, err)
	// URL should be reset to default after Send
	assert.Equal(t, "https://hooks.slack.com/test", svc.url)
}

func TestSlackServiceSendWithSlackError(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return []error{assert.AnError}
	}
	defer func() { sendSlack = originalSend }()

	cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
	svc, _ := New(cfg)

	err := svc.Send(context.Background(), "test message", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error when send slack")
}

func TestSlackServiceSendWithMonitoringTransaction(t *testing.T) {
	originalSend := sendSlack
	sendSlack = func(url string, msg string, payload sgw.Payload) []error {
		return nil
	}
	defer func() { sendSlack = originalSend }()

	nr := monitoring.InitNewRelic(
		monitoring.WithAppName("test-slack"),
		monitoring.WithLicenseKey("0123456789012345678901234567890123456789"),
	)
	if nr != nil && nr.GetApp() != nil {
		txn := nr.GetApp().StartTransaction("test")
		ctx := monitoring.NewContext(context.Background(), txn)
		cfg := &SlackConfig{URL: "https://hooks.slack.com/test", IsActive: true}
		svc, _ := New(cfg)
		err := svc.Send(ctx, "test", nil)
		assert.NoError(t, err)
	}
}
