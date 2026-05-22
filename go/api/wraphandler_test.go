package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/common"
)

// --- wrapHandler sync path: handleResponseError non-HttpError ---

func TestApp_WrapHandler_SyncPath_NonHttpError(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	require.True(t, app.syncMode)

	handler := func(req Request, ctx context.Context) *Response {
		resp := NewResponse()
		resp.SetError(assert.AnError)
		return resp
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(common.RequestIDHeader, "sync-non-http-err")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- wrapHandler sync path: success ---

func TestApp_WrapHandler_SyncPath_Success(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("sync-ok")
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/sync-ok", nil)
	req.Header.Set(common.RequestIDHeader, "sync-ok-trace")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "sync-ok", body["message"])
}

// --- wrapHandler sync path: panic recovery ---

func TestApp_WrapHandler_SyncPath_Panic(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		panic("test panic in sync path")
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/panic-sync", nil)
	req.Header.Set(common.RequestIDHeader, "panic-sync-trace")
	w := httptest.NewRecorder()
	wrapped(w, req) // should not crash
}

// --- wrapHandler async path: success ---

func TestApp_WrapHandler_AsyncPath_Success(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()
	require.False(t, app.syncMode)

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("async-ok")
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/async-ok", nil)
	req.Header.Set(common.RequestIDHeader, "async-ok-trace")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "async-ok", body["message"])
}

// --- wrapHandler async path: error response ---

func TestApp_WrapHandler_AsyncPath_ErrorResponse(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetError(BadRequest("bad request", "BAD_REQ"))
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/async-err", nil)
	req.Header.Set(common.RequestIDHeader, "async-err-trace")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- wrapHandler async path: timeout ---

func TestApp_WrapHandler_AsyncPath_Timeout2(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(1))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		time.Sleep(3 * time.Second)
		return NewResponse().SetMessage("late")
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/timeout", nil)
	req.Header.Set(common.RequestIDHeader, "timeout-trace")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
}

// --- wrapHandler async path: singleflight ---

func TestApp_WrapHandler_AsyncPath_Singleflight(t *testing.T) {
	originalSF := os.Getenv("SINGLEFLIGHT_ACTIVE")
	os.Setenv("SINGLEFLIGHT_ACTIVE", "true")
	defer os.Setenv("SINGLEFLIGHT_ACTIVE", originalSF)

	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()
	require.True(t, app.sfActive)

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("sf-ok")
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/sf-test", nil)
	req.Header.Set(common.RequestIDHeader, "sf-trace-id")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- wrapHandler async path: singleflight error ---

func TestApp_WrapHandler_AsyncPath_SingleflightError(t *testing.T) {
	originalSF := os.Getenv("SINGLEFLIGHT_ACTIVE")
	os.Setenv("SINGLEFLIGHT_ACTIVE", "true")
	defer os.Setenv("SINGLEFLIGHT_ACTIVE", originalSF)

	app := NewApp(WithTimezone("UTC"), WithAPITimeout(5))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetError(InternalServerError("sf error", "SF_ERR"))
	}

	wrapped := app.wrapHandler(handler)
	req := httptest.NewRequest(http.MethodGet, "/sf-err", nil)
	req.Header.Set(common.RequestIDHeader, "sf-err-trace")
	req.Header.Set("X-Forwarded-For", "10.0.0.2")
	w := httptest.NewRecorder()
	wrapped(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Response.SetError with InternalServerError HttpError ---

func TestResponse_SetError_InternalServerErrorHttpError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	httpErr := InternalServerError("something went wrong", "INTERNAL_ERR")
	resp.SetError(httpErr)

	assert.Equal(t, "Internal Server Error", resp.Err.Error())
	assert.NotNil(t, resp.InternalError)
	assert.Equal(t, http.StatusInternalServerError, resp.InternalError.Status)
}

// --- Response.SetError with non-500 HttpError ---

func TestResponse_SetError_OtherHttpError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	httpErr := BadRequest("bad input", "BAD_INPUT")
	resp.SetError(httpErr)

	assert.Equal(t, httpErr, resp.Err)
	assert.Equal(t, http.StatusBadRequest, resp.InternalError.Status)
}

// --- generateUniqueRequestKey ---

func TestGenerateUniqueRequestKey_NoQueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")

	key := generateUniqueRequestKey(req)

	assert.Contains(t, key, "192.168.1.100")
	assert.Contains(t, key, "POST")
	assert.Contains(t, key, "/api/users")
}

func TestGenerateUniqueRequestKey_MultipleQueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search?q=foo&page=1&sort=name", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.5")

	key := generateUniqueRequestKey(req)

	assert.Contains(t, key, "page=1")
	assert.Contains(t, key, "q=foo")
	assert.Contains(t, key, "sort=name")
}

func TestGenerateUniqueRequestKey_RemoteAddrFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	key := generateUniqueRequestKey(req)
	assert.NotEmpty(t, key)
}
