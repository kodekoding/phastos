package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/context"
)

func TestJWTAuth_MissingAuthHeader(t *testing.T) {
	t.Setenv(common.EnvJWTSigningKey, "test-secret-key")

	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, common.ErrInvalidTokenCode, body["code"])
	assert.Equal(t, common.ErrInvalidTokenMessage, body["message"])
}

func TestJWTAuth_EmptySigningKey(t *testing.T) {
	// Ensure no signing key is set
	os.Unsetenv(common.EnvJWTSigningKey)

	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_KEY", body["code"])
	assert.Contains(t, body["message"], "JWT Signing Key is nil")
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	t.Setenv(common.EnvJWTSigningKey, "test-secret-key")

	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-value")
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_CLAIMS", body["code"])
}

func TestJWTAuth_ValidToken(t *testing.T) {
	signingKey := "test-secret-key-for-jwt"
	t.Setenv(common.EnvJWTSigningKey, signingKey)

	// Create a valid JWT token
	claims := jwt.MapClaims{
		"user_id": "12345",
		"role":    "admin",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signingKey))
	require.NoError(t, err)

	nextCalled := false
	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		// Verify JWT data is set in context
		jwtData := context.GetJWT(r.Context())
		assert.NotNil(t, jwtData)
		assert.Equal(t, tokenString, jwtData.Token)
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	handler.ServeHTTP(rr, req)

	assert.True(t, nextCalled, "next handler should be called")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTAuth_InvalidSigningMethod(t *testing.T) {
	signingKey := "test-secret-key-for-jwt"
	t.Setenv(common.EnvJWTSigningKey, signingKey)

	// Create a token signed with a different key but HS384 instead of HS256
	// This will be parsed but rejected by the keyFunc that checks Alg() == "HS256"
	claims := jwt.MapClaims{
		"user_id": "12345",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	tokenString, err := token.SignedString([]byte(signingKey))
	require.NoError(t, err)

	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var body map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_CLAIMS", body["code"])
	assert.Contains(t, body["message"], "unexpected jwt signing method")
}

func TestJWTAuth_WrongSigningKey(t *testing.T) {
	t.Setenv(common.EnvJWTSigningKey, "correct-signing-key")

	// Create token with wrong key
	claims := jwt.MapClaims{
		"user_id": "12345",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("wrong-signing-key"))
	require.NoError(t, err)

	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_TraceIdFromContext(t *testing.T) {
	t.Setenv(common.EnvJWTSigningKey, "test-key")

	// Test with request ID in context
	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Without Authorization header
	handler.ServeHTTP(rr, req)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	// trace_id should be present (empty string from context)
	_, hasTraceId := body["trace_id"]
	assert.True(t, hasTraceId)
}

func TestJWTAuth_BearerTokenPrefix(t *testing.T) {
	t.Setenv(common.EnvJWTSigningKey, "test-key")

	// Test that "Bearer " prefix is correctly stripped
	signingKey := "test-key"
	claims := jwt.MapClaims{
		"user_id": "12345",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signingKey))
	require.NoError(t, err)

	nextCalled := false
	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	handler.ServeHTTP(rr, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	signingKey := "test-key-expired"
	t.Setenv(common.EnvJWTSigningKey, signingKey)

	claims := jwt.MapClaims{
		"user_id": "12345",
		"exp":     time.Now().Add(-time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signingKey))
	require.NoError(t, err)

	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_WithNewRelicTransaction(t *testing.T) {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("test-jwt-nr"),
		newrelic.ConfigLicense("0123456789012345678901234567890123456789"),
		newrelic.ConfigEnabled(false),
	)
	if err != nil || app == nil {
		t.Skip("New Relic not available")
	}

	signingKey := "test-key-nr"
	t.Setenv(common.EnvJWTSigningKey, signingKey)

	claims := jwt.MapClaims{
		"user_id": "12345",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signingKey))
	require.NoError(t, err)

	txn := app.StartTransaction("test")

	nextCalled := false
	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req = req.WithContext(newrelic.NewContext(req.Context(), txn))
	handler.ServeHTTP(rr, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTAuth_BearerTokenWithWhitespace(t *testing.T) {
	signingKey := "test-key-whitespace"
	t.Setenv(common.EnvJWTSigningKey, signingKey)

	// Create token with surrounding whitespace in the bearer token
	claims := jwt.MapClaims{
		"user_id": "12345",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signingKey))
	require.NoError(t, err)

	nextCalled := false
	handler := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Add whitespace around token
	req.Header.Set("Authorization", "Bearer  "+tokenString+"  ")
	handler.ServeHTTP(rr, req)

	// Should still work since TrimSpace is applied
	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
}
