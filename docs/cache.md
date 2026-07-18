# Redis Cache

Phastos Redis cache module provides a high-level wrapper around `redigo` with built-in connection pooling,
fallback pattern with singleflight deduplication, monitoring spans, and Redis Streams support.

## Connection

Create a cache store with `cache.New()` using functional options:

```go
import "github.com/kodekoding/phastos/v2/go/cache"

store := cache.New(
    cache.WithAddress("localhost:6379"),
    cache.WithDatabaseNo(0),
)
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithAddress` | `string` | `""` | Redis server address (`host:port`) |
| `WithDatabaseNo` | `int` | `0` | Redis database number |
| `WithTimeout` | `int` | `0` | Connection/read/write timeout in seconds |
| `WithMaxActive` | `int` | `MaxIdle * 5` | Maximum active connections in pool |
| `WithMaxIdle` | `int` | `10` | Maximum idle connections in pool |
| `WithMaxRetry` | `int` | `10` | Max retries on `ErrPoolExhausted` (1s backoff) |
| `WithPassword` | `string` | `""` | Redis password (AUTH) |
| `WithUsername` | `string` | `""` | Redis username (ACL) |

### Key Prefix

All keys are automatically prefixed. The prefix is read from the `REDIS_PREFIX_KEY` environment variable,
defaulting to `phastos:`.

### Connection Pool

Uses `redigo.Pool` with:
- `TestOnBorrow`: sends `PING` to validate connections before use
- `MaxRetry` on `ErrPoolExhausted` with 1-second backoff between retries
- On `New()`, a PING is sent to verify connectivity — logs fatal if unreachable

## Basic Operations

### Get

```go
err := store.Get(ctx, "user:1", &myStruct)
```

- Auto JSON unmarshals into `typeDestination` (must be a pointer)
- If the destination is `*string`, returns the raw string without JSON unmarshal
- Returns `redigo.ErrNil` for cache miss (unless fallback is provided)

### Set

```go
err := store.Set(ctx, "user:1", myStruct)           // default 10 min TTL
err := store.Set(ctx, "user:1", myStruct, 3600)     // 1 hour TTL
err := store.Set(ctx, "user:1", "string-value", 60) // raw string
```

- Non-string values are auto JSON marshalled
- Default expiry: 10 minutes
- Uses Redis `SET ... EX` command

### Del

```go
deleted, err := store.Del(ctx, "user:1")
```

- Returns count of deleted keys (`int64`)

### HGet

```go
err := store.HGet(ctx, "user:1", "name", &name)
err := store.HGet(ctx, "user:1", "profile", &profile)
```

- Gets a single field from a hash
- Auto JSON unmarshals into `typeDestination` (must be a pointer)
- If the destination is `*string`, returns raw string
- Supports fallback (see Fallback Pattern below)

### HSet

```go
err := store.HSet(ctx, "user:1", "name", "Raditya")          // no expiry by default
err := store.HSet(ctx, "user:1", "profile", profile, 3600)   // 1 hour TTL
```

- Sets a single field in a hash
- Non-string values are auto JSON marshalled
- If `expire` is provided and > 0, sets `EXPIRE` on the key

### HDel

```go
err := store.HDel(ctx, "user:1", "name")
```

- Deletes a single field from a hash

### HGetAll

```go
// Into a string map
var fields map[string]string
err := store.HGetAll(ctx, "user:1", &fields)

// Into a struct (JSON unmarshalled)
var profile UserProfile
err := store.HGetAll(ctx, "user:1", &profile)
```

- Gets all fields of a hash
- `dest` must be a pointer
- If `dest` is `*map[string]string`, returns raw string map
- Otherwise, marshals to JSON then unmarshals into `dest`

### HSetBulk

```go
err := store.HSetBulk(ctx, "user:1", map[string]interface{}{
    "name":    "Raditya",
    "email":   "raditya@example.com",
    "role":    "admin",
}, 3600) // 1 hour TTL
```

- Sets multiple hash fields in one pipelined operation
- Non-string values are auto JSON marshalled
- Optional `expire` parameter sets key TTL after all fields are written

## Fallback Pattern

When a cache miss occurs, the fallback function is called to fetch the data from the primary source
(e.g. database, API), auto-caches the result, and returns it. Concurrent requests for the same key
are deduplicated via `singleflight`.

### FallbackFn Signature

```go
type FallbackFn func(ctx context.Context) (result any, expire int64, err error)
```

| Return | Description |
|--------|-------------|
| `result` | The value to cache and return (any type — auto marshalled) |
| `expire` | Cache TTL in seconds (0 = defaults to 10 minutes) |
| `err` | Error from the primary source lookup |

### Get with Fallback

```go
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

var user User
err := store.Get(ctx, "user:1", &user, func(ctx context.Context) (any, int64, error) {
    // Called only on cache miss
    u, err := userRepo.FindByID(ctx, 1)
    if err != nil {
        return nil, 0, err
    }
    return u, 300, nil // cache for 5 minutes
})
```

### HGet with Fallback

```go
var profile Profile
err := store.HGet(ctx, "user:1", "profile", &profile, func(ctx context.Context) (any, int64, error) {
    p, err := profileRepo.GetByUserID(ctx, 1)
    if err != nil {
        return nil, 0, err
    }
    return p, 600, nil // cache for 10 minutes
})
```

### Singleflight Deduplication

If multiple concurrent requests miss the cache for the same key, only **one** fallback function is
executed. All other callers wait for and receive the same result.

```go
// 100 concurrent requests for "user:1"
// → only 1 DB query executes
// → result cached in Redis
// → all 100 callers receive the same value
```

The singleflight key for `Get` is `{prefix}{key}`; for `HGet` it is `{prefix}{key}:{field}`.

### Fallback Flow

```
Get/HGet → Redis miss → singleflight.Do(sfKey, ...)
  → fallbackFn(ctx) → fetch from DB/API
  → SET key value EX expire (or HSET + EXPIRE)
  → return cached value
```

## Redis Streams

### PublishStream

```go
messageID, err := store.PublishStream(ctx, "orders",
    map[string]any{"action": "created", "order_id": "123"},
)
```

- Accepts variadic `map[string]any` (at least one required)
- Uses `XADD` with `MAXLEN ~ 100` (approximate capping)
- Returns the message ID

### SubscribeStream

```go
go store.SubscribeStream(ctx, "orders", func(ctx context.Context, data *cache.StreamData) error {
    fmt.Println("Message ID:", data.ID)
    fmt.Println("Fields:", data.Values)
    // data.Values["action"] → "created"
    // data.Values["order_id"] → "123"
    return nil
})
```

- Blocking call — run in a goroutine
- Uses `XREAD BLOCK 0` (wait indefinitely for new messages)
- Stops on context cancellation or after 10 consecutive failures
- `StreamData` contains `ID` (message ID) and `Values` (field map)

### Consumer Groups

```go
// Create consumer group (with MKSTREAM — creates stream if not exists)
err := store.XGroupCreateMkStream(ctx, "orders", "order-processors", "0")

// Read pending/new messages as a consumer group member
msgs, err := store.XReadGroup(ctx, "order-processors", "worker-1",
    []string{"orders"}, []string{">"}, 5*time.Second, 10)

for _, stream := range msgs {
    for _, msg := range stream.Messages {
        // Process message
        fmt.Println(msg.ID, msg.Values)

        // Acknowledge
        count, err := store.XAck(ctx, "orders", "order-processors", msg.ID)
        _ = count
    }
}
```

- `XReadGroup`: reads messages with consumer group semantics; `">"` for new messages, `"0"` for pending
- `XAck`: acknowledges a message, returns count of acknowledged messages
- `block` parameter enables blocking read with timeout
- `count` limits max messages returned

## Context Integration

### WrapToHandler (HTTP Middleware)

```go
mux := http.NewServeMux()
mux.Handle("/api/", store.WrapToHandler(apiHandler))
```

Stores the `*Store` in the request context. Use `cache.GetCacheFromContext(ctx)` in handlers.

### WrapToContext

```go
ctx = store.WrapToContext(ctx)
```

Stores the `*Store` in an existing context (useful outside HTTP handlers, e.g. background workers).

### GetCacheFromContext

```go
import "github.com/kodekoding/phastos/v2/go/cache"

func MyHandler(w http.ResponseWriter, r *http.Request) {
    c := cache.GetCacheFromContext(r.Context())
    if c == nil {
        // cache not available in context
        return
    }
    var user User
    err := c.Get(r.Context(), "user:1", &user)
}
```

- Returns `nil` if no cache is found in the context
- Returns the `Caches` interface — use any cache operation

## Caches Interface

```go
type Caches interface {
    Get(ctx context.Context, key string, typeDestination any, fallbackFn ...FallbackFn) error
    Del(ctx context.Context, key string) (int64, error)
    Set(ctx context.Context, key string, value any, expire ...int) error
    HSet(ctx context.Context, key, field string, value any, expire ...int) error
    HGet(ctx context.Context, key, field string, typeDestination any, fallbackFn ...FallbackFn) error
    HDel(ctx context.Context, key, field string) error
    HGetAll(ctx context.Context, key string, dest interface{}) error
    HSetBulk(ctx context.Context, key string, fields map[string]interface{}, expire ...int) error
    XGroupCreateMkStream(ctx context.Context, streamKey, group, startID string) error
    XReadGroup(ctx context.Context, group, consumer string, streams []string, ids []string, block time.Duration, count int64) ([]StreamMessages, error)
    XAck(ctx context.Context, streamKey, group, id string) (int64, error)
}
```
