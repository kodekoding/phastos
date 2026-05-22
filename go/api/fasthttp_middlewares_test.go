package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"golang.org/x/time/rate"

	"github.com/kodekoding/phastos/v2/go/common"
)

// --- FastStaticAuth ---

func TestFastStaticAuth_ValidToken(t *testing.T) {
	// Set the expected token in env
	originalVal := os.Getenv(common.EnvServiceSecret)
	os.Setenv(common.EnvServiceSecret, "test-secret-token")
	defer os.Setenv(common.EnvServiceSecret, originalVal)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastStaticAuth(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(common.HeaderSecret, "test-secret-token")
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)
	assert.True(t, nextCalled, "next handler should be called with valid token")
}

func TestFastStaticAuth_MissingToken(t *testing.T) {
	originalVal := os.Getenv(common.EnvServiceSecret)
	os.Setenv(common.EnvServiceSecret, "test-secret-token")
	defer os.Setenv(common.EnvServiceSecret, originalVal)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastStaticAuth(next)

	ctx := &fasthttp.RequestCtx{}
	// Don't set the secret header
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)
	assert.False(t, nextCalled, "next handler should NOT be called without token")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

func TestFastStaticAuth_WrongToken(t *testing.T) {
	originalVal := os.Getenv(common.EnvServiceSecret)
	os.Setenv(common.EnvServiceSecret, "correct-token")
	defer os.Setenv(common.EnvServiceSecret, originalVal)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastStaticAuth(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(common.HeaderSecret, "wrong-token")
	ctx.Request.Header.Set(common.RequestIDHeader, "trace-123")

	middleware(ctx)
	assert.False(t, nextCalled, "next handler should NOT be called with wrong token")
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
}

func TestFastStaticAuth_UsesResponseHeaderFallback(t *testing.T) {
	originalVal := os.Getenv(common.EnvServiceSecret)
	os.Setenv(common.EnvServiceSecret, "test-secret-token")
	defer os.Setenv(common.EnvServiceSecret, originalVal)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	middleware := FastStaticAuth(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(common.HeaderSecret, "test-secret-token")
	// Don't set request header, but set response header
	ctx.Response.Header.Set(common.RequestIDHeader, "trace-456")

	middleware(ctx)
	assert.True(t, nextCalled, "next handler should be called with valid token and fallback trace ID")
}

// --- FastNewRateLimiter ---

func TestFastNewRateLimiter_DefaultOptions(t *testing.T) {
	middleware := FastNewRateLimiter()
	assert.NotNil(t, middleware)

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	handler := middleware(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.SetMethod("GET")

	handler(ctx)
	assert.True(t, nextCalled, "first request should be allowed")
}

func TestFastNewRateLimiter_SkipPath(t *testing.T) {
	middleware := FastNewRateLimiter(func(rl *fastRateLimiter) {
		rl.skipPaths = map[string]struct{}{
			"/health": {},
		}
	})

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	handler := middleware(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/health")
	ctx.Request.Header.SetMethod("GET")

	handler(ctx)
	assert.True(t, nextCalled, "skipped path should always pass through")
}

func TestFastNewRateLimiter_RateLimitExceeded(t *testing.T) {
	// Create a very restrictive rate limiter: 1 request per second, burst of 1
	middleware := FastNewRateLimiter(func(rl *fastRateLimiter) {
		rl.limiter = newRateLimiterForTest(1, 1) // rate=1, burst=1
		rl.msg = "too many requests"
		rl.code = "RATE_LIMITED"
	})

	next := func(ctx *fasthttp.RequestCtx) {}

	handler := middleware(next)

	// First request should pass
	ctx1 := &fasthttp.RequestCtx{}
	ctx1.Request.SetRequestURI("/test")
	ctx1.Request.Header.SetMethod("GET")
	handler(ctx1)

	// Second request should be rate limited
	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.SetRequestURI("/test")
	ctx2.Request.Header.SetMethod("GET")
	handler(ctx2)

	assert.Equal(t, http.StatusTooManyRequests, ctx2.Response.StatusCode())
}

func TestFastNewRateLimiter_CustomKeyExtractor(t *testing.T) {
	extractedKey := ""
	middleware := FastNewRateLimiter(func(rl *fastRateLimiter) {
		rl.keyExtractor = func(ctx *fasthttp.RequestCtx) string {
			key := "custom-" + string(ctx.Request.Header.Peek("X-API-Key"))
			extractedKey = key
			return key
		}
	})

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	handler := middleware(next)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("X-API-Key", "abc123")

	handler(ctx)
	assert.True(t, nextCalled)
	assert.Equal(t, "custom-abc123", extractedKey)
}

func TestFastNewRateLimiter_DefaultKeyExtractor(t *testing.T) {
	extractor := fastDefaultKeyExtractor()

	t.Run("uses X-Forwarded-For header", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.Set("X-Forwarded-For", "10.0.0.1")
		key := extractor(ctx)
		assert.Equal(t, "10.0.0.1", key)
	})

	t.Run("falls back to X-Real-Ip", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.Set("X-Real-Ip", "10.0.0.2")
		key := extractor(ctx)
		assert.Equal(t, "10.0.0.2", key)
	})

	t.Run("takes first IP from comma-separated X-Forwarded-For", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
		key := extractor(ctx)
		assert.Equal(t, "10.0.0.1", key)
	})

	t.Run("falls back to RemoteIP", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		key := extractor(ctx)
		// RemoteIP returns something, just verify it's not empty
		assert.NotEmpty(t, key)
	})
}

// newRateLimiterForTest creates a rate.Limiter with specific parameters.
func newRateLimiterForTest(r rate.Limit, b int) *rate.Limiter {
	return rate.NewLimiter(r, b)
}
