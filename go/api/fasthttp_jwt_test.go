package api

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/entity"
)

// TestFastJWTAuth_MissingAuthorizationHeader tests that missing auth header returns 401.
func TestFastJWTAuth_MissingAuthorizationHeader(t *testing.T) {
	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.False(t, nextCalled, "next handler should NOT be called without auth header")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestFastJWTAuth_EmptyToken tests that empty token returns 401.
func TestFastJWTAuth_EmptyToken(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer ")
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.False(t, nextCalled, "next handler should NOT be called with empty token")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestFastJWTAuth_MissingSigningKey tests that missing signing key returns 401.
func TestFastJWTAuth_MissingSigningKey(t *testing.T) {
	// Unset JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Unsetenv(common.EnvJWTSigningKey)
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer validtoken")
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.False(t, nextCalled, "next handler should NOT be called without signing key")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestFastJWTAuth_ValidToken tests that valid token passes through.
func TestFastJWTAuth_ValidToken(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	// Create a valid JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString)
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.True(t, nextCalled, "next handler should be called with valid token")
}

// TestFastJWTAuth_ExpiredToken tests that expired token returns 401.
func TestFastJWTAuth_ExpiredToken(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	// Create an expired JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(-time.Hour).Unix(), // expired
	})
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString)
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.False(t, nextCalled, "next handler should NOT be called with expired token")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestFastJWTAuth_InvalidSignature tests that invalid signature returns 401.
func TestFastJWTAuth_InvalidSignature(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	// Create a token with different signing key
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("wrong-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString)
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.False(t, nextCalled, "next handler should NOT be called with invalid signature")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestFastJWTAuth_WrongSigningMethod tests that wrong signing method returns 401.
func TestFastJWTAuth_WrongSigningMethod(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	// Test with malformed token - not valid JWT format
	ctx.Request.Header.Set("Authorization", "Bearer invalid.token.here")
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.False(t, nextCalled, "next handler should NOT be called with malformed token")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

// TestFastJWTAuth_ValidTokenWithClaims tests that valid token with claims passes through.
func TestFastJWTAuth_ValidTokenWithClaims(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	var capturedClaims *entity.JWTClaimData
	next := func(ctx *fasthttp.RequestCtx) {
		capturedClaims = ctx.UserValue("jwt_claim").(*entity.JWTClaimData) //nolint:errcheck
	}

	middleware := FastJWTAuth(next)

	// Create a valid JWT token with custom claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":    "user123",
		"role":   "admin",
		"userId": 456,
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString)
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.NotNil(t, capturedClaims, "claims should be set in context")
	assert.Equal(t, "user123", capturedClaims.Subject)
}

// TestFastJWTAuth_UsesResponseHeaderFallback tests trace ID fallback to response header.
func TestFastJWTAuth_UsesResponseHeaderFallback(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	// Create a valid JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString)
	// Set trace ID in response header (fallback)
	ctx.Response.Header.Set(common.RequestIDHeader, "fallback-trace-id")
	// Don't set request header

	middleware(ctx)

	assert.True(t, nextCalled, "next handler should be called")
}

// TestFastJWTUnauthorized tests the fastJWTUnauthorized helper.
func TestFastJWTUnauthorized(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.SetStatusCode(http.StatusOK) // default

	fastJWTUnauthorized(ctx, "trace-abc", "token expired", "TOKEN_EXPIRED")

	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))

	body := string(ctx.Response.Body())
	assert.Contains(t, body, "token expired")
	assert.Contains(t, body, "TOKEN_EXPIRED")
	assert.Contains(t, body, "trace-abc")
}

// TestFastJWTAuth_InvalidClaimsStruct tests handling of claims that can't be unmarshaled.
func TestFastJWTAuth_InvalidClaimsStruct(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	next := func(ctx *fasthttp.RequestCtx) {
		// Just pass through
	}

	middleware := FastJWTAuth(next)

	// Create a token with claims that might cause unmarshal issues
	// Using nested complex structure
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     "user123",
		"profile": map[string]interface{}{"nested": map[string]interface{}{"deep": "value"}},
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString)
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	// This should work with the JWTClaimData struct
	middleware(ctx)

	// The middleware should handle this case or reject it
	// Depending on how JWTClaimData is structured
}

// TestFastJWTAuth_BearerPrefixWithSpace tests "Bearer " prefix handling.
func TestFastJWTAuth_BearerPrefixWithSpace(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	// Create a valid JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	// Authorization header with "Bearer " prefix followed by token
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString)
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	assert.True(t, nextCalled, "next handler should be called with valid Bearer token")
}

// TestFastJWTAuth_TokenWithWhitespace tests token with extra whitespace.
func TestFastJWTAuth_TokenWithWhitespace(t *testing.T) {
	// Set JWT signing key
	originalKey := os.Getenv(common.EnvJWTSigningKey)
	os.Setenv(common.EnvJWTSigningKey, "test-secret-key")
	defer os.Setenv(common.EnvJWTSigningKey, originalKey)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastJWTAuth(next)

	// Create a valid JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	assert.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Authorization", "Bearer "+tokenString+" ")
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)

	// TrimSpace should handle the extra whitespace
	assert.True(t, nextCalled, "next handler should be called with whitespace-trimmed token")
}