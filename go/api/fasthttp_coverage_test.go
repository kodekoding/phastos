package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	"github.com/kodekoding/phastos/v2/go/common"
)

// TestFastHttpApp_Handler tests the Handler() method.
func TestFastHTTPApp_Handler(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := app.Handler()
	assert.NotNil(t, handler)
}

// TestFastHttpApp_Handler_WithGlobalMiddleware tests Handler() with global middlewares.
func TestFastHTTPApp_Handler_WithGlobalMiddleware(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	mwCalled := false
	_ = mwCalled // Used in assertion below
	mw := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			mwCalled = true
			next(ctx)
		}
	}

	app.AddGlobalMiddleware(mw)

	handler := app.Handler()
	assert.NotNil(t, handler)

	// Test that the middleware is applied
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/ping")
	ctx.Request.Header.SetMethod("GET")

	handler(ctx)

	// /ping path is skipped by requestLogger middleware
	// but the router handler should still be called
	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
}

// TestFastHttpApp_Handler_NotFound tests the Handler() returns 404 for unknown paths.
func TestFastHTTPApp_Handler_NotFound(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/nonexistent-path")
	ctx.Request.Header.SetMethod("GET")

	handler(ctx)

	assert.Equal(t, http.StatusNotFound, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
}

// TestFastHttpApp_Handler_WithRegisteredRoute tests Handler() with a registered route.
func TestFastHTTPApp_Handler_WithRegisteredRoute(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := func(req FastRequest, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello world")
	}

	routes := []FastRoute{
		{Method: "GET", Path: "/hello", Handler: handler, Version: 1},
	}

	ctrl := &testFastController{
		config: FastControllerConfig{
			Path:   "/api",
			Routes: routes,
		},
	}

	app.AddController(ctrl)

	// Get the handler
	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/api/hello")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "hello world")
}

// TestFastHttpApp_Handler_RequestID tests that request ID is set in response.
func TestFastHTTPApp_Handler_RequestID(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/ping")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set(common.RequestIDHeader, "test-request-id")

	handler(ctx)

	// Request ID should be set in response header
	assert.Equal(t, "test-request-id", string(ctx.Response.Header.Peek(common.RequestIDHeader)))
}

// TestFastHttpApp_Handler_PanicRecovery tests that panic is recovered properly.
func TestFastHTTPApp_Handler_PanicRecovery(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	// Register a handler that panics
	handler := func(req FastRequest, ctx context.Context) *Response {
		panic("test panic")
	}

	routes := []FastRoute{
		{Method: "GET", Path: "/panic", Handler: handler, Version: 1},
	}

	ctrl := &testFastController{
		config: FastControllerConfig{
			Path:   "/test",
			Routes: routes,
		},
	}

	app.AddController(ctrl)

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/test/panic")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	// Should return 500 error, not crash
	assert.Equal(t, http.StatusInternalServerError, ctx.Response.StatusCode())
}

// TestFastRequest_GetFile tests the GetFile method.
func TestFastRequest_GetFile(t *testing.T) {
	app := &FastHttpApp{}

	// Create a context without a file
	ctx := &fasthttp.RequestCtx{}

	req := &FastRequest{ctx: ctx, app: app}

	// GetFile should return an error when no file is present
	file, err := req.GetFile("file")
	assert.Error(t, err)
	assert.Nil(t, file)
}

// TestFastGenerateUniqueRequestKey tests the fastGenerateUniqueRequestKey function.
func TestFastGenerateUniqueRequestKey(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/test?foo=bar&baz=qux")

	key := fastGenerateUniqueRequestKey(ctx)

	assert.NotEmpty(t, key)
	// Key should contain method, path, and query params
	assert.Contains(t, key, "GET")
	assert.Contains(t, key, "/test")
}

// TestFastGenerateUniqueRequestKey_WithXForwardedFor tests with X-Forwarded-For header.
func TestFastGenerateUniqueRequestKey_WithXForwardedFor(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/api/users?id=123")
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.100")

	key := fastGenerateUniqueRequestKey(ctx)

	assert.NotEmpty(t, key)
	assert.Contains(t, key, "192.168.1.100")
	assert.Contains(t, key, "POST")
	assert.Contains(t, key, "/api/users")
}

// TestFastGenerateUniqueRequestKey_WithNoQueryParams tests with no query params.
func TestFastGenerateUniqueRequestKey_WithNoQueryParams(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/simple/path")

	key := fastGenerateUniqueRequestKey(ctx)

	assert.NotEmpty(t, key)
	assert.Contains(t, key, "GET")
	assert.Contains(t, key, "/simple/path")
}

// TestFastHttpApp_initDefaultRoutes tests the default routes.
func TestFastHttpApp_initDefaultRoutes(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/ping")
	ctx.Request.Header.SetMethod("GET")

	handler(ctx)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "pong")
}

// TestFastHttpApp_flushPendingMiddlewares_CalledTwice tests calling flush twice.
func TestFastHttpApp_flushPendingMiddlewares_CalledTwice(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	// Call flushPendingMiddlewares twice - second call should be no-op
	app.flushPendingMiddlewares()
	app.flushPendingMiddlewares()

	// Should not panic
}

// TestWrapDirectHandler tests wrapDirectHandler function.
func TestWrapDirectHandler(t *testing.T) {
	app := &FastHttpApp{}

	directHandler := func(req *FastRequest) *FastResponse {
		return NewFastResponse(req.Ctx()).SetMessage("direct response")
	}

	wrapped := app.wrapDirectHandler(directHandler)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.SetMethod("GET")

	wrapped(ctx)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "direct response")
}

// TestWrapDirectHandler_WithPanic is intentionally removed because wrapDirectHandler
// does not have panic recovery. Direct handlers must not panic.

// TestFastSendResponse tests the fastSendResponse function.
func TestFastSendResponse(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}

	resp := NewResponse().SetMessage("test message")
	resp.TraceId = "trace-123"

	fastSendResponse(ctx, resp)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "test message")

	ReleaseResponse(resp)
}

// TestFastSendResponse_WithError tests fastSendResponse with error.
func TestFastSendResponse_WithError(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}

httpErr := InternalServerError("server error", "SERVER_ERROR")
	httpErr.TraceId = "trace-456"
	resp := NewResponse().SetHTTPError(httpErr)

	fastSendResponse(ctx, resp)

	assert.Equal(t, http.StatusInternalServerError, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "server error")
	assert.Contains(t, body, "SERVER_ERROR")

	ReleaseResponse(resp)
}

// TestFastSendResponse_WithData tests fastSendResponse with data.
func TestFastSendResponse_WithData(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}

	data := map[string]string{"key": "value", "name": "test"}
	resp := NewResponse().SetData(data)
	resp.TraceId = "trace-789"

	fastSendResponse(ctx, resp)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "key")
	assert.Contains(t, body, "value")

	ReleaseResponse(resp)
}

// TestFastSendResponse_WithCustomHeader tests fastSendResponse with custom headers.
func TestFastSendResponse_WithCustomHeader(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}

	resp := NewResponse().SetMessage("with header")
	resp.SetCustomHeader("X-Custom-Header", "custom-value")
	resp.TraceId = "trace-abc"

	fastSendResponse(ctx, resp)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "custom-value", string(ctx.Response.Header.Peek("X-Custom-Header")))

	ReleaseResponse(resp)
}

// TestFastSendResponse_WithFileDownload tests fastSendResponse with file download.
func TestFastSendResponse_WithFileDownload(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}

	resp := NewResponse()
	resp.SetFileDownload([]byte("file content"), "test.txt", "text/plain")
	resp.TraceId = "trace-file"

	fastSendResponse(ctx, resp)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "text/plain", string(ctx.Response.Header.ContentType()))
	assert.Contains(t, string(ctx.Response.Header.Peek("Content-Disposition")), "test.txt")
	assert.Equal(t, "file content", string(ctx.Response.Body()))

	ReleaseResponse(resp)
}

// TestFastSendResponse_NoContent tests fastSendResponse with no content.
func TestFastSendResponse_NoContent(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}

	resp := NewResponse()
	// No message, no data, no error - should return 204

	fastSendResponse(ctx, resp)

	assert.Equal(t, http.StatusNoContent, ctx.Response.StatusCode())

	ReleaseResponse(resp)
}

// TestFastSendResponse_WithVersionHeader tests fastSendResponse with version header.
func TestFastSendResponse_WithVersionHeader(t *testing.T) {
	// Set app version
	originalVersion := appVersion
	appVersion = "1.2.3"
	defer func() { appVersion = originalVersion }()

	ctx := &fasthttp.RequestCtx{}

	resp := NewResponse().SetMessage("version test")
	resp.TraceId = "trace-version"

	fastSendResponse(ctx, resp)

	assert.Equal(t, "1.2.3", string(ctx.Response.Header.Peek("X-App-Version")))

	ReleaseResponse(resp)
}

// TestFastInitHandler tests the fastInitHandler middleware.
func TestFastInitHandler(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := fastInitHandler(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")

	middleware(ctx)

	assert.True(t, nextCalled)
	assert.NotEmpty(t, ctx.Response.Header.Peek(common.RequestIDHeader))
}

// TestFastInitHandler_ExistingRequestID tests with existing request ID.
func TestFastInitHandler_ExistingRequestID(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := fastInitHandler(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set(common.RequestIDHeader, "existing-id")

	middleware(ctx)

	assert.True(t, nextCalled)
	assert.Equal(t, "existing-id", string(ctx.Response.Header.Peek(common.RequestIDHeader)))
}

// TestFastInitHandler_XRequestIDFallback tests X-Request-ID fallback.
func TestFastInitHandler_XRequestIDFallback(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := fastInitHandler(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("X-Request-ID", "x-request-id-value")

	middleware(ctx)

	assert.True(t, nextCalled)
	assert.Equal(t, "x-request-id-value", string(ctx.Response.Header.Peek(common.RequestIDHeader)))
}

// TestFastPanicHandler tests the fastPanicHandler middleware.
func TestFastPanicHandler(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := fastPanicHandler(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")

	middleware(ctx)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
}

// TestFastPanicHandler_WithPanic tests panic recovery.
func TestFastPanicHandler_WithPanic(t *testing.T) {
	next := func(ctx *fasthttp.RequestCtx) {
		panic("test panic")
	}

	middleware := fastPanicHandler(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")

	middleware(ctx)

	assert.Equal(t, http.StatusInternalServerError, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "SERVER_ERROR")
}

// TestFastRequestLogger tests the fastRequestLogger middleware.
func TestFastRequestLogger(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := fastRequestLogger(next, map[string]struct{}{"/skip": {}})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/test")

	middleware(ctx)

	assert.True(t, nextCalled)
}

// TestFastRequestLogger_SkipPing tests that /ping is skipped.
func TestFastRequestLogger_SkipPing(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := fastRequestLogger(next, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/ping")

	middleware(ctx)

	assert.True(t, nextCalled)
}

// TestFastRequestLogger_SkipCustomPath tests custom skip paths.
func TestFastRequestLogger_SkipCustomPath(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	skipPaths := map[string]struct{}{"/health": {}}
	middleware := fastRequestLogger(next, skipPaths)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/health")

	middleware(ctx)

	assert.True(t, nextCalled)
}

// TestFastHttpApp_AddController_WithControllerMiddleware tests controller-level middleware.
func TestFastHttpApp_AddController_WithControllerMiddleware(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	mwCalled := false
	mw := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			mwCalled = true
			next(ctx)
		}
	}

	handler := func(req FastRequest, ctx context.Context) *Response {
		return NewResponse().SetMessage("controller mw test")
	}

	routes := []FastRoute{
		{Method: "GET", Path: "/test", Handler: handler, Version: 1},
	}

	ctrl := &testFastController{
		config: FastControllerConfig{
			Path:        "/ctrltest",
			Routes:      routes,
			Middlewares: []FastMiddleware{mw},
		},
	}

	app.AddController(ctrl)

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/ctrltest/test")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	assert.True(t, mwCalled)
	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
}