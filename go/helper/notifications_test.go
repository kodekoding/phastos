package helper

import (
	"context"
	"net/http"
	"testing"

	ctx2 "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/notifications"
	"github.com/stretchr/testify/assert"
)

func TestNotifMsgType(t *testing.T) {
	t.Run("should set msgType on param", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifMsgType(NotifInfoType)
		opt(param)
		assert.Equal(t, NotifInfoType, param.msgType)
	})

	t.Run("should set warn type", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifMsgType(NotifWarnType)
		opt(param)
		assert.Equal(t, NotifWarnType, param.msgType)
	})

	t.Run("should set error type", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifMsgType(NotifErrorType)
		opt(param)
		assert.Equal(t, NotifErrorType, param.msgType)
	})
}

func TestNotifData(t *testing.T) {
	t.Run("should set data on param", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		param := new(sentNotifParam)
		opt := NotifData(data)
		opt(param)
		assert.Equal(t, data, param.data)
	})

	t.Run("should handle nil data", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifData(nil)
		opt(param)
		assert.Nil(t, param.data)
	})

	t.Run("should handle empty data", func(t *testing.T) {
		data := map[string]string{}
		param := new(sentNotifParam)
		opt := NotifData(data)
		opt(param)
		assert.Empty(t, param.data)
	})
}

func TestNotifTitle(t *testing.T) {
	t.Run("should set title on param", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifTitle("Alert Title")
		opt(param)
		assert.Equal(t, "Alert Title", param.titleMsg)
	})

	t.Run("should handle empty title", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifTitle("")
		opt(param)
		assert.Equal(t, "", param.titleMsg)
	})
}

func TestNotifChannel(t *testing.T) {
	t.Run("should set channel on param", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifChannel("#general")
		opt(param)
		assert.Equal(t, "#general", param.channel)
	})

	t.Run("should handle empty channel", func(t *testing.T) {
		param := new(sentNotifParam)
		opt := NotifChannel("")
		opt(param)
		assert.Equal(t, "", param.channel)
	})
}

func TestNotifConstants(t *testing.T) {
	t.Run("should have correct constant values", func(t *testing.T) {
		assert.Equal(t, "info", NotifInfoType)
		assert.Equal(t, "warn", NotifWarnType)
		assert.Equal(t, "error", NotifErrorType)
	})
}

func TestSendSlackNotification_NoNotifContext(t *testing.T) {
	t.Run("should return nil when no notif in context", func(t *testing.T) {
		ctx := context.Background()
		err := SendSlackNotification(ctx)
		assert.NoError(t, err)
	})

	t.Run("should return nil with options when no notif in context", func(t *testing.T) {
		ctx := context.Background()
		err := SendSlackNotification(ctx,
			NotifMsgType(NotifInfoType),
			NotifTitle("Test"),
			NotifChannel("#test"),
			NotifData(map[string]string{"key": "val"}),
		)
		assert.NoError(t, err)
	})
}

func TestSendSlackNotification_WithNilNotif(t *testing.T) {
	t.Run("should return nil when notif context returns nil", func(t *testing.T) {
		// Use a context where GetNotif returns nil
		ctx := context.Background()
		err := SendSlackNotification(ctx, NotifMsgType(NotifErrorType))
		assert.NoError(t, err)
	})
}

func TestSendSlackNotification_OptionsComposition(t *testing.T) {
	t.Run("should apply all options to param", func(t *testing.T) {
		param := new(sentNotifParam)
		opts := []SentNotifParamOptions{
			NotifMsgType(NotifWarnType),
			NotifData(map[string]string{"env": "prod"}),
			NotifTitle("Warning Alert"),
			NotifChannel("#alerts"),
		}
		for _, opt := range opts {
			opt(param)
		}

		assert.Equal(t, NotifWarnType, param.msgType)
		assert.Equal(t, "Warning Alert", param.titleMsg)
		assert.Equal(t, "#alerts", param.channel)
		assert.Equal(t, map[string]string{"env": "prod"}, param.data)
	})
}

// ---- Stub for testing active notification context ----

// stubAction implements notifications.Action for testing
type stubAction struct {
	active      bool
	typeVal     string
	traceId     string
	destination interface{}
	sendErr     error
}

func (s *stubAction) Send(ctx context.Context, text string, attachment interface{}) error {
	return s.sendErr
}
func (s *stubAction) IsActive() bool       { return s.active }
func (s *stubAction) Type() string         { return s.typeVal }
func (s *stubAction) SetTraceId(traceId string) { s.traceId = traceId }
func (s *stubAction) SetDestination(dest interface{}) { s.destination = dest }

// stubPlatform implements notifications.Platforms for testing
type stubPlatform struct {
	slack *stubAction
}

func (p *stubPlatform) Telegram() notifications.Action { return nil }
func (p *stubPlatform) Slack() notifications.Action    { return p.slack }
func (p *stubPlatform) FCM() notifications.Action      { return nil }
func (p *stubPlatform) GetAllPlatform() []notifications.Action { return nil }
func (p *stubPlatform) WrapToHandler(next http.Handler) http.Handler { return next }
func (p *stubPlatform) WrapToContext(ctx context.Context) context.Context { return ctx }

// ---- Tests for SendSlackNotification with active context ----

func TestSendSlackNotification_ActiveContext(t *testing.T) {
	t.Run("should send notification when slack is active with nil send err", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: nil}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifInfoType),
			NotifTitle("Test Alert"),
			NotifData(map[string]string{"key": "value"}),
		)
		assert.NoError(t, err)
	})

	t.Run("should return error when slack Send fails", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: assert.AnError}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifErrorType),
			NotifTitle("Error Alert"),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error when sent")
		assert.Contains(t, err.Error(), "slack")
	})

	t.Run("should not send when slack is not active", func(t *testing.T) {
		slack := &stubAction{active: false, typeVal: "slack"}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifInfoType),
			NotifTitle("Test"),
		)
		assert.NoError(t, err)
	})

	t.Run("should handle warn message type", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: nil}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifWarnType),
			NotifTitle("Warning"),
		)
		assert.NoError(t, err)
	})

	t.Run("should handle info message type", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: nil}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifInfoType),
			NotifTitle("Info"),
		)
		assert.NoError(t, err)
	})

	t.Run("should handle error message type", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: nil}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifErrorType),
			NotifTitle("Error"),
		)
		assert.NoError(t, err)
	})

	t.Run("should handle default message type", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: nil}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifTitle("Default"),
		)
		assert.NoError(t, err)
	})

	t.Run("should set destination when channel is provided", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: nil}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifInfoType),
			NotifTitle("Test"),
			NotifChannel("#alerts"),
		)
		assert.NoError(t, err)
		assert.Equal(t, "#alerts", slack.destination)
	})

	t.Run("should handle data with keys starting with dash", func(t *testing.T) {
		slack := &stubAction{active: true, typeVal: "slack", sendErr: nil}
		platform := &stubPlatform{slack: slack}

		req, _ := http.NewRequest("GET", "/", nil)
		ctx2.SetNotif(req, platform)
		ctx := req.Context()

		err := SendSlackNotification(ctx,
			NotifMsgType(NotifInfoType),
			NotifTitle("Test"),
			NotifData(map[string]string{"-long_key": "value1", "short_key": "value2"}),
		)
		assert.NoError(t, err)
	})

	t.Run("should return nil when GetNotif returns nil", func(t *testing.T) {
		// Context with nil GetNotif (wrong type)
		ctx := context.WithValue(context.Background(), struct{}{}, "not a platform")
		err := SendSlackNotification(ctx, NotifMsgType(NotifInfoType))
		assert.NoError(t, err)
	})
}
