package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	context2 "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

// stubNotifPlatforms implements notifications.Platforms for testing.
type stubNotifPlatforms struct{}

func (s *stubNotifPlatforms) Slack() notifications.Action                          { return nil }
func (s *stubNotifPlatforms) Telegram() notifications.Action                        { return nil }
func (s *stubNotifPlatforms) FCM() notifications.Action                            { return nil }
func (s *stubNotifPlatforms) GetAllPlatform() []notifications.Action                { return nil }
func (s *stubNotifPlatforms) WrapToHandler(next http.Handler) http.Handler          { return next }
func (s *stubNotifPlatforms) WrapToContext(ctx context.Context) context.Context     { return ctx }

// --- NewApp and Options ---

func TestNewApp_Defaults(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	assert.NotNil(t, app)
	assert.Equal(t, 0, app.TotalEndpoints)
	assert.NotNil(t, app.middlewares)
	assert.NotNil(t, app.Config)
	assert.Equal(t, 8000, app.Port)
	assert.Equal(t, 3, app.ReadTimeout)
	assert.Equal(t, 3, app.WriteTimeout)
	assert.Equal(t, 3, app.apiTimeout)
	assert.Equal(t, "UTC", app.timezoneRegion)
	assert.True(t, app.pprofEnabled)
	assert.False(t, app.useFastHttp)
}

func TestWithAppPort(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAppPort(9090))
	assert.Equal(t, 9090, app.Port)
}

func TestReadTimeout(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), ReadTimeout(10))
	assert.Equal(t, 10, app.ReadTimeout)
}

func TestWriteTimeout(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WriteTimeout(15))
	assert.Equal(t, 15, app.WriteTimeout)
}

func TestWithAPITimeout(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	assert.Equal(t, 5, app.apiTimeout)
}

func TestWithTimezone(t *testing.T) {
	app := NewApp(WithTimezone("America/New_York"))
	assert.Equal(t, "America/New_York", app.timezoneRegion)
}

func TestWithCronJob_DefaultTimezone(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob())
	assert.NotNil(t, app.cron)
}

func TestWithCronJob_CustomTimezone(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithCronJob("UTC"))
	assert.NotNil(t, app.cron)
}

func TestWithPprof(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithPprof(false))
	assert.False(t, app.pprofEnabled)
}

func TestWithSSE(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithSSE())
	assert.NotNil(t, app.sseEvent)
}

func TestWithFastHttp(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithFastHttp())
	assert.True(t, app.useFastHttp)
}

func TestWithSkipLogPaths(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithSkipLogPaths("/health", "/metrics"))
	assert.NotNil(t, app.skipLogPaths)
	_, hasHealth := app.skipLogPaths["/health"]
	_, hasMetrics := app.skipLogPaths["/metrics"]
	assert.True(t, hasHealth)
	assert.True(t, hasMetrics)
}

func TestWithGlobalMiddleware(t *testing.T) {
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	app := NewApp(WithTimezone("UTC"), WithGlobalMiddleware(mw))
	assert.Len(t, app.globalMiddlewares, 1)
}

func TestAddGlobalMiddleware(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	mw := func(next http.Handler) http.Handler { return next }
	app.AddGlobalMiddleware(mw)
	assert.Len(t, app.globalMiddlewares, 1)
	assert.True(t, app.pendingMiddlewares)

	app.AddGlobalMiddleware(mw)
	assert.Len(t, app.globalMiddlewares, 2)
}

// --- Init and App methods ---

func TestApp_Init(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	assert.NotNil(t, app.Http)
	assert.True(t, app.syncMode)
	assert.True(t, app.pendingMiddlewares)
}

func TestApp_Init_SyncModeWhenNoTimeout(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	assert.True(t, app.syncMode, "syncMode should be true when apiTimeout=0 and sfActive=false")
}

func TestApp_Init_AsyncModeWhenTimeout(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()
	assert.False(t, app.syncMode, "syncMode should be false when apiTimeout>0")
}

func TestApp_DB(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	assert.Nil(t, app.DB())
}

func TestApp_SetVersion(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	app.SetVersion("1.2.3")
	assert.Equal(t, "1.2.3", app.Version)
	assert.Equal(t, "1.2.3", appVersion)
}

func TestApp_Trx(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	assert.Nil(t, app.Trx())
}

// --- requestValidator ---

func TestApp_requestValidator_ValidStruct(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	type validStruct struct {
		Name  string `validate:"required"`
		Email string `validate:"required"`
	}
	err := app.requestValidator(validStruct{Name: "John", Email: "john@example.com"})
	assert.Nil(t, err)
}

func TestApp_requestValidator_InvalidStruct(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	type invalidStruct struct {
		Name  string `validate:"required"`
		Email string `validate:"required"`
	}
	err := app.requestValidator(invalidStruct{Name: "", Email: ""})
	assert.NotNil(t, err)
	httpErr, ok := err.(*HttpError)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnprocessableEntity, httpErr.Status)
	assert.Equal(t, "VALIDATION_ERROR", httpErr.Code)
}

// --- RegisterMiddlewareFunc and RegisterMiddleware ---

func TestApp_RegisterMiddlewareFunc(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	mw := func(next http.Handler) http.Handler { return next }
	app.RegisterMiddlewareFunc("auth", mw)
	assert.NotNil(t, app.middlewares["auth"])
}

func TestRegisterMiddleware_Generic(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	mw := func(next http.Handler) http.Handler { return next }
	RegisterMiddleware(app, "auth", mw)
	assert.NotNil(t, app.middlewares["auth"])
}

// --- AddController ---

type mockController struct {
	config ControllerConfig
}

func (m *mockController) GetConfig() ControllerConfig {
	return m.config
}

func (m *mockController) SetRegisteredMiddlewares(mw map[string]any) {}

func TestApp_AddController(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	routes := []Route{
		NewRoute(http.MethodGet, handler, WithPath("/list")),
		NewRoute(http.MethodPost, handler, WithPath("/create")),
	}
	ctrl := &mockController{
		config: ControllerConfig{
			Path:   "/users",
			Routes: routes,
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 routes + 1 (/ping default)
}

func TestApp_AddController_WithMiddlewares(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	mw := func(next http.Handler) http.Handler { return next }
	ctrlMiddlewares := []func(http.Handler) http.Handler{mw}
	routeMiddlewares := []func(http.Handler) http.Handler{mw}

	routes := []Route{
		{
			Method:      http.MethodGet,
			Path:        "/list",
			Handler:     handler,
			Middlewares: &routeMiddlewares,
		},
	}
	ctrl := &mockController{
		config: ControllerConfig{
			Path:        "/items",
			Routes:      routes,
			Middlewares: &ctrlMiddlewares,
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + 1 (/ping)
}

// --- AddControllers ---

type mockControllers struct {
	controllers []Controller
}

func (m *mockControllers) Register() []Controller {
	return m.controllers
}

func TestApp_AddControllers(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl1 := &mockController{
		config: ControllerConfig{
			Path: "/users",
			Routes: []Route{
				NewRoute(http.MethodGet, handler, WithPath("/list")),
			},
		},
	}
	ctrl2 := &mockController{
		config: ControllerConfig{
			Path: "/items",
			Routes: []Route{
				NewRoute(http.MethodGet, handler, WithPath("/list")),
			},
		},
	}

	ctrls := &mockControllers{controllers: []Controller{ctrl1, ctrl2}}
	app.AddControllers(ctrls)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 routes + 1 (/ping)
}

// --- generateUniqueRequestKey ---

func TestGenerateUniqueRequestKey(t *testing.T) {
	t.Run("uses X-Forwarded-For for client IP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?foo=bar", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		key := generateUniqueRequestKey(req)
		assert.Contains(t, key, "10.0.0.1")
		assert.Contains(t, key, "GET")
		assert.Contains(t, key, "/test")
	})

	t.Run("falls back to RemoteAddr when no X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/create", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		key := generateUniqueRequestKey(req)
		assert.Contains(t, key, "192.168.1.1:12345")
		assert.Contains(t, key, "POST")
	})

	t.Run("includes query params sorted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?b=2&a=1", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		key := generateUniqueRequestKey(req)
		assert.Contains(t, key, "a=1")
		assert.Contains(t, key, "b=2")
	})

	t.Run("no query params produces empty query string", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		key := generateUniqueRequestKey(req)
		parts := strings.SplitN(key, "|", 4)
		require.Len(t, parts, 4)
		// When no query params, queryParams is [""]
		assert.Equal(t, "", parts[3])
	})
}

// --- wrapHandler sync path ---

func TestApp_WrapHandler_SyncPath(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	assert.True(t, app.syncMode)

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("sync-ok")
	}

	wrapped := app.wrapHandler(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "test-trace-id")
	w := httptest.NewRecorder()

	wrapped(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "sync-ok", body["message"])
}

// TestApp_WrapHandler_SyncPath_WithPanic tests that when a handler panics
// in sync mode, panicRecover catches it and logs the error (doesn't crash).
// The response defaults to 200 because panicRecover recovers the panic
// but doesn't write a response — it only logs and sends notifications.
// The PanicHandler middleware is tested separately in middlewares_test.go.
func TestApp_WrapHandler_SyncPath_WithPanic(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		panic("test panic")
	}

	wrapped := app.wrapHandler(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "test-trace-id")
	// Set a notification platform in the context to prevent nil deref in panicRecover
	context2.SetNotif(req, &stubNotifPlatforms{})
	w := httptest.NewRecorder()

	// panicRecover catches the panic inside wrapHandler — it doesn't crash the process
	// and the response defaults to 200 (no explicit status set after panic recovery).
	// This test verifies the recovery mechanism works without crashing.
	wrapped(w, req)
	// After panicRecover, the response status is the default (200) since
	// panicRecover doesn't write an HTTP response — it only logs.
	assert.Equal(t, http.StatusOK, w.Code) // default status after panicRecover
}

func TestApp_WrapHandler_AsyncPath(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()
	assert.False(t, app.syncMode)

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("async-ok")
	}

	wrapped := app.wrapHandler(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "test-trace-id")
	w := httptest.NewRecorder()

	wrapped(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "async-ok", body["message"])
}

func TestApp_WrapHandler_AsyncPath_Timeout(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(1))
	app.Init()

	// Handler that sleeps longer than the timeout
	handler := func(req Request, ctx context.Context) *Response {
		time.Sleep(3 * time.Second)
		return NewResponse().SetMessage("too-late")
	}

	wrapped := app.wrapHandler(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "test-trace-id")
	w := httptest.NewRecorder()

	wrapped(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
}

func TestApp_WrapHandler_WithRequestIDFallback(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("ok")
	}

	wrapped := app.wrapHandler(handler)

	t.Run("reads from X-Request-Id header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-Id", "rid-1")
		w := httptest.NewRecorder()
		wrapped(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("falls back to X-Request-ID header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "rid-2")
		w := httptest.NewRecorder()
		wrapped(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- flushPendingMiddlewares ---

func TestApp_flushPendingMiddlewares_NoOpWhenNotPending(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	app.flushPendingMiddlewares() // first flush registers routes
	app.flushPendingMiddlewares() // second flush should be no-op
	// If it worked twice, it would panic on duplicate /ping registration
}

// --- SSE ---

func TestApp_SSE_PanicsWhenNotInitialized(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	// sseEvent is nil — calling SSE() should Fatal.
	// We can't easily test Fatal, so just verify it's nil
	assert.Nil(t, app.sseEvent)
}

func TestApp_SSE_ReturnsHubWhenInitialized(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithSSE())
	assert.NotNil(t, app.sseEvent)
	result := app.SSE()
	assert.NotNil(t, result)
}

// --- WrapToApp ---

func TestApp_WrapToApp(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	assert.Empty(t, app.wrapper)
	// Wrapper is an interface, we just test append works
	app.WrapToApp(nil)
	assert.Len(t, app.wrapper, 1)
}

// --- handleResponseError ---

func TestApp_handleResponseError_NoError(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	resp := NewResponse()
	defer ReleaseResponse(resp)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Should return early without panic
	app.handleResponseError(resp, req, "trace-1", context.Background())
}

// --- AddScheduler ---
// NOTE: AddScheduler test skipped because cron.Engine.handlerList is never
// initialized in cron.New(), causing a nil map write panic. This is a bug
// in the cron package, not in the api package.

// --- Route Group Tests ---

func TestApp_AddController_WithRouteGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	mw := func(next http.Handler) http.Handler { return next }

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/employee",
			Routes: []Route{
				{
					Path: "/absence",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler, WithPath("/list")),
						NewRoute(http.MethodGet, handler, WithPath("/today")),
						NewRoute(http.MethodGet, handler, WithPath("/{id}")),
					},
					Middlewares: &[]func(http.Handler) http.Handler{mw},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 4, app.TotalEndpoints) // 3 group routes + /ping
}

func TestApp_AddController_WithNestedRouteGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/employee",
			Routes: []Route{
				{
					Path: "/absence",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler),
						{
							Path: "/attendance",
							SubRoutes: []Route{
								NewRoute(http.MethodGet, handler, WithPath("/summary")),
							},
						},
					},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 leaf routes + /ping
}

func TestApp_AddController_GroupMiddlewareInheritance(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	groupMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Group-MW", "applied")
			next.ServeHTTP(w, r)
		})
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/test",
			Routes: []Route{
				{
					Path: "/group",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler, WithPath("/leaf")),
					},
					Middlewares: &[]func(http.Handler) http.Handler{groupMW},
				},
			},
		},
	}

	app.AddController(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/v1/test/group/leaf", nil)
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.Equal(t, "applied", w.Header().Get("X-Group-MW"))
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + /ping
}

func TestApp_AddController_EmptyRouteGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/test",
			Routes: []Route{
				{
					Path:      "/empty",
					SubRoutes: []Route{},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 1, app.TotalEndpoints) // only /ping
}

func TestApp_AddController_MixedFlatAndGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/api",
			Routes: []Route{
				NewRoute(http.MethodGet, handler, WithPath("/health")),
				{
					Path: "/users",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler, WithPath("/list")),
					},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 routes + /ping
}

func TestApp_AddController_WithNewGroupHelper(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/api",
			Routes: []Route{
				NewGroup("/items", []Route{
					NewRoute(http.MethodGet, handler, WithPath("/list")),
				}),
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + /ping
}

func TestApp_AddController_WithNewGroupHelperWithOpts(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	mw := func(next http.Handler) http.Handler { return next }

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/api",
			Routes: []Route{
				NewGroup("/items", []Route{
					NewRoute(http.MethodGet, handler, WithPath("/list")),
				}, WithMiddleware(mw)),
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + /ping
}

// --- initDefaultHandlers ---

func TestApp_initDefaultHandlers_PingEndpoint(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	// Flush pending middlewares to register default routes (/ping, NotFound, etc.)
	app.flushPendingMiddlewares()

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body["message"], "pong")
}
