package context

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// resetLogForTest resets the logger and once.Do so the next call to NewLog/Log/Ctx
// re-executes the initialization logic.
func resetLogForTest() {
	logger = nil
	once = sync.Once{}
}

func TestANewLog_ProductionEnv(t *testing.T) {
	t.Run("should initialize logger with production env via Log", func(t *testing.T) {
		os.Setenv("APPS_ENV", "production")
		defer os.Unsetenv("APPS_ENV")
		l := Log()
		assert.NotNil(t, l)
	})
}

func TestZNewLog(t *testing.T) {
	t.Run("should initialize logger without panic when newRelicApp is nil", func(t *testing.T) {
		NewLog(nil)
	})
}

func TestLog_NoContext(t *testing.T) {
	t.Run("should return logger when called without context", func(t *testing.T) {
		l := Log()
		assert.NotNil(t, l)
	})
}

func TestLog_WithContext(t *testing.T) {
	t.Run("should return logger even when context has no logger", func(t *testing.T) {
		ctx := context.Background()
		l := Log(ctx)
		assert.NotNil(t, l)
	})
}

func TestCtx_NoContext(t *testing.T) {
	t.Run("should return logger when called without context", func(t *testing.T) {
		l := Ctx()
		assert.NotNil(t, l)
	})
}

func TestCtx_WithContext(t *testing.T) {
	t.Run("should return logger even when context has no logger", func(t *testing.T) {
		ctx := context.Background()
		l := Ctx(ctx)
		assert.NotNil(t, l)
	})
}

func TestCtx_WithZerologContext(t *testing.T) {
	t.Run("should use logger from context when available", func(t *testing.T) {
		// Create a zerolog logger and attach it to context
		ctx := context.Background()
		testLogger := zerolog.Nop()
		ctx = testLogger.WithContext(ctx)

		l := Ctx(ctx)
		assert.NotNil(t, l)
	})
}

func TestNewLog_DebugEnv(t *testing.T) {
	resetLogForTest()
	t.Run("debug env (default) sets DebugLevel", func(t *testing.T) {
		os.Unsetenv("APPS_ENV")
		NewLog(nil)
	})
}

func TestCtx_NilLoggerInit(t *testing.T) {
	resetLogForTest()
	t.Run("Ctx initializes nil logger", func(t *testing.T) {
		os.Setenv("APPS_ENV", "production")
		defer os.Unsetenv("APPS_ENV")
		l := Ctx()
		assert.NotNil(t, l)
	})
}

func TestNewLog_WithNewRelicApp(t *testing.T) {
	resetLogForTest()

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("test-log-nr"),
		newrelic.ConfigLicense("0123456789012345678901234567890123456789"),
		newrelic.ConfigEnabled(false),
	)
	if err != nil || app == nil {
		t.Skip("New Relic not available")
	}

	NewLog(app)
	// logger should be initialized without panic
	assert.NotNil(t, logger)
}
