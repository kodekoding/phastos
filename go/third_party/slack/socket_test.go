package slack

import (
	"testing"

	handler2 "github.com/kodekoding/phastos/v2/go/third_party/slack/handler"
	"github.com/stretchr/testify/assert"
)

func TestNewSlackAppInvalidAppToken(t *testing.T) {
	app, err := NewSlackApp("invalid-token", "xoxb-valid-bot-token")
	assert.Error(t, err)
	assert.Nil(t, app)
	assert.Contains(t, err.Error(), "SLACK_APP_TOKEN must have the prefix \"xapp-\"")
}

func TestNewSlackAppInvalidBotToken(t *testing.T) {
	app, err := NewSlackApp("xapp-valid-app-token", "invalid-bot-token")
	assert.Error(t, err)
	assert.Nil(t, app)
	assert.Contains(t, err.Error(), "SLACK_BOT_TOKEN must have the prefix \"xoxb-\"")
}

func TestNewSlackAppBothTokensInvalid(t *testing.T) {
	app, err := NewSlackApp("invalid", "also-invalid")
	assert.Error(t, err)
	assert.Nil(t, app)
}

func TestAppImplementorStruct(t *testing.T) {
	impl := AppImplementor{}
	assert.NotNil(t, impl)
}

func TestAppStruct(t *testing.T) {
	app := &App{
		botToken:   "xoxb-test",
		appToken:   "xapp-test",
		LoadModule: func() {},
	}
	assert.Equal(t, "xoxb-test", app.botToken)
	assert.Equal(t, "xapp-test", app.appToken)
	assert.NotNil(t, app.LoadModule)
}

func TestAppGetAPIWhenNil(t *testing.T) {
	app := &App{}
	assert.Nil(t, app.GetAPI())
}

func TestAddHandlerWhenSocketIsNil(t *testing.T) {
	app := &App{}
	stubHandler := &stubSocketHandler{}
	app.AddHandler(stubHandler)
	assert.Equal(t, 0, app.totalEvents)
}

func TestAppStartWhenNoSocketOrHTTP(t *testing.T) {
	app := &App{}
	app.Start()
}

func TestWithHTTP(t *testing.T) {
	opt := WithHttp(9000)
	assert.NotNil(t, opt)
}

func TestWithHTTPDefaultPort(t *testing.T) {
	opt := WithHttp()
	assert.NotNil(t, opt)
}

func TestWithDebugDisabled(t *testing.T) {
	opt := WithDebug(false)
	assert.NotNil(t, opt)
}

// stubSocketHandler implements handler.SocketHandler for testing
type stubSocketHandler struct{}

func (s *stubSocketHandler) GetConfig() handler2.SocketHandlerConfig {
	return handler2.SocketHandlerConfig{
		Handler: []handler2.SocketEvent{},
	}
}
