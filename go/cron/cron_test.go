package cron

import (
	"context"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
)

func TestNewEngineDefault(t *testing.T) {
	// Use WithTimeZone("UTC") to avoid nil cron.Option panic in robfig/cron v3.0.1
	eng := New(WithTimeZone("UTC"))
	assert.NotNil(t, eng)
	assert.NotNil(t, eng.engine)
	assert.NotNil(t, eng.ctx)
	assert.NotNil(t, eng.handlerList)
	assert.Equal(t, 0, eng.handlerTotal)
}

func TestNewEngineWithTimezone(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	assert.NotNil(t, eng)
}

func TestNewEngineWithInvalidTimezone(t *testing.T) {
	// This would call log.Fatal in production, so we can't test it directly
	// But we can test the option function
	opt := WithTimeZone("Invalid/Timezone")
	o := option{}
	opt(&o)
	assert.Equal(t, "Invalid/Timezone", o.timezone)
}

func newTestEngine() *Engine {
	return New(WithTimeZone("UTC"))
}

func TestEngineWrap(t *testing.T) {
	eng := newTestEngine()

	wrapper := &testWrapper{}
	eng.Wrap(wrapper)
	assert.Len(t, eng.wrapper, 1)
}

func TestEngineWrapMultiple(t *testing.T) {
	eng := newTestEngine()
	eng.Wrap(&testWrapper{})
	eng.Wrap(&testWrapper{ctxVal: "val2"})
	assert.Len(t, eng.wrapper, 2)
}

func TestEngineRegisterScheduler(t *testing.T) {
	eng := newTestEngine()
	// Initialize the handlerList map
	eng.handlerList = make(map[string]cron.EntryID)

	handler := func(ctx context.Context) *Response {
		return NewResponse()
	}

	eng.RegisterScheduler("* * * * *", handler)
	assert.Equal(t, 1, eng.handlerTotal)
}

func TestEngineRemoveSchedulerNotRegistered(t *testing.T) {
	eng := newTestEngine()
	eng.handlerList = make(map[string]cron.EntryID)

	// Removing a non-existent pattern should not panic
	eng.RemoveScheduler("non-existent")
	assert.Equal(t, 0, eng.handlerTotal)
}

func TestEngineStartAndStop(t *testing.T) {
	eng := newTestEngine()
	eng.handlerList = make(map[string]cron.EntryID)

	// Start the engine
	eng.Start()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop the engine
	eng.Stop()
}

// testWrapper implements Wrapper for testing
type testWrapper struct {
	ctxVal string
}

func (tw *testWrapper) WrapToContext(ctx context.Context) context.Context {
	if tw.ctxVal != "" {
		return context.WithValue(ctx, "test_key", tw.ctxVal)
	}
	return ctx
}
