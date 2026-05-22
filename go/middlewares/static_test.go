package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/common"
)

func TestStaticAuth_CorrectToken(t *testing.T) {
	secret := "my-secret-token"
	t.Setenv(common.EnvServiceSecret, secret)

	nextCalled := false
	handler := StaticAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(common.HeaderSecret, secret)
	handler.ServeHTTP(rr, req)

	assert.True(t, nextCalled, "next handler should be called")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStaticAuth_MissingToken(t *testing.T) {
	t.Setenv(common.EnvServiceSecret, "my-secret-token")

	handler := StaticAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No secret header set
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, common.ErrInvalidTokenCode, body["code"])
	assert.Equal(t, common.ErrInvalidTokenMessage, body["message"])
}

func TestStaticAuth_WrongToken(t *testing.T) {
	t.Setenv(common.EnvServiceSecret, "correct-secret")

	handler := StaticAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(common.HeaderSecret, "wrong-secret")
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, common.ErrInvalidTokenCode, body["code"])
	assert.Equal(t, common.ErrInvalidTokenMessage, body["message"])
}

func TestStaticAuth_EmptyEnvSecret(t *testing.T) {
	os.Unsetenv(common.EnvServiceSecret)

	handler := StaticAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(common.HeaderSecret, "any-token")
	handler.ServeHTTP(rr, req)

	// Empty expected token means nothing matches, should be unauthorized
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestStaticAuth_TraceId(t *testing.T) {
	t.Setenv(common.EnvServiceSecret, "secret")

	handler := StaticAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No secret header — check trace_id in response
	handler.ServeHTTP(rr, req)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	// trace_id should be present
	_, hasTraceId := body["trace_id"]
	assert.True(t, hasTraceId)
}

func TestStaticAuth_WithRequestIdInHeader(t *testing.T) {
	t.Setenv(common.EnvServiceSecret, "secret")

	handler := StaticAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Set request ID in header (the middleware reads from header)
	req.Header.Set(common.RequestIDHeader, "test-trace-123")

	// No secret header — should get unauthorized with trace ID
	handler.ServeHTTP(rr, req)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "test-trace-123", body["trace_id"])
}

func TestStaticAuth_WithAlternateRequestIdHeader(t *testing.T) {
	t.Setenv(common.EnvServiceSecret, "secret")

	handler := StaticAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Set request ID via alternate header
	req.Header.Set("X-Request-ID", "alt-trace-456")

	handler.ServeHTTP(rr, req)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "alt-trace-456", body["trace_id"])
}
