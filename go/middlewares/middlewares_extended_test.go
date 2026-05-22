package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultKeyExtractor(t *testing.T) {
	extractor := defaultKeyExtractor()

	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		realIP     string
		expected   string
	}{
		{"from X-Forwarded-For", "1.2.3.4:1234", "10.0.0.1", "", "10.0.0.1"},
		{"from X-Real-Ip", "1.2.3.4:1234", "", "10.0.0.2", "10.0.0.2"},
		{"from RemoteAddr", "1.2.3.4:1234", "", "", "1.2.3.4"},
		{"X-Forwarded-For priority", "1.2.3.4:1234", "10.0.0.1", "10.0.0.2", "10.0.0.1"},
		{"X-Forwarded-For multiple IPs", "1.2.3.4:1234", "10.0.0.1, 10.0.0.2", "", "10.0.0.1"},
		{"RemoteAddr no port", "1.2.3.4", "", "", "1.2.3.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if tt.realIP != "" {
				req.Header.Set("X-Real-Ip", tt.realIP)
			}
			result := extractor(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithRate(t *testing.T) {
	opt := WithRate(100, 200)
	rl := &rateLimiter{}
	opt(rl)
	assert.Equal(t, float64(100), float64(rl.limiter.Limit()))
	assert.Equal(t, 200, rl.limiter.Burst())
}

func TestWithKeyExtractor(t *testing.T) {
	customExtractor := func(r *http.Request) string { return "custom-key" }
	opt := WithKeyExtractor(customExtractor)
	rl := &rateLimiter{}
	opt(rl)
	assert.NotNil(t, rl.keyExtractor)
	
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	result := rl.keyExtractor(req)
	assert.Equal(t, "custom-key", result)
}

func TestWithSkipPaths(t *testing.T) {
	opt := WithSkipPaths("/health", "/metrics")
	rl := &rateLimiter{skipPaths: map[string]struct{}{}}
	opt(rl)
	assert.Contains(t, rl.skipPaths, "/health")
	assert.Contains(t, rl.skipPaths, "/metrics")
}

func TestWithMessage(t *testing.T) {
	opt := WithMessage("too many", "RATE_EXCEEDED")
	rl := &rateLimiter{}
	opt(rl)
	assert.Equal(t, "too many", rl.msg)
	assert.Equal(t, "RATE_EXCEEDED", rl.code)
}

func TestNewRateLimiterDefaults(t *testing.T) {
	middleware := NewRateLimiter()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	assert.NotNil(t, handler)
}

func TestNewRateLimiterSkipPath(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	
	middleware := NewRateLimiter(WithSkipPaths("/health"))
	handler := middleware(next)
	
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	assert.True(t, called)
}

func TestNewRateLimiterAllowsRequest(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	
	// High rate limit
	middleware := NewRateLimiter(WithRate(1000, 1000))
	handler := middleware(next)
	
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	assert.True(t, called)
}

func TestStaticAuthNoHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	
	handler := StaticAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/static/file.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	assert.False(t, called)
	// Should return 401
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStaticAuthWrongToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	
	// Set env for expected token
	handler := StaticAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/static/file.js", nil)
	req.Header.Set("secret", "wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	assert.False(t, called)
}

func TestUnauthorizedInvalidToken(t *testing.T) {
	rec := httptest.NewRecorder()
	unauthorizedInvalidToken(rec, "trace-123")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
