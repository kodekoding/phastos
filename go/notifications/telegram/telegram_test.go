package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/monitoring"
)

const telegramGetMeResponse = `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Test","username":"testbot"}}`
const telegramSendSuccessResponse = `{"ok":true,"result":{"message_id":1,"date":1234567890,"chat":{"id":12345,"type":"private"}}}`
const telegramSendErrorResponse = `{"ok":false,"error_code":400,"description":"Bad Request"}`

// newMockBot creates a BotAPI pointed at a test server.
// The API endpoint format is: baseURL/bot%s/%s where %s=token, %s=method
func newMockBot(t *testing.T, server *httptest.Server) *tbot.BotAPI {
	t.Helper()
	endpoint := server.URL + "/bot%s/%s"
	bot, err := tbot.NewBotAPIWithAPIEndpoint("123456:ABC-test-token", endpoint)
	require.NoError(t, err)
	return bot
}

func TestTelegramConfigStruct(t *testing.T) {
	cfg := TelegramConfig{
		IsActive: true,
		BotToken: "123456:ABC",
		ChatId:   12345,
	}
	assert.True(t, cfg.IsActive)
	assert.Equal(t, "123456:ABC", cfg.BotToken)
	assert.Equal(t, int64(12345), cfg.ChatId)
}

func TestServiceType(t *testing.T) {
	svc := &Service{}
	assert.Equal(t, "telegram", svc.Type())
}

func TestServiceIsActive(t *testing.T) {
	svc := &Service{isActive: true}
	assert.True(t, svc.IsActive())

	svc2 := &Service{isActive: false}
	assert.False(t, svc2.IsActive())
}

func TestServiceSetTraceId(t *testing.T) {
	svc := &Service{}
	svc.SetTraceId("trace-456")
	assert.Equal(t, "trace-456", svc.traceId)
}

func TestServiceSetDestination(t *testing.T) {
	svc := &Service{}
	svc.SetDestination(int64(99999))
	assert.Equal(t, int64(99999), svc.chatId)
}

func TestServiceResetChatId(t *testing.T) {
	svc := &Service{
		defaultChatId: 12345,
		chatId:        99999,
	}
	svc.resetChatId()
	assert.Equal(t, int64(12345), svc.chatId)
}

func TestNewWithInvalidToken(t *testing.T) {
	svc, err := New(&TelegramConfig{BotToken: "invalid-token", IsActive: true})
	assert.Error(t, err)
	assert.Nil(t, svc)
	assert.Contains(t, err.Error(), "pkg.notificications.telegram.NewBot")
}

func TestNewWithEmptyToken(t *testing.T) {
	svc, err := New(&TelegramConfig{BotToken: "", IsActive: true})
	assert.Error(t, err)
	assert.Nil(t, svc)
}

func TestServiceSendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "sendMessage") {
			_, _ = w.Write([]byte(telegramSendSuccessResponse))
			return
		}
		_, _ = w.Write([]byte(telegramGetMeResponse))
	}))
	defer server.Close()

	svc := &Service{
		chatId:        12345,
		defaultChatId: 12345,
		bot:           newMockBot(t, server),
		isActive:      true,
	}

	err := svc.Send(context.Background(), "hello world", nil)
	require.NoError(t, err)
	assert.Equal(t, svc.defaultChatId, svc.chatId)
}

func TestServiceSendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "sendMessage") {
			_, _ = w.Write([]byte(telegramSendErrorResponse))
			return
		}
		_, _ = w.Write([]byte(telegramGetMeResponse))
	}))
	defer server.Close()

	svc := &Service{
		chatId:        12345,
		defaultChatId: 12345,
		bot:           newMockBot(t, server),
		isActive:      true,
	}

	err := svc.Send(context.Background(), "hello", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pkg.notifications.telegram.Send")
}

func TestServiceSendResetsChatIdAfterSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "sendMessage") {
			_, _ = w.Write([]byte(telegramSendSuccessResponse))
			return
		}
		_, _ = w.Write([]byte(telegramGetMeResponse))
	}))
	defer server.Close()

	svc := &Service{
		chatId:        99999,
		defaultChatId: 12345,
		bot:           newMockBot(t, server),
		isActive:      true,
	}

	err := svc.Send(context.Background(), "test", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), svc.chatId)
}

func TestServiceSendWithModifiedDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "sendMessage") {
			_, _ = w.Write([]byte(telegramSendSuccessResponse))
			return
		}
		_, _ = w.Write([]byte(telegramGetMeResponse))
	}))
	defer server.Close()

	svc := &Service{
		chatId:        12345,
		defaultChatId: 12345,
		bot:           newMockBot(t, server),
		isActive:      true,
	}

	svc.SetDestination(int64(88888))
	assert.Equal(t, int64(88888), svc.chatId)

	err := svc.Send(context.Background(), "test", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), svc.chatId)
}

func TestServiceSendNotActive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "sendMessage") {
			_, _ = w.Write([]byte(telegramSendSuccessResponse))
			return
		}
		_, _ = w.Write([]byte(telegramGetMeResponse))
	}))
	defer server.Close()

	svc := &Service{
		chatId:        12345,
		defaultChatId: 12345,
		bot:           newMockBot(t, server),
		isActive:      false,
	}

	err := svc.Send(context.Background(), "test", nil)
	require.NoError(t, err)
	assert.False(t, svc.IsActive())
}

func TestServiceSendWithMonitoringTransaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "sendMessage") {
			_, _ = w.Write([]byte(telegramSendSuccessResponse))
			return
		}
		_, _ = w.Write([]byte(telegramGetMeResponse))
	}))
	defer server.Close()

	nr := monitoring.InitNewRelic(
		monitoring.WithAppName("test-telegram"),
		monitoring.WithLicenseKey("0123456789012345678901234567890123456789"),
	)
	if nr != nil && nr.GetApp() != nil {
		txn := nr.GetApp().StartTransaction("test")
		ctx := monitoring.NewContext(context.Background(), txn)

		svc := &Service{
			chatId:        12345,
			defaultChatId: 12345,
			bot:           newMockBot(t, server),
			isActive:      true,
		}
		err := svc.Send(ctx, "test", nil)
		assert.NoError(t, err)
	}
}

func TestNewWithMockBot(t *testing.T) {
	t.Run("should create service when newBotAPI succeeds", func(t *testing.T) {
		originalNewBotAPI := NewBotAPIFunc
		defer func() { NewBotAPIFunc = originalNewBotAPI }()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(telegramGetMeResponse))
		}))
		defer server.Close()

		botsURL := server.URL + "/bot%s/%s"
		NewBotAPIFunc = func(token string) (*tbot.BotAPI, error) {
			return tbot.NewBotAPIWithAPIEndpoint(token, botsURL)
		}

		svc, err := New(&TelegramConfig{BotToken: "123456:test-token", IsActive: true, ChatId: 12345})
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.True(t, svc.IsActive())
		assert.Equal(t, int64(12345), svc.chatId)
	})
}
