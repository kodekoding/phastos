package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_AllowsRequestsWithinBurst(t *testing.T) {
	handler := NewRateLimiter(WithRate(1, 3))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should pass", i+1)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "4th request should be rate limited")
}

func TestRateLimiter_SkipPaths(t *testing.T) {
	handler := NewRateLimiter(
		WithRate(1, 1),
		WithSkipPaths("/health"),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestRateLimiter_PerIP(t *testing.T) {
	handler := NewRateLimiter(WithRate(1, 1))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "5.6.7.8:5678"

	handler.ServeHTTP(httptest.NewRecorder(), req1)
	handler.ServeHTTP(httptest.NewRecorder(), req2)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req1)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimiter_Concurrent(t *testing.T) {
	// Use a very low RPS so tokens replenish slowly during the test.
	// With burst=200 and 500 goroutines competing, the high RPS would
	// allow too many through due to replenishment. Using RPS=1 means
	// tokens barely replenish during the test, so we can assert
	// okCount is bounded reasonably.
	burst := 200
	handler := NewRateLimiter(WithRate(1, burst))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var okCount, limitedCount int64
	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
			if rr.Code == http.StatusOK {
				atomic.AddInt64(&okCount, 1)
			} else if rr.Code == http.StatusTooManyRequests {
				atomic.AddInt64(&limitedCount, 1)
			}
		}()
	}
	wg.Wait()

	// With RPS=1 and burst=200, tokens replenish ~1/sec, so in the
	// ~millisecond the test runs, at most 1 extra token is added.
	assert.LessOrEqual(t, okCount, int64(burst+5), "okCount should be near burst")
	assert.Greater(t, limitedCount, int64(0), "some requests should be limited")
	assert.Equal(t, int64(500), okCount+limitedCount, "all requests should be accounted for")
}

func TestRateLimiter_SequentialDrain(t *testing.T) {
	handler := NewRateLimiter(WithRate(1, 5))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should pass", i+1)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "6th request should be rate limited")
}

func TestRateLimiter_WithMessage(t *testing.T) {
	handler := NewRateLimiter(
		WithRate(1, 1),
		WithMessage("custom limit msg", "CUSTOM_LIMITED"),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	var body map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "custom limit msg", body["message"])
	assert.Equal(t, "CUSTOM_LIMITED", body["code"])
}

func TestRateLimiter_WithKeyExtractor(t *testing.T) {
	called := false
	extractor := func(r *http.Request) string {
		called = true
		return "fixed-key"
	}
	handler := NewRateLimiter(
		WithRate(1, 1),
		WithKeyExtractor(extractor),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.True(t, called, "custom key extractor should be called")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDefaultKeyExtractor_XForwardedFor(t *testing.T) {
	extractor := defaultKeyExtractor()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1")

	key := extractor(r)
	assert.Equal(t, "10.0.0.1", key)
}

func TestDefaultKeyExtractor_XForwardedForMultiple(t *testing.T) {
	extractor := defaultKeyExtractor()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2, 10.0.0.3")

	key := extractor(r)
	assert.Equal(t, "10.0.0.1", key, "should use first IP from X-Forwarded-For")
}

func TestDefaultKeyExtractor_XRealIP(t *testing.T) {
	extractor := defaultKeyExtractor()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Real-Ip", "10.0.0.5")

	key := extractor(r)
	assert.Equal(t, "10.0.0.5", key)
}

func TestDefaultKeyExtractor_RemoteAddr(t *testing.T) {
	extractor := defaultKeyExtractor()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.1:8080"

	key := extractor(r)
	assert.Equal(t, "192.168.1.1", key, "should strip port from RemoteAddr")
}

func TestDefaultKeyExtractor_RemoteAddrNoPort(t *testing.T) {
	extractor := defaultKeyExtractor()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.1"

	key := extractor(r)
	assert.Equal(t, "192.168.1.1", key, "should return RemoteAddr as-is if no port")
}

func TestNewRateLimiter_DefaultValues(t *testing.T) {
	handler := NewRateLimiter()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 20; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}
