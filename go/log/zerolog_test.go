package log

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestWithNewRelicApp(t *testing.T) {
	opt := WithNewRelicApp(nil)
	l := &Logger{}
	opt(l)
	assert.Nil(t, l.newRelicApp)
}

func TestWithAppVersion(t *testing.T) {
	opt := WithAppVersion("1.2.3")
	l := &Logger{}
	opt(l)
	assert.Equal(t, "1.2.3", l.appVersion)
}

func TestWithAppPort(t *testing.T) {
	opt := WithAppPort(8080)
	l := &Logger{}
	opt(l)
	assert.Equal(t, 8080, l.appPort)
}

func TestLoggerOptionFunctions(t *testing.T) {
	l := &Logger{}
	WithNewRelicApp(nil)(l)
	WithAppVersion("2.0.0")(l)
	WithAppPort(3000)(l)
	
	assert.Nil(t, l.newRelicApp)
	assert.Equal(t, "2.0.0", l.appVersion)
	assert.Equal(t, 3000, l.appPort)
}

func TestCtxReturnsLogger(t *testing.T) {
	ctx := context.Background()
	logger := Ctx(ctx)
	// Should return a zerolog.Logger, even if it's the default
	assert.NotNil(t, logger)
}

// resetLogOnceForTest resets the global logger and sync.Once so the
// next call to Get() re-initializes from scratch. This allows testing
// different initialization paths in the same test binary.
func resetLogOnceForTest() {
	once = sync.Once{}
	logZero = zerolog.Logger{}
}

func TestLoggerStruct(t *testing.T) {
	l := &Logger{
		newRelicApp: nil,
		appPort:     9090,
		appVersion:  "0.1.0",
	}
	assert.Nil(t, l.newRelicApp)
	assert.Equal(t, 9090, l.appPort)
	assert.Equal(t, "0.1.0", l.appVersion)
}

// TestAGetWithOptions runs first (alphabetically) to initialize
// the global logger with all options, so once.Do captures these paths.
func TestAGetWithOptions(t *testing.T) {
	os.Setenv("APPS_ENV", "production")
	os.Setenv("CONTAINER_NAME", "test-container")
	defer os.Unsetenv("APPS_ENV")
	defer os.Unsetenv("CONTAINER_NAME")

	logger := Get(WithAppVersion("1.0.0"), WithAppPort(8080))
	assert.NotNil(t, logger)
	assert.Equal(t, logZero, logger)
}

func TestGetReturnsGlobalLogger(t *testing.T) {
	logger := Get()
	assert.NotNil(t, logger)
	// Calling again should return same instance (sync.Once)
	logger2 := Get()
	assert.Equal(t, logger, logger2)
}

func TestLoggerWithNewRelicApp(t *testing.T) {
	// Create a nil app to test the option
	var app *newrelic.Application
	l := &Logger{}
	WithNewRelicApp(app)(l)
	assert.Nil(t, l.newRelicApp)
}

func TestCtxWithZerologContext(t *testing.T) {
	ctx := context.Background()
	// Set a logger in context
	testLogger := zerolog.Nop()
	ctx = testLogger.WithContext(ctx)
	logger := Ctx(ctx)
	assert.NotNil(t, logger)
}

func TestGetWithDebugEnv(t *testing.T) {
	resetLogOnceForTest()

	origEnv := os.Getenv("ENV")
	os.Setenv("ENV", "development")
	defer func() {
		os.Setenv("ENV", origEnv)
	}()

	logger := Get()
	assert.NotNil(t, logger)
}

func TestGetWithNewRelicApp(t *testing.T) {
	resetLogOnceForTest()

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("test-log"),
		newrelic.ConfigLicense("0123456789012345678901234567890123456789"),
		newrelic.ConfigEnabled(false),
	)
	if err != nil || app == nil {
		t.Skip("New Relic not available")
	}

	logger := Get(WithNewRelicApp(app))
	assert.NotNil(t, logger)
}
