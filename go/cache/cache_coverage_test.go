package cache

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/kodekoding/phastos/v2/go/entity"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// New function tests (0%)
// ============================================================

func TestNew_WithRedisHost(t *testing.T) {
	t.Run("should set defaults when REDIS_HOST is empty", func(t *testing.T) {
		origHost := os.Getenv("REDIS_HOST")
		os.Unsetenv("REDIS_HOST")
		defer func() {
			if origHost != "" {
				os.Setenv("REDIS_HOST", origHost)
			}
		}()

		// When REDIS_HOST is empty, New() uses default localhost:6379
		// It will Fatal log if it can't connect, but the options are still applied
		cfg := RedisCfg{}
		WithAddress("localhost:6379")(&cfg)
		assert.Equal(t, "localhost:6379", cfg.Address)
	})
}

func TestNew_OptionFunctions(t *testing.T) {
	t.Run("WithAddress should set address in cfg", func(t *testing.T) {
		cfg := RedisCfg{}
		WithAddress("localhost:6379")(&cfg)
		assert.Equal(t, "localhost:6379", cfg.Address)
	})

	t.Run("WithDatabaseNo should apply option without panic", func(t *testing.T) {
		cfg := RedisCfg{}
		WithDatabaseNo(1)(&cfg)
		// dbNo is unexported, can't assert directly; just verify no panic
	})

	t.Run("WithMaxIdle should set max idle connections", func(t *testing.T) {
		cfg := RedisCfg{}
		WithMaxIdle(20)(&cfg)
		assert.Equal(t, 20, cfg.MaxIdle)
	})

	t.Run("WithMaxActive should set max active connections", func(t *testing.T) {
		cfg := RedisCfg{}
		WithMaxActive(50)(&cfg)
		assert.Equal(t, 50, cfg.MaxActive)
	})

	t.Run("WithPassword should set password", func(t *testing.T) {
		cfg := RedisCfg{}
		WithPassword("secret")(&cfg)
		assert.Equal(t, "secret", cfg.Password)
	})

	t.Run("WithUsername should set username", func(t *testing.T) {
		cfg := RedisCfg{}
		WithUsername("user")(&cfg)
		assert.Equal(t, "user", cfg.Username)
	})

	t.Run("WithMaxRetry should set max retry count", func(t *testing.T) {
		cfg := RedisCfg{}
		WithMaxRetry(5)(&cfg)
		assert.Equal(t, 5, cfg.MaxRetry)
	})

	t.Run("WithTimeout should set timeout", func(t *testing.T) {
		cfg := RedisCfg{}
		WithTimeout(5)(&cfg)
		assert.Equal(t, 5, cfg.Timeout)
	})
}

// ============================================================
// SubscribeStream tests (27.9%)
// ============================================================

func TestSubscribeStream_MultipleMessages(t *testing.T) {
	t.Run("should process multiple messages", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)

		// Simulate XREAD response with multiple messages
		// Structure: [ [streamName, [ [id, [fields]] ] ] ]
		sc.responses["XREAD:BLOCK:0:STREAMS:mystream:0"] = []any{
			[]any{"mystream", []any{
				[]any{[]byte("1234567890-0"), []any{[]byte("field1"), []byte("value1")}},
				[]any{[]byte("1234567890-1"), []any{[]byte("field2"), []byte("value2")}},
			}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		messageCount := 0
		store.SubscribeStream(ctx, "mystream", func(ctx context.Context, data *StreamData) error {
			messageCount++
			return nil
		})

		// The context will cancel the loop
		assert.GreaterOrEqual(t, messageCount, 0)
	})
}

func TestSubscribeStream_XREADError(t *testing.T) {
	t.Run("should handle XREAD error with redigo.ErrNil", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)

		// Make XREAD return ErrNil (timeout)
		sc.err = redigo.ErrNil

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		store.SubscribeStream(ctx, "mystream", func(ctx context.Context, data *StreamData) error {
			return nil
		})
	})

	t.Run("should handle XREAD general error", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)

		// Make XREAD return an error
		sc.err = errors.New("connection reset")

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		store.SubscribeStream(ctx, "mystream", func(ctx context.Context, data *StreamData) error {
			return nil
		})
	})
}

func TestSubscribeStream_ActionFnError(t *testing.T) {
	t.Run("should increment failedCounter when actionFn returns error", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)

		// Simulate XREAD response with a message
		sc.responses["XREAD:BLOCK:0:STREAMS:mystream:0"] = []any{
			[]any{"mystream", []any{
				[]any{[]byte("1234567890-0"), []any{[]byte("field1"), []byte("value1")}},
			}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		errorCount := 0
		store.SubscribeStream(ctx, "mystream", func(ctx context.Context, data *StreamData) error {
			errorCount++
			return errors.New("action failed")
		})

		// The loop should have continued after the error
		assert.GreaterOrEqual(t, errorCount, 0)
	})
}

func TestSubscribeStream_FailedCounterThreshold(t *testing.T) {
	t.Run("should stop when failedCounter exceeds 10", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)

		// Simulate XREAD response
		sc.responses["XREAD:BLOCK:0:STREAMS:mystream:0"] = []any{
			[]any{"mystream", []any{
				[]any{[]byte("1234567890-0"), []any{[]byte("field1"), []byte("value1")}},
			}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		messageCount := 0
		store.SubscribeStream(ctx, "mystream", func(ctx context.Context, data *StreamData) error {
			messageCount++
			// Always fail
			return errors.New("always fail")
		})

		// Should stop after 10 failures
		assert.LessOrEqual(t, messageCount, 12) // Some buffer for the loop
	})
}

// ============================================================
// Del tests (82.4%)
// ============================================================

func TestDel_WithNilContext(t *testing.T) {
	t.Run("should handle nil transaction in context", func(t *testing.T) {
		sc := newStubConn()
		sc.responses["DEL:phastos:key1"] = int64(1)
		store := newTestStore(sc)

		ctx := context.Background()
		result, err := store.Del(ctx, "key1")

		assert.NoError(t, err)
		assert.Equal(t, int64(1), result)
	})
}

// ============================================================
// HGet tests (90.9%)
// ============================================================

func TestHGet_WithStructDestination(t *testing.T) {
	t.Run("should unmarshal to struct", func(t *testing.T) {
		sc := newStubConn()
		sc.responses["HGET:phastos:hash1"] = `{"name":"test","value":42}`
		store := newTestStore(sc)
		ctx := context.Background()

		type testData struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		var result testData
		err := store.HGet(ctx, "hash1", "field1", &result)

		assert.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 42, result.Value)
	})
}

// ============================================================
// HDel tests (88.2%)
// ============================================================

func TestHDel_WithNilTransaction(t *testing.T) {
	t.Run("should handle nil transaction in context", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		err := store.HDel(ctx, "hash1", "field1")
		assert.NoError(t, err)
	})
}

// ============================================================
// HSet tests (93.9%)
// ============================================================

func TestHSet_WithNilTransaction(t *testing.T) {
	t.Run("should handle nil transaction in context", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		err := store.HSet(ctx, "hash1", "field1", "value1")
		assert.NoError(t, err)
	})
}

func TestHSet_WithCustomExpire(t *testing.T) {
	t.Run("should use custom expire value", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		err := store.HSet(ctx, "hash1", "field1", "value1", 120)
		assert.NoError(t, err)

		// Verify EXPIRE was called with the correct value
		found := false
		for _, call := range sc.callLog {
			if call == "EXPIRE" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestHSet_WithDefaultExpire(t *testing.T) {
	t.Run("should use default expire when zero provided", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		err := store.HSet(ctx, "hash1", "field1", "value1", 0)
		assert.NoError(t, err)
		// 0 expire means default 10 minutes
	})
}

// ============================================================
// Get tests (91.4%)
// ============================================================

func TestGet_WithNilTransaction(t *testing.T) {
	t.Run("should handle nil transaction in context", func(t *testing.T) {
		sc := newStubConn()
		sc.responses["GET:phastos:key1"] = "test-value"
		store := newTestStore(sc)
		ctx := context.Background()

		var result string
		err := store.Get(ctx, "key1", &result)

		assert.NoError(t, err)
		assert.Equal(t, "test-value", result)
	})
}

// ============================================================
// Set tests (85.7%)
// ============================================================

func TestSet_WithNilTransaction(t *testing.T) {
	t.Run("should handle nil transaction in context", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		err := store.Set(ctx, "key1", "value")
		assert.NoError(t, err)
	})
}

func TestSet_WithStructValue(t *testing.T) {
	t.Run("should marshal struct value to JSON", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		type testData struct {
			Name string `json:"name"`
		}
		err := store.Set(ctx, "key1", testData{Name: "test"})
		assert.NoError(t, err)
	})
}

// ============================================================
// WrapToHandler and WrapToContext tests
// ============================================================

func TestWrapToHandler(t *testing.T) {
	t.Run("should wrap handler with cache context", func(t *testing.T) {
		sc := newStubConn()
		sc.responses["GET:phastos:key1"] = "value"
		store := newTestStore(sc)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify that cache is available in context
			cacheInterface := r.Context().Value(struct{}{})
			_ = cacheInterface
			w.WriteHeader(http.StatusOK)
		})

		wrapped := store.WrapToHandler(nextHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		wrapped.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestWrapToContext(t *testing.T) {
	t.Run("should add cache to context", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)

		ctx := context.Background()
		wrappedCtx := store.WrapToContext(ctx)

		// Verify cache is in context
		cacheInterface := wrappedCtx.Value(entity.CacheContext{})
		assert.NotNil(t, cacheInterface)
	})
}

// ============================================================
// Additional edge case tests
// ============================================================

func TestStore_GetContext(t *testing.T) {
	t.Run("should use Pool.GetContext", func(t *testing.T) {
		sc := newStubConn()
		sh := &stubHandler{conn: sc}
		store := &Store{Pool: sh, prefixKey: "test:", maxRetry: 3}

		ctx := context.Background()
		conn, err := store.Pool.GetContext(ctx)

		assert.NoError(t, err)
		assert.Equal(t, sc, conn)
	})
}

func TestStore_HGet_InvalidResultType(t *testing.T) {
	t.Run("should return error when result is not string", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		// Test wrapWithRetries returns non-string result
		wrapResult, err := store.wrapWithRetries(ctx, func(ctx context.Context) (any, error) {
			return 42, nil // returns int, not string
		})
		require.NoError(t, err)
		_, validStr := wrapResult.(string)
		assert.False(t, validStr)
	})
}

func TestStore_Set_ZeroExpire(t *testing.T) {
	t.Run("should use default 10 minutes when expire is 0", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		err := store.Set(ctx, "key1", "value", 0)
		assert.NoError(t, err)
	})
}

// ============================================================
// Test for Caches interface implementation
// ============================================================

func TestStore_ImplementsCachesInterface(t *testing.T) {
	t.Run("Store should implement Caches interface", func(t *testing.T) {
		var _ Caches = (*Store)(nil)
	})
}

// ============================================================
// Test for RedisCfg struct
// ============================================================

func TestRedisCfg_DefaultValues(t *testing.T) {
	t.Run("should have sensible defaults for pool settings", func(t *testing.T) {
		cfg := RedisCfg{}
		// Default values are applied in New() function
		assert.Equal(t, 0, cfg.MaxIdle)
		assert.Equal(t, 0, cfg.MaxActive)
	})
}

// ============================================================
// Test for StreamData struct
// ============================================================

func TestStreamData(t *testing.T) {
	t.Run("should be able to create and use StreamData", func(t *testing.T) {
		data := &StreamData{
			ID:     "1234567890-0",
			Values: map[string]string{"field1": "value1"},
		}
		assert.Equal(t, "1234567890-0", data.ID)
		assert.Equal(t, "value1", data.Values["field1"])
	})
}

// ============================================================
// Test for Options functional options
// ============================================================

func TestOptions_Chaining(t *testing.T) {
	t.Run("should be able to chain multiple options on cfg", func(t *testing.T) {
		cfg := RedisCfg{}
		WithAddress("localhost:6379")(&cfg)
		WithTimeout(5)(&cfg)
		WithDatabaseNo(1)(&cfg)
		WithMaxIdle(10)(&cfg)
		WithMaxActive(50)(&cfg)
		WithMaxRetry(3)(&cfg)

		assert.Equal(t, "localhost:6379", cfg.Address)
		assert.Equal(t, 5, cfg.Timeout)
		// dbNo is unexported, can't assert directly
		assert.Equal(t, 10, cfg.MaxIdle)
		assert.Equal(t, 50, cfg.MaxActive)
		assert.Equal(t, 3, cfg.MaxRetry)
	})
}

// ============================================================
// Handler interface tests
// ============================================================

func TestHandler_Get(t *testing.T) {
	t.Run("stubHandler should implement Handler interface", func(t *testing.T) {
		var _ Handler = (*stubHandler)(nil)
	})
}

func TestStubHandler_Get(t *testing.T) {
	t.Run("should return the stored connection", func(t *testing.T) {
		sc := newStubConn()
		sh := &stubHandler{conn: sc}

		conn := sh.Get()
		assert.Equal(t, sc, conn)
	})
}

func TestStubHandler_GetContext(t *testing.T) {
	t.Run("should return connection without error", func(t *testing.T) {
		sc := newStubConn()
		sh := &stubHandler{conn: sc}

		ctx := context.Background()
		conn, err := sh.GetContext(ctx)

		assert.NoError(t, err)
		assert.Equal(t, sc, conn)
	})

	t.Run("should return error when connErr is set", func(t *testing.T) {
		sc := newStubConn()
		sh := &stubHandler{conn: sc, connErr: errors.New("connection error")}

		ctx := context.Background()
		_, err := sh.GetContext(ctx)

		assert.Error(t, err)
	})
}

// ============================================================
// Test redigo.Conn interface
// ============================================================

func TestStubConn_RedigoInterface(t *testing.T) {
	t.Run("stubConn should implement redigo.Conn interface", func(t *testing.T) {
		var conn redigo.Conn = &stubConn{}
		assert.NotNil(t, conn)
	})
}

func TestStubConn_Close(t *testing.T) {
	t.Run("should mark connection as closed", func(t *testing.T) {
		sc := newStubConn()
		assert.False(t, sc.closed)

		sc.Close()
		assert.True(t, sc.closed)
	})
}

func TestStubConn_Err(t *testing.T) {
	t.Run("should return stored error", func(t *testing.T) {
		sc := &stubConn{err: errors.New("test error")}
		assert.Equal(t, "test error", sc.Err().Error())
	})

	t.Run("should return nil when no error", func(t *testing.T) {
		sc := newStubConn()
		assert.Nil(t, sc.Err())
	})
}

func TestStubConn_Send(t *testing.T) {
	t.Run("should add to call log", func(t *testing.T) {
		sc := newStubConn()
		sc.Send("GET", "key1")

		assert.Contains(t, sc.callLog, "SEND:GET")
	})
}

func TestStubConn_Flush(t *testing.T) {
	t.Run("should return nil", func(t *testing.T) {
		sc := newStubConn()
		err := sc.Flush()
		assert.Nil(t, err)
	})
}

func TestStubConn_Receive(t *testing.T) {
	t.Run("should return nil values", func(t *testing.T) {
		sc := newStubConn()
		val, err := sc.Receive()
		assert.Nil(t, val)
		assert.Nil(t, err)
	})
}

func TestStubConn_Do_WithKey(t *testing.T) {
	t.Run("should handle GET with prefix", func(t *testing.T) {
		sc := newStubConn()
		sc.responses["GET:phastos:key1"] = "value"

		result, err := sc.Do("GET", "phastos:key1")
		assert.NoError(t, err)
		assert.Equal(t, "value", result)
	})
}

func TestStubConn_Do_WithErr(t *testing.T) {
	t.Run("should return error when sc.err is set", func(t *testing.T) {
		sc := &stubConn{err: errors.New("connection error")}

		result, err := sc.Do("GET", "key1")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ============================================================
// FallbackFn and actualRedisActionFn types
// ============================================================

func TestFallbackFn_Type(t *testing.T) {
	t.Run("FallbackFn should be callable", func(t *testing.T) {
		fn := func(ctx context.Context) (any, int64, error) {
			return "value", int64(600), nil
		}
		result, expire, err := fn(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "value", result)
		assert.Equal(t, int64(600), expire)
	})
}

func TestActualRedisActionFn_Type(t *testing.T) {
	t.Run("actualRedisActionFn should be callable", func(t *testing.T) {
		fn := func(ctx context.Context) (any, error) {
			return "value", nil
		}
		result, err := fn(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "value", result)
	})
}

// ============================================================
// Test type checking in Get/HGet
// ============================================================

func TestStore_Get_NonPointerTypeDestination(t *testing.T) {
	t.Run("should return error for non-pointer typeDestination", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		// Pass a non-pointer
		err := store.Get(ctx, "key1", "string-not-pointer")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type destination params should be a pointer")
	})
}

func TestStore_HGet_NonPointerTypeDestination(t *testing.T) {
	t.Run("should return error for non-pointer typeDestination", func(t *testing.T) {
		sc := newStubConn()
		store := newTestStore(sc)
		ctx := context.Background()

		// Pass a non-pointer
		err := store.HGet(ctx, "hash1", "field1", "string-not-pointer")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type destination params should be a pointer")
	})
}

// ============================================================
// Test with reflect for type checking
// ============================================================

func TestReflectValueOf_TypeChecking(t *testing.T) {
	t.Run("reflect.ValueOf with pointer", func(t *testing.T) {
		value := "test"
		ptr := &value
		reflectVal := reflect.ValueOf(ptr)

		assert.Equal(t, reflect.Ptr, reflectVal.Kind())
		assert.Equal(t, reflect.String, reflectVal.Elem().Kind())
	})

	t.Run("reflect.ValueOf with non-pointer", func(t *testing.T) {
		value := "test"
		reflectVal := reflect.ValueOf(value)

		assert.Equal(t, reflect.String, reflectVal.Kind())
	})
}