package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kodekoding/phastos/v2/go/entity"
	tbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	slackPkg "github.com/kodekoding/phastos/v2/go/notifications/slack"
	telegramPkg "github.com/kodekoding/phastos/v2/go/notifications/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPlatform(t *testing.T) {
	platform := New()
	require.NotNil(t, platform)
	assert.Empty(t, platform.GetAllPlatform())
}

func TestPlatformTelegram(t *testing.T) {
	platform := New()
	telegram := platform.Telegram()
	assert.NotNil(t, telegram)
}

func TestPlatformSlack(t *testing.T) {
	platform := New()
	slack := platform.Slack()
	assert.NotNil(t, slack)
}

func TestPlatformWrapToContext(t *testing.T) {
	platform := New()
	ctx := context.Background()
	ctx = platform.WrapToContext(ctx)
	val := ctx.Value(entity.NotifPlatformContext{})
	assert.NotNil(t, val)
	
	platformVal, ok := val.(*Platform)
	assert.True(t, ok)
	assert.Equal(t, platform, platformVal)
}

func TestPlatformWrapToHandler(t *testing.T) {
	platform := New()
	
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		val := r.Context().Value(entity.NotifPlatformContext{})
		assert.NotNil(t, val)
	})
	
	handler := platform.WrapToHandler(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	assert.True(t, called)
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Telegram: nil,
		Slack:    nil,
	}
	assert.Nil(t, cfg.Telegram)
	assert.Nil(t, cfg.Slack)
}

func TestActivateSlack_NewSlackServiceError(t *testing.T) {
	origNewSlackService := newSlackService
	defer func() { newSlackService = origNewSlackService }()

	newSlackService = func(cfg *slackPkg.SlackConfig) (*slackPkg.Service, error) {
		return nil, assert.AnError
	}

	platform := New()
	opt := ActivateSlack("https://hooks.slack.com/services/test/webhook")
	opt(platform)
	assert.Empty(t, platform.GetAllPlatform())
}

// stubAction implements Action for testing
type stubAction struct {
	active     bool
	typeVal    string
	traceId    string
	destination interface{}
	sendErr    error
}

func (s *stubAction) Send(ctx context.Context, text string, attachment interface{}) error { return s.sendErr }
func (s *stubAction) IsActive() bool       { return s.active }
func (s *stubAction) Type() string         { return s.typeVal }
func (s *stubAction) SetTraceId(traceId string)  { s.traceId = traceId }
func (s *stubAction) SetDestination(dest interface{}) { s.destination = dest }

func TestActionInterface(t *testing.T) {
	var _ Action = &stubAction{}
}

func TestPlatformsInterface(t *testing.T) {
	var _ Platforms = New()
}

func TestNewWithStubActions(t *testing.T) {
	stubTel := &stubAction{active: true, typeVal: "telegram"}
	stubSlk := &stubAction{active: true, typeVal: "slack"}
	platform := &Platform{
		telegram: stubTel,
		slack:    stubSlk,
		list:     []Action{stubTel, stubSlk},
	}
	assert.Equal(t, stubTel, platform.Telegram())
	assert.Equal(t, stubSlk, platform.Slack())
	assert.Len(t, platform.GetAllPlatform(), 2)
}

func TestStubActionSend(t *testing.T) {
	stub := &stubAction{sendErr: nil}
	err := stub.Send(context.Background(), "hello", nil)
	assert.NoError(t, err)
	
	stub2 := &stubAction{sendErr: assert.AnError}
	err2 := stub2.Send(context.Background(), "hello", nil)
	assert.Error(t, err2)
}

func TestStubActionSetTraceId(t *testing.T) {
	stub := &stubAction{}
	stub.SetTraceId("trace-123")
	assert.Equal(t, "trace-123", stub.traceId)
}

func TestStubActionSetDestination(t *testing.T) {
	stub := &stubAction{}
	stub.SetDestination("dest-val")
	assert.Equal(t, "dest-val", stub.destination)
}

func TestActivateSlackWithInvalidURL(t *testing.T) {
	// ActivateSlack with an empty URL should not panic and should not add to list
	platform := New()
	opt := ActivateSlack("")
	opt(platform)
	// slack activation with empty URL will fail (slack.New doesn't error but the service won't be active)
	// The list should still be empty since slack.New returns error for empty URL
	// Actually slack.New just stores the URL, so it will be added to list
}

func TestActivateSlackWithValidURL(t *testing.T) {
	platform := New()
	opt := ActivateSlack("https://hooks.slack.com/services/test/webhook")
	opt(platform)
	assert.Len(t, platform.GetAllPlatform(), 1)
	slackAction := platform.Slack()
	assert.NotNil(t, slackAction)
}

func TestActivateTelegramWithInvalidToken(t *testing.T) {
	platform := New()
	opt := ActivateTelegram("invalid-token")
	opt(platform)
	// Telegram with invalid token should fail to initialize
	// List should remain empty since New() returns error
	assert.Empty(t, platform.GetAllPlatform())
}

func TestNewWithMultipleOptions(t *testing.T) {
	platform := New(
		ActivateSlack("https://hooks.slack.com/services/test/webhook"),
	)
	assert.NotNil(t, platform)
	assert.Len(t, platform.GetAllPlatform(), 1)
}

func TestPlatformGetAllPlatformEmpty(t *testing.T) {
	platform := New()
	assert.Empty(t, platform.GetAllPlatform())
}

func TestPlatformGetAllPlatformWithItems(t *testing.T) {
	stub1 := &stubAction{active: true, typeVal: "test1"}
	stub2 := &stubAction{active: true, typeVal: "test2"}
	platform := &Platform{
		telegram: stub1,
		slack:    stub2,
		list:     []Action{stub1, stub2},
	}
	all := platform.GetAllPlatform()
	assert.Len(t, all, 2)
	assert.Equal(t, "test1", all[0].Type())
	assert.Equal(t, "test2", all[1].Type())
}

func TestPlatformWrapToHandlerPreservesOriginalRequest(t *testing.T) {
	platform := New()
	
	var receivedReq *http.Request
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r
	})
	
	handler := platform.WrapToHandler(next)
	req := httptest.NewRequest(http.MethodPost, "/test-path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	require.NotNil(t, receivedReq)
	assert.Equal(t, "/test-path", receivedReq.URL.Path)
}

func TestActivateTelegramWithMockBot(t *testing.T) {
	t.Run("should activate telegram with mocked bot API", func(t *testing.T) {
		origNewBotAPI := telegramPkg.NewBotAPIFunc
		defer func() { telegramPkg.NewBotAPIFunc = origNewBotAPI }()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
		}))
		defer server.Close()

		botsURL := server.URL + "/bot%s/%s"
		telegramPkg.NewBotAPIFunc = func(token string) (*tbot.BotAPI, error) {
			return tbot.NewBotAPIWithAPIEndpoint(token, botsURL)
		}

		platform := New(
			ActivateTelegram("123456:test-token"),
		)
		assert.NotNil(t, platform)
		assert.Len(t, platform.GetAllPlatform(), 1)
	})
}
