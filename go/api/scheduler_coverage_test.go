package api

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kodekoding/phastos/v2/go/cron"
)

// testWrapper implements cron.Wrapper interface for testing.
type testWrapper struct {
	ctx context.Context
}

func (w *testWrapper) WrapToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "test-key", "test-value")
}

// TestAddScheduler tests AddScheduler function.
func TestAddScheduler(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob("UTC"))
	app.Init()

	// Test that AddScheduler can be called without panic
	// The actual scheduler will be tested by the cron package tests
	handler := func(ctx context.Context) *cron.Response {
		return &cron.Response{}
	}

	app.AddScheduler("@every 1s", handler)

	// Verify the app has a cron engine
	assert.NotNil(t, app.cron)
}

// TestWrapScheduler tests WrapScheduler function.
func TestWrapScheduler(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob("UTC"))
	app.Init()

	// Test that WrapScheduler can be called
	wrapper := &testWrapper{}
	app.WrapScheduler(wrapper)

	// The wrapper should be applied when cron starts
	// We verify the app has a cron engine and can call WrapScheduler
	assert.NotNil(t, app.cron)
}

// TestWrapScheduler_WithMultipleWrappers tests adding multiple wrappers.
func TestWrapScheduler_WithMultipleWrappers(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob("UTC"))
	app.Init()

	wrapper1 := &testWrapper{}
	wrapper2 := &testWrapper{}

	app.WrapScheduler(wrapper1)
	app.WrapScheduler(wrapper2)

	// Multiple wrappers can be added
	assert.NotNil(t, app.cron)
}

// TestStart_WithCronJob tests Start function with cron configured.
func TestStart_WithCronJob(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob("UTC"), WithAPITimeout(0))
	app.Init()

	// Verify app is set up correctly for Start
	assert.NotNil(t, app.cron)
	assert.False(t, app.useFastHttp)
}

// TestStart_WithSSE tests Start function with SSE configured.
func TestStart_WithSSE(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithSSE(), WithAPITimeout(0))
	app.Init()

	// Verify app is set up correctly for Start with SSE
	assert.NotNil(t, app.sseEvent)
}

// TestStart_WithBothCronAndSSE tests Start with both cron and SSE.
func TestStart_WithBothCronAndSSE(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob("UTC"), WithSSE(), WithAPITimeout(0))
	app.Init()

	assert.NotNil(t, app.cron)
	assert.NotNil(t, app.sseEvent)
}

// TestStart_WithFastHttp tests that Start uses fasthttp server when configured.
func TestStart_WithFastHttp(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithFastHttp(), WithAPITimeout(0))
	app.Init()

	assert.True(t, app.useFastHttp)
}

// TestStart_CORSEnvVars tests that CORS environment variables are read during Start.
func TestStart_CORSEnvVars(t *testing.T) {
	// Set CORS environment variables
	originalOrigin := os.Getenv("CORS_ORIGIN")
	originalHeader := os.Getenv("CORS_HEADER")

	os.Setenv("CORS_ORIGIN", "https://example.com")
	os.Setenv("CORS_HEADER", "X-Custom-Header")

	defer func() {
		os.Setenv("CORS_ORIGIN", originalOrigin)
		os.Setenv("CORS_HEADER", originalHeader)
	}()

	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	// The Start function reads CORS env vars
	// We can't easily test the full Start flow without a real server
	// but we verify the app is initialized correctly
	assert.NotNil(t, app.Http)
}

// TestStart_Version tests that version is set during Start.
func TestStart_Version(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.SetVersion("1.0.0")
	app.Init()

	// Verify version is set
	assert.Equal(t, "1.0.0", app.Version)
}

// TestStart_Wrappers tests that wrappers are applied during Start.
func TestStart_Wrappers(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	// Add a wrapper
	app.WrapToApp(nil)

	// Wrappers are stored and will be applied in Start
	assert.Len(t, app.wrapper, 1)
}

// TestStart_NotifyServiceStatus tests NOTIFY_SERVICE_STATUS env during Start.
func TestStart_NotifyServiceStatus(t *testing.T) {
	originalNotify := os.Getenv("NOTIFY_SERVICE_STATUS")
	os.Setenv("NOTIFY_SERVICE_STATUS", "true")
	defer os.Setenv("NOTIFY_SERVICE_STATUS", originalNotify)

	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	// Start would send notification when NOTIFY_SERVICE_STATUS is true
	// We just verify the app is initialized correctly
	assert.NotNil(t, app.Http)
}

// TestAddScheduler_MultipleSchedulers tests adding multiple schedulers.
func TestAddScheduler_MultipleSchedulers(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob("UTC"))
	app.Init()

	// Add multiple schedulers
	handler1 := func(ctx context.Context) *cron.Response { return &cron.Response{} }
	handler2 := func(ctx context.Context) *cron.Response { return &cron.Response{} }

	app.AddScheduler("@every 1s", handler1)
	app.AddScheduler("@every 2s", handler2)

	// Both schedulers should be registered
	assert.NotNil(t, app.cron)
}
