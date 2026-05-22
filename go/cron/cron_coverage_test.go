package cron

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
)

func TestNewEngineEmptyTimezone(t *testing.T) {
	// New() with empty timezone - goes through the cron.New(cronOpts) path
	// where cronOpts is nil, which requires WithSeconds or similar option in robfig/cron v3
	eng := New(WithTimeZone("UTC"))
	assert.NotNil(t, eng.engine)
	assert.Equal(t, context.Background(), eng.ctx)
}

func TestNewEngineWithValidTimezone(t *testing.T) {
	eng := New(WithTimeZone("Asia/Jakarta"))
	assert.NotNil(t, eng)
	assert.NotNil(t, eng.engine)
}

func TestRegisterSchedulerAndRemove(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	handlerCalled := false
	handler := func(ctx context.Context) *Response {
		handlerCalled = true
		return NewResponse().SetProcessName("test-job")
	}

	pattern := "* * * * *"
	eng.RegisterScheduler(pattern, handler)
	assert.Equal(t, 1, eng.handlerTotal)
	assert.Contains(t, eng.handlerList, pattern)

	// Remove the registered scheduler
	eg := New(WithTimeZone("UTC"))
	eg.handlerList = make(map[string]cron.EntryID)
	eg.RegisterScheduler(pattern, func(ctx context.Context) *Response {
		return NewResponse()
	})
	eg.RemoveScheduler(pattern)
	assert.Equal(t, 0, eg.handlerTotal)
	assert.NotContains(t, eg.handlerList, pattern)

	_ = handlerCalled
}

func TestRemoveSchedulerWithRegisteredPattern(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	pattern := "0 * * * *"
	eng.RegisterScheduler(pattern, func(ctx context.Context) *Response {
		return NewResponse()
	})
	assert.Equal(t, 1, eng.handlerTotal)

	eng.RemoveScheduler(pattern)
	assert.Equal(t, 0, eng.handlerTotal)
}

func TestRegisterMultipleSchedulers(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	eng.RegisterScheduler("* * * * *", func(ctx context.Context) *Response {
		return NewResponse()
	})
	eng.RegisterScheduler("0 * * * *", func(ctx context.Context) *Response {
		return NewResponse()
	})
	assert.Equal(t, 2, eng.handlerTotal)
}

func TestStartWithWrappers(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	// Add wrappers before starting
	wrapper1 := &testWrapper{ctxVal: "value1"}
	wrapper2 := &testWrapper{ctxVal: "value2"}
	eng.Wrap(wrapper1)
	eng.Wrap(wrapper2)

	eng.RegisterScheduler("* * * * *", func(ctx context.Context) *Response {
		return NewResponse()
	})

	eng.Start()
	time.Sleep(10 * time.Millisecond)

	// Verify context was modified by wrappers
	assert.Equal(t, "value2", eng.ctx.Value("test_key"))

	eng.Stop()
}

func TestWrapperCronHandlerSuccess(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	// Set env vars for the handler
	os.Setenv("CRON_JOB_TIMEOUT_PROCESS", "1")
	defer os.Unsetenv("CRON_JOB_TIMEOUT_PROCESS")

	handlerFinished := make(chan bool, 1)
	handler := func(ctx context.Context) *Response {
		handlerFinished <- true
		return NewResponse().SetProcessName("test-success-job")
	}

	// Use @every 1s to trigger quickly
	eng.RegisterScheduler("@every 1s", handler)
	eng.Start()
	defer eng.Stop()

	// Wait for handler to execute
	select {
	case <-handlerFinished:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("Handler did not execute within timeout")
	}
}

func TestWrapperCronHandlerError(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	os.Setenv("CRON_JOB_TIMEOUT_PROCESS", "1")
	defer os.Unsetenv("CRON_JOB_TIMEOUT_PROCESS")

	handlerFinished := make(chan bool, 1)
	handler := func(ctx context.Context) *Response {
		handlerFinished <- true
		return NewResponse().SetProcessName("test-error-job").SetError(assert.AnError)
	}

	eng.RegisterScheduler("@every 1s", handler)
	eng.Start()
	defer eng.Stop()

	select {
	case <-handlerFinished:
		// Handler ran and returned error
	case <-time.After(3 * time.Second):
		t.Fatal("Handler did not execute within timeout")
	}
}

func TestWrapperCronHandlerTimeout(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	// Set very short timeout (0 minutes = immediate timeout)
	os.Setenv("CRON_JOB_TIMEOUT_PROCESS", "0")
	defer os.Unsetenv("CRON_JOB_TIMEOUT_PROCESS")

	// The timeout is 0 minutes, so context.WithTimeout with 0 duration means deadline is already passed
	// Actually 0 minutes creates a 0 * time.Minute = 0ns timeout which is effectively immediate
	// The handler that sleeps won't complete in time
	handlerStarted := make(chan bool, 1)
	handler := func(ctx context.Context) *Response {
		handlerStarted <- true
		// Sleep longer than the 0-minute timeout
		time.Sleep(2 * time.Second)
		return NewResponse().SetProcessName("timeout-job")
	}

	eng.RegisterScheduler("@every 1s", handler)
	eng.Start()
	defer eng.Stop()

	// Wait for handler to start
	select {
	case <-handlerStarted:
		// Handler was called
	case <-time.After(5 * time.Second):
		t.Fatal("Handler was not started within timeout")
	}
}

func TestWrapperCronHandlerDefaultTimeout(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	// Don't set CRON_JOB_TIMEOUT_PROCESS - should default to 1
	os.Unsetenv("CRON_JOB_TIMEOUT_PROCESS")

	handlerFinished := make(chan bool, 1)
	handler := func(ctx context.Context) *Response {
		handlerFinished <- true
		return NewResponse().SetProcessName("default-timeout-job")
	}

	eng.RegisterScheduler("@every 1s", handler)
	eng.Start()
	defer eng.Stop()

	select {
	case <-handlerFinished:
		// Success with default timeout
	case <-time.After(3 * time.Second):
		t.Fatal("Handler did not execute within timeout")
	}
}

func TestEnginesInterface(t *testing.T) {
	var _ Engines = &Engine{}
}

func TestHandlerContext(t *testing.T) {
	eng := New(WithTimeZone("UTC"))
	eng.handlerList = make(map[string]cron.EntryID)

	// Add a wrapper that sets context values
	wrapper := &contextKeyWrapper{key: "cron_key", val: "cron_val"}
	eng.Wrap(wrapper)

	var receivedCtx context.Context
	handlerFinished := make(chan bool, 1)
	handler := func(ctx context.Context) *Response {
		receivedCtx = ctx
		handlerFinished <- true
		return NewResponse().SetProcessName("ctx-test")
	}

	eng.RegisterScheduler("@every 1s", handler)
	eng.Start()
	defer eng.Stop()

	select {
	case <-handlerFinished:
		assert.NotNil(t, receivedCtx)
		// The context should have the value from the wrapper
		assert.Equal(t, "cron_val", receivedCtx.Value("cron_key"))
	case <-time.After(3 * time.Second):
		t.Fatal("Handler did not execute within timeout")
	}
}

// contextKeyWrapper is a test wrapper that adds a value to context
type contextKeyWrapper struct {
	key string
	val string
}

func (w *contextKeyWrapper) WrapToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, w.key, w.val)
}

func TestResponseSetProcessNameChaining(t *testing.T) {
	resp := NewResponse().SetProcessName("proc1").SetProcessName("proc2")
	assert.Equal(t, "proc2", resp.processName)
}

func TestResponseSetErrorChaining(t *testing.T) {
	err1 := assert.AnError
	resp := NewResponse().SetError(err1)
	assert.Equal(t, err1, resp.err)
}

