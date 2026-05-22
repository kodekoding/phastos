package context

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/stretchr/testify/assert"
)

// stubCaches implements cache.Caches for testing.
type stubCaches struct{}

func (s *stubCaches) Get(ctx context.Context, key string, typeDestination any, fallbackFn ...cache.FallbackFn) error {
	return nil
}
func (s *stubCaches) Del(ctx context.Context, key string) (int64, error) { return 0, nil }
func (s *stubCaches) Set(ctx context.Context, key string, value any, expire ...int) error {
	return nil
}
func (s *stubCaches) HSet(ctx context.Context, key, field string, value any, expire ...int) error {
	return nil
}
func (s *stubCaches) HGet(ctx context.Context, key, field string, typeDestination any, fallbackFn ...cache.FallbackFn) error {
	return nil
}
func (s *stubCaches) HDel(ctx context.Context, key, field string) error { return nil }

func TestCreateAsyncContext_EmptyContext(t *testing.T) {
	t.Run("should return background context when source context is empty", func(t *testing.T) {
		ctx := context.Background()
		asyncCtx := CreateAsyncContext(ctx)

		// No values should be present
		assert.Nil(t, GetJWT(asyncCtx))
		assert.Nil(t, GetNotif(asyncCtx))
		assert.Equal(t, "", GetNotifDestination(asyncCtx))
	})
}

func TestCreateAsyncContext_WithJWT(t *testing.T) {
	t.Run("should propagate JWT to async context", func(t *testing.T) {
		req := httptestRequest()
		jwtData := &entity.JWTClaimData{Data: "async-user", Token: "async-token"}
		SetJWT(req, jwtData)

		asyncCtx := CreateAsyncContext(req.Context())
		result := GetJWT(asyncCtx)
		assert.NotNil(t, result)
		assert.Equal(t, "async-token", result.Token)
	})
}

func TestCreateAsyncContext_WithNotif(t *testing.T) {
	t.Run("should propagate notif platform to async context", func(t *testing.T) {
		req := httptestRequest()
		platform := &stubPlatforms{}
		SetNotif(req, platform)

		asyncCtx := CreateAsyncContext(req.Context())
		result := GetNotif(asyncCtx)
		assert.NotNil(t, result)
	})
}

func TestCreateAsyncContext_WithNotifDestination(t *testing.T) {
	t.Run("should not propagate notif destination (only notif platform is propagated)", func(t *testing.T) {
		req := httptestRequest()
		SetNotifDestination(req, "#alerts")

		asyncCtx := CreateAsyncContext(req.Context())
		// NotifDestination is not propagated by CreateAsyncContext
		result := GetNotifDestination(asyncCtx)
		assert.Equal(t, "", result)
	})
}

func TestCreateAsyncContext_WithCache(t *testing.T) {
	t.Run("should propagate cache to async context", func(t *testing.T) {
		req := httptestRequest()
		cacheInstance := &stubCaches{}
		ctx := context.WithValue(req.Context(), entity.CacheContext{}, cacheInstance)

		asyncCtx := CreateAsyncContext(ctx)
		cacheResult := asyncCtx.Value(entity.CacheContext{})
		assert.NotNil(t, cacheResult)
	})
}

func TestCreateAsyncContext_WithTraceId(t *testing.T) {
	t.Run("should propagate traceId to async context", func(t *testing.T) {
		req := httptestRequest()
		ctx := context.WithValue(req.Context(), "traceId", "trace-123")

		asyncCtx := CreateAsyncContext(ctx)
		traceId, ok := asyncCtx.Value("traceId").(string)
		assert.True(t, ok)
		assert.Equal(t, "trace-123", traceId)
	})
}

func TestCreateAsyncContext_WithNoTraceId(t *testing.T) {
	t.Run("should not have traceId in async context when not set", func(t *testing.T) {
		ctx := context.Background()
		asyncCtx := CreateAsyncContext(ctx)
		traceId := asyncCtx.Value("traceId")
		assert.Nil(t, traceId)
	})
}

func TestCreateAsyncContext_WithWrongTypeTraceId(t *testing.T) {
	t.Run("should not propagate traceId when value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "traceId", 12345)
		asyncCtx := CreateAsyncContext(ctx)
		traceId, ok := asyncCtx.Value("traceId").(string)
		assert.False(t, ok)
		assert.Equal(t, "", traceId)
	})
}

func TestCreateAsyncContext_WithAllValues(t *testing.T) {
	t.Run("should propagate JWT, notif, cache, and traceId together", func(t *testing.T) {
		req := httptestRequest()
		jwtData := &entity.JWTClaimData{Data: "full-user", Token: "full-token"}
		SetJWT(req, jwtData)

		platform := &stubPlatforms{}
		SetNotif(req, platform)

		cacheInstance := &stubCaches{}
		ctx := context.WithValue(req.Context(), entity.CacheContext{}, cacheInstance)
		ctx = context.WithValue(ctx, "traceId", "trace-full")

		asyncCtx := CreateAsyncContext(ctx)

		// All values should be present
		jwtResult := GetJWT(asyncCtx)
		assert.NotNil(t, jwtResult)
		assert.Equal(t, "full-token", jwtResult.Token)

		notifResult := GetNotif(asyncCtx)
		assert.NotNil(t, notifResult)

		cacheResult := asyncCtx.Value(entity.CacheContext{})
		assert.NotNil(t, cacheResult)

		traceId, ok := asyncCtx.Value("traceId").(string)
		assert.True(t, ok)
		assert.Equal(t, "trace-full", traceId)
	})
}

func TestCreateAsyncContext_WithNewRelicTransaction(t *testing.T) {
	t.Run("should propagate New Relic transaction to async context", func(t *testing.T) {
		nr := monitoring.InitNewRelic(
			monitoring.WithAppName("test-async-nr"),
			monitoring.WithLicenseKey("0123456789012345678901234567890123456789"),
		)
		if nr == nil || nr.GetApp() == nil {
			t.Skip("New Relic not available")
		}

		txn := nr.GetApp().StartTransaction("test")
		ctx := monitoring.NewContext(context.Background(), txn)

		asyncCtx := CreateAsyncContext(ctx)
		assert.NotNil(t, asyncCtx)
	})
}

// httptestRequest is a helper to create a new http.Request for tests.
func httptestRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/test", nil)
}
