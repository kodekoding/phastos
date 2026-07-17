package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/stretchr/testify/assert"
)

// stubStore implements Caches for testing
type stubStore struct{}

func (s *stubStore) Get(ctx context.Context, key string, typeDestination any, fallbackFn ...FallbackFn) error {
	return nil
}
func (s *stubStore) Del(ctx context.Context, key string) (int64, error) { return 0, nil }
func (s *stubStore) Set(ctx context.Context, key string, value any, expire ...int) error {
	return nil
}
func (s *stubStore) HSet(ctx context.Context, key, field string, value any, expire ...int) error {
	return nil
}
func (s *stubStore) HGet(ctx context.Context, key, field string, typeDestination any, fallbackFn ...FallbackFn) error {
	return nil
}
func (s *stubStore) HDel(ctx context.Context, key, field string) error           { return nil }
func (s *stubStore) HGetAll(ctx context.Context, key string, dest interface{}) error { return nil }
func (s *stubStore) HSetBulk(ctx context.Context, key string, fields map[string]interface{}, expire ...int) error {
	return nil
}
func (s *stubStore) XGroupCreateMkStream(ctx context.Context, streamKey, group, startID string) error {
	return nil
}
func (s *stubStore) XReadGroup(ctx context.Context, group, consumer string, streams []string, ids []string, block time.Duration, count int64) ([]StreamMessages, error) {
	return nil, nil
}
func (s *stubStore) XAck(ctx context.Context, streamKey, group, id string) (int64, error) {
	return 0, nil
}

func TestGetCacheFromContext_WhenPresent(t *testing.T) {
	ctx := context.Background()
	store := &stubStore{}
	ctx = context.WithValue(ctx, entity.CacheContext{}, store)
	result := GetCacheFromContext(ctx)
	assert.NotNil(t, result)
}

func TestGetCacheFromContext_WhenAbsent(t *testing.T) {
	ctx := context.Background()
	result := GetCacheFromContext(ctx)
	assert.Nil(t, result)
}

func TestGetCacheFromContext_WhenWrongType(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, entity.CacheContext{}, "not-a-cache")
	result := GetCacheFromContext(ctx)
	assert.Nil(t, result)
}

func TestRedisCfgDefaults(t *testing.T) {
	cfg := RedisCfg{}
	assert.Equal(t, "", cfg.Address)
	assert.Equal(t, 0, cfg.MaxIdle)
	assert.Equal(t, 0, cfg.MaxActive)
	assert.Equal(t, 0, cfg.MaxRetry)
}

func TestOptionFunctions(t *testing.T) {
	cfg := RedisCfg{}
	
	WithAddress("localhost:6379")(&cfg)
	assert.Equal(t, "localhost:6379", cfg.Address)
	
	WithTimeout(5)(&cfg)
	assert.Equal(t, 5, cfg.Timeout)
	
	WithMaxActive(50)(&cfg)
	assert.Equal(t, 50, cfg.MaxActive)
	
	WithMaxIdle(10)(&cfg)
	assert.Equal(t, 10, cfg.MaxIdle)
	
	WithMaxRetry(3)(&cfg)
	assert.Equal(t, 3, cfg.MaxRetry)
	
	WithPassword("secret")(&cfg)
	assert.Equal(t, "secret", cfg.Password)
	
	WithUsername("admin")(&cfg)
	assert.Equal(t, "admin", cfg.Username)
	
	WithDatabaseNo(2)(&cfg)
	assert.Equal(t, 2, cfg.dbNo)
}

func TestStoreWrapToContext(t *testing.T) {
	// Test the WrapToContext method without actually connecting to Redis
	store := &Store{
		Pool:      nil, // not used in WrapToContext
		prefixKey: "test:",
		maxRetry:  3,
	}
	ctx := context.Background()
	ctx = store.WrapToContext(ctx)
	val := ctx.Value(entity.CacheContext{})
	assert.NotNil(t, val)
	
	cacheVal, ok := val.(*Store)
	assert.True(t, ok)
	assert.Equal(t, "test:", cacheVal.prefixKey)
}

func TestStoreWrapToHandler(t *testing.T) {
	store := &Store{
		Pool:      nil,
		prefixKey: "test:",
		maxRetry:  3,
	}
	
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		cache := GetCacheFromContext(r.Context())
		assert.NotNil(t, cache)
	})
	
	handler := store.WrapToHandler(next)
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	assert.True(t, called)
}

func TestDefaultPrefixKey(t *testing.T) {
	assert.Equal(t, "phastos:", defaultPrefixKey)
}

func TestStreamDataStruct(t *testing.T) {
	sd := StreamData{
		ID:     "123",
		Values: map[string]string{"key": "val"},
	}
	assert.Equal(t, "123", sd.ID)
	assert.Equal(t, "val", sd.Values["key"])
}
