package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	context2 "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

// stubNotifAction implements notifications.Action for testing.
// All fields that are written from goroutines are protected by mu.
type stubNotifAction struct {
	mu         sync.Mutex
	sendCalled bool
	sendErr    error
	isActive   bool
	notifType  string
	traceId    string
	dest       interface{}
}

func (s *stubNotifAction) Send(ctx context.Context, text string, attachment interface{}) error {
	s.mu.Lock()
	s.sendCalled = true
	s.mu.Unlock()
	return s.sendErr
}

func (s *stubNotifAction) IsActive() bool                  { return s.isActive }
func (s *stubNotifAction) Type() string                    { return s.notifType }
func (s *stubNotifAction) SetTraceId(traceId string) {
	s.mu.Lock()
	s.traceId = traceId
	s.mu.Unlock()
}
func (s *stubNotifAction) SetDestination(dest interface{}) { s.dest = dest }

func (s *stubNotifAction) wasSendCalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sendCalled
}

func (s *stubNotifAction) getTraceId() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.traceId
}

// stubActiveNotifPlatforms implements notifications.Platforms with active platforms.
type stubActiveNotifPlatforms struct {
	actions []notifications.Action
}

func (s *stubActiveNotifPlatforms) Slack() notifications.Action                      { return nil }
func (s *stubActiveNotifPlatforms) Telegram() notifications.Action                    { return nil }
func (s *stubActiveNotifPlatforms) GetAllPlatform() []notifications.Action           { return s.actions }
func (s *stubActiveNotifPlatforms) WrapToHandler(next http.Handler) http.Handler     { return next }
func (s *stubActiveNotifPlatforms) WrapToContext(ctx context.Context) context.Context { return ctx }

func TestPanicRecover_NoPanic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No panic — panicRecover should be a no-op
	panicRecover(req, "trace-1")
}

func TestPanicRecover_WithPanic_SendsNotification(t *testing.T) {
	// Set up request with notification context to prevent nil deref
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-recover-1")

	action := &stubNotifAction{isActive: true, notifType: "slack"}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action}}
	context2.SetNotif(req, platforms)

	// Trigger panic inside a function that calls panicRecover in defer
	func() {
		defer panicRecover(req, "trace-recover-1")
		panic("test panic for recovery")
	}()

	// Give the goroutine in panicRecover time to execute
	time.Sleep(100 * time.Millisecond)

	assert.True(t, action.wasSendCalled(), "notification Send should be called for active platform")
	assert.Equal(t, "trace-recover-1", action.getTraceId(), "trace ID should be set on notification action")
}

func TestPanicRecover_WithPanic_InactivePlatform(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-recover-2")

	action := &stubNotifAction{isActive: false, notifType: "slack"}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action}}
	context2.SetNotif(req, platforms)

	func() {
		defer panicRecover(req, "trace-recover-2")
		panic("test panic inactive")
	}()

	time.Sleep(100 * time.Millisecond)

	assert.False(t, action.wasSendCalled(), "Send should not be called for inactive platform")
}

func TestPanicRecover_WithPanic_SendError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-recover-3")

	action := &stubNotifAction{isActive: true, notifType: "slack", sendErr: assert.AnError}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action}}
	context2.SetNotif(req, platforms)

	// Should not panic even if Send returns an error
	func() {
		defer panicRecover(req, "trace-recover-3")
		panic("test panic send error")
	}()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, action.wasSendCalled(), "Send should be called even if it returns error")
}

func TestPanicRecover_WithPanic_UniqueKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-recover-4")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	action := &stubNotifAction{isActive: true, notifType: "telegram"}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action}}
	context2.SetNotif(req, platforms)

	func() {
		defer panicRecover(req, "trace-recover-4", "unique-key-123")
		panic("test panic with unique key")
	}()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, action.wasSendCalled(), "notification Send should be called with unique key")
}

func TestPanicRecover_WithPanic_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-recover-5")
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	action := &stubNotifAction{isActive: true, notifType: "slack"}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action}}
	context2.SetNotif(req, platforms)

	func() {
		defer panicRecover(req, "trace-recover-5")
		panic("test panic x-forwarded-for")
	}()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, action.wasSendCalled())
}

func TestPanicRecover_WithPanic_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-recover-6")
	req.RemoteAddr = "10.0.0.1:12345"

	action := &stubNotifAction{isActive: true, notifType: "slack"}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action}}
	context2.SetNotif(req, platforms)

	func() {
		defer panicRecover(req, "trace-recover-6")
		panic("test panic remote addr")
	}()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, action.wasSendCalled())
}

func TestPanicRecover_NilNotifContext(t *testing.T) {
	// When notif context is nil, panicRecover should still not crash the process.
	// The notif.GetAllPlatform() call happens in a goroutine, and if notif is nil,
	// it will panic inside that goroutine (which is recovered by the runtime).
	// With context2.SetNotif we avoid the nil deref.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-recover-nil")

	// Set an empty stub so GetNotif returns non-nil but GetAllPlatform returns nil
	context2.SetNotif(req, &stubNotifPlatforms{})

	// This should NOT crash — GetAllPlatform returns nil slice, for range is safe
	func() {
		defer panicRecover(req, "trace-recover-nil")
		panic("test panic nil notif context")
	}()

	time.Sleep(100 * time.Millisecond)
	// If we get here, no crash occurred
}

func TestPanicRecover_WithPanic_MultiplePlatforms(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "trace-multi")

	action1 := &stubNotifAction{isActive: true, notifType: "slack"}
	action2 := &stubNotifAction{isActive: true, notifType: "telegram"}
	action3 := &stubNotifAction{isActive: false, notifType: "email"}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action1, action2, action3}}
	context2.SetNotif(req, platforms)

	func() {
		defer panicRecover(req, "trace-multi")
		panic("test panic multi platform")
	}()

	time.Sleep(100 * time.Millisecond)

	assert.True(t, action1.wasSendCalled(), "slack notification should be sent")
	assert.True(t, action2.wasSendCalled(), "telegram notification should be sent")
	assert.False(t, action3.wasSendCalled(), "inactive platform should not send")
}

func TestPanicRecover_IntegrationWithApp(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		panic("integration panic test")
	}

	wrapped := app.wrapHandler(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "integration-trace")

	action := &stubNotifAction{isActive: true, notifType: "slack"}
	platforms := &stubActiveNotifPlatforms{actions: []notifications.Action{action}}
	context2.SetNotif(req, platforms)

	w := httptest.NewRecorder()

	// Should not crash
	wrapped(w, req)

	time.Sleep(100 * time.Millisecond)
	assert.True(t, action.wasSendCalled(), "notification should be sent on panic in handler")
}

func TestPanicRecover_SyncPath_PanicHandledGracefully(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	require.True(t, app.syncMode)

	handler := func(req Request, ctx context.Context) *Response {
		panic("sync panic")
	}

	wrapped := app.wrapHandler(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "sync-trace")
	context2.SetNotif(req, &stubNotifPlatforms{})

	w := httptest.NewRecorder()
	wrapped(w, req)
	// After panicRecover in sync path, response is default 200 since
	// panicRecover doesn't write a response
	assert.Equal(t, http.StatusOK, w.Code)
}
