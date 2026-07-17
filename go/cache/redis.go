package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/kodekoding/phastos/v2/go/entity"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/singleflight"
)

// Store object
type Store struct {
	Pool      Handler
	prefixKey string
	maxRetry  int
	sf        *singleflight.Group
}

type Options func(*RedisCfg)

type FallbackFn func(ctx context.Context) (result any, expire int64, err error)
type actualRedisActionFn func(ctx context.Context) (result any, err error)

type RedisCfg struct {
	Address   string `yaml:"address"`
	Timeout   int    `yaml:"timeout"`
	MaxIdle   int    `yaml:"max_iddle"`
	MaxActive int    `yaml:"max_active"`
	Password  string `yaml:"password"`
	Username  string `yaml:"username"`
	MaxRetry  int
	dbNo      int
}

type StreamData struct {
	ID     string
	Values map[string]string
}

type StreamMessages struct {
	Stream   string
	Messages []StreamData
}

// Handler handler for cache
type Handler interface {
	Get() redigo.Conn
	GetContext(context.Context) (redigo.Conn, error)
}

const defaultPrefixKey = "phastos:"

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

func New(options ...Options) *Store {
	var cfg RedisCfg
	for _, opt := range options {
		opt(&cfg)
	}

	log := plog.Get()

	// Apply default pool settings if not provided
	if cfg.MaxIdle == 0 {
		cfg.MaxIdle = 10 // sensible default
	}
	if cfg.MaxActive == 0 {
		cfg.MaxActive = cfg.MaxIdle * 5 // default active connections
	}
	if cfg.MaxRetry == 0 {
		cfg.MaxRetry = 10
	}

	store := &Store{
		Pool: &redigo.Pool{
			MaxIdle:     cfg.MaxIdle,
			MaxActive:   cfg.MaxActive,
			IdleTimeout: time.Duration(cfg.Timeout) * time.Second,
			Dial: func() (redigo.Conn, error) {
				var dialOpts []redigo.DialOption

				if cfg.Password != "" {
					dialOpts = append(dialOpts, redigo.DialPassword(cfg.Password))
				}
				if cfg.Username != "" {
					dialOpts = append(dialOpts, redigo.DialUsername(cfg.Username))
				}

				dialOpts = append(dialOpts, redigo.DialConnectTimeout(time.Duration(cfg.Timeout)*time.Second))
				dialOpts = append(dialOpts, redigo.DialReadTimeout(time.Duration(cfg.Timeout)*time.Second))
				dialOpts = append(dialOpts, redigo.DialWriteTimeout(time.Duration(cfg.Timeout)*time.Second))

				dialOpts = append(dialOpts, redigo.DialDatabase(cfg.dbNo))

				c, err := redigo.Dial("tcp", cfg.Address, dialOpts...)
				if err != nil {
					log.Fatal().Msgf("Can't connect to redis: %s", err.Error())
				}
				return c, nil
			},
			TestOnBorrow: func(c redigo.Conn, t time.Time) error {
				_, err := redigo.String(c.Do("PING"))
				return err
			},
		},
	}

	pool := store.Pool.Get()
	if _, err := redigo.String(pool.Do("PING")); err != nil {
		log.Fatal().Msg(fmt.Sprintf("Cannot connect to redis: %s", err.Error()))
	}

	prefixKey := os.Getenv("REDIS_PREFIX_KEY")
	if prefixKey == "" {
		prefixKey = defaultPrefixKey
	}
	store.prefixKey = prefixKey
	store.maxRetry = cfg.MaxRetry
	store.sf = &singleflight.Group{}
	log.Info().Int("db", cfg.dbNo).Msg("Successful connect to redis")

	return store
}

func WithAddress(address string) Options {
	return func(cfg *RedisCfg) {
		cfg.Address = address
	}
}

func WithDatabaseNo(dbNo int) Options {
	return func(cfg *RedisCfg) {
		cfg.dbNo = dbNo
	}
}

func WithTimeout(timeout int) Options {
	return func(cfg *RedisCfg) {
		cfg.Timeout = timeout
	}
}

func WithMaxActive(maxActive int) Options {
	return func(cfg *RedisCfg) {
		cfg.MaxActive = maxActive
	}
}

func WithMaxIdle(maxIdle int) Options {
	return func(cfg *RedisCfg) {
		cfg.MaxIdle = maxIdle
	}
}

func WithMaxRetry(maxRetry int) Options {
	return func(cfg *RedisCfg) {
		cfg.MaxRetry = maxRetry
	}
}

func WithPassword(password string) Options {
	return func(cfg *RedisCfg) {
		cfg.Password = password
	}
}

func WithUsername(username string) Options {
	return func(cfg *RedisCfg) {
		cfg.Username = username
	}
}

func (r *Store) wrapWithRetries(ctx context.Context, actualFn actualRedisActionFn) (any, error) {
	log := plog.Ctx(ctx)
	// input validation

	// data validation
	for i := 0; i < r.maxRetry; i++ {
		result, err := actualFn(ctx)
		if err != nil {
			if errors.Is(err, redigo.ErrPoolExhausted) {
				log.Warn().Int("counter", i+1).Msg("[CACHE][REDIS] Connection pool exhausted, retrying...")
				time.Sleep(time.Second) // Tunggu sebelum mencoba lagi
				continue                // Coba lagi
			}
			return nil, err
		}

		// save data
		// telemetryu
		return result, nil
	}

	errFailedAfterRetry := errors.New(fmt.Sprintf("Failed to get connection pool after %d retries", r.maxRetry))
	return nil, errors.Wrap(errFailedAfterRetry, "phastos.cache.redis.WrapRetry")
}

// Get string value
func (r *Store) Get(ctx context.Context, key string, typeDestination any, fallbackFn ...FallbackFn) error {
	// validate is `typeDestination` is a pointer
	reflectVal := reflect.ValueOf(typeDestination)
	if reflectVal.Kind() != reflect.Ptr {
		return errors.Wrap(errors.New("type destination params should be a pointer"), "phastos.cache.redis.Get.CheckTypeDestinationParam")
	}
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		segmentName := "Redis-Get"
		if len(fallbackFn) > 0 {
			segmentName = fmt.Sprintf("%sWithFallback", segmentName)
		}
		_, span := monitoring.StartSpan(ctx, segmentName)
		defer span.End()
		span.SetAttributes(attribute.String("key", key))
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return "", errors.Wrap(err, "cache.redis.Get.GetContext")
		}

		defer conn.Close() //nolint:errcheck
		resp, err := redigo.String(conn.Do("GET", fmt.Sprintf("%s%s", r.prefixKey, key)))
		if errors.Is(err, redigo.ErrNil) {
			if len(fallbackFn) > 0 {
				fallbackAction := fallbackFn[0]
				return r.fallbackAction(ctx, key, "", fallbackAction, span, conn)
			}
			return "", err
		}
		return resp, err
	})

	if err != nil {
		return err
	}

	resultStr, validStr := wrapResult.(string)
	if !validStr {
		return errors.New(fmt.Sprintf("[CACHE][REDIS] - Result is not valid: %v", wrapResult))
	}

	if strVal, isStringType := typeDestination.(*string); isStringType {
		*strVal = resultStr
		return nil
	}

	if err = json.Unmarshal([]byte(resultStr), typeDestination); err != nil {
		unmarshalErr := errors.New(fmt.Sprintf("[CACHE][REDIS][GET] - Failed Unmarshal result %s with error: %s", resultStr, err.Error()))
		return errors.Wrap(unmarshalErr, "phastos.cache.redis.Get.UnmarshalValueToTypeDestination")
	}

	return nil
}

func (r *Store) fallbackAction(ctx context.Context, key, field string, fallbackFn FallbackFn, span monitoring.Span, conn redigo.Conn) (string, error) {
	log := plog.Ctx(ctx)
	sfKey := fmt.Sprintf("%s%s", r.prefixKey, key)
	if field != "" {
		sfKey = fmt.Sprintf("%s:%s", sfKey, field)
	}

	if r.sf == nil {
		r.sf = &singleflight.Group{}
	}

	result, err, _ := r.sf.Do(sfKey, func() (any, error) {
		fallbackResult, fallbackExpire, fallbackErr := fallbackFn(ctx)
		if fallbackErr != nil {
			return "", errors.Wrap(fallbackErr, "phastos.cache.redis.Get.FallbackFunction.Error")
		}

		fallbackValue, isStringType := fallbackResult.(string)
		var cacheValue string
		if !isStringType {
			byteFallbackResult, marshallErr := json.Marshal(fallbackResult)
			if marshallErr != nil {
				return "", errors.Wrap(marshallErr, "phastos.cache.redis.Get.FallbackFunction.FailedMarshalResult")
			}
			cacheValue = string(byteFallbackResult)
		} else {
			cacheValue = fallbackValue
		}
		var setParams []any
		key = fmt.Sprintf("%s%s", r.prefixKey, key)
		setParams = append(setParams, key)
		if field != "" {
			setParams = append(setParams, field)
		}
		setParams = append(setParams, cacheValue)
		if fallbackExpire == 0 {
			fallbackExpire = int64(10 * time.Minute.Seconds())
		}
		if span != nil {
			span.SetAttributes(attribute.Int64("expire", fallbackExpire))
		}

		redisCommand := "SET"
		isHSETTTLAlreadyExist := false
		if field != "" {
			redisCommand = "HSET"
			val, err := redigo.Int64(conn.Do("TTL", key))
			if err != nil {
				return "", err
			}

			if val > 0 {
				isHSETTTLAlreadyExist = true
			}
		}

		if field == "" {
			setParams = append(setParams, "EX")
			setParams = append(setParams, fallbackExpire)
		}

		if _, err := conn.Do(redisCommand, setParams...); err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("phastos.cache.redis.fallbackAction.%s", redisCommand))
		}

		if field != "" && fallbackExpire > 0 && !isHSETTTLAlreadyExist {
			if _, err := conn.Do("EXPIRE", key, fallbackExpire); err != nil {
				log.Err(err).Str("key", key).Str("field", field).Msg("Failed to set Expire")
			}
		}
		return cacheValue, nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// Del key value
func (r *Store) Del(ctx context.Context, key string) (int64, error) {
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		_, span := monitoring.StartSpan(ctx, "Redis-Delete")
		defer span.End()
		span.SetAttributes(attribute.String("key", key))
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return 0, errors.Wrap(err, "cache.redis.Del.GetContext")
		}
		defer conn.Close() //nolint:errcheck
		resp, err := redigo.Int64(conn.Do("DEL", fmt.Sprintf("%s%s", r.prefixKey, key)))
		if err != nil {
			return int64(0), errors.Wrap(err, "infrastructure.cache.redis.Del")
		}
		return resp, err
	})
	if err != nil {
		return 0, err
	}

	return wrapResult.(int64), nil //nolint:errcheck
}

func (r *Store) PublishStream(ctx context.Context, streamName string, data ...map[string]any) (string, error) {
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return "", errors.Wrap(err, "cache.redis.PublishStream.GetContext")
		}
		defer conn.Close() //nolint:errcheck

		if len(data) == 0 {
			return "", errors.New("please provide the data at least 1 data")
		}

		valueList := make([]any, 0, len(data)*2)
		for key, value := range data {
			valueList = append(valueList, key, value)
		}

		messageID, err := redigo.String(conn.Do("XADD", streamName, "MAXLEN", "~", "100", valueList))
		if err != nil {
			return "", errors.Wrap(err, "infrastructure.cache.redis.PublishStream.XADD")
		}

		return messageID, nil
	})
	if err != nil {
		return "", err
	}
	return wrapResult.(string), nil //nolint:errcheck
}

func (r Store) SubscribeStream(ctx context.Context, streamName string, actionFn func(ctx context.Context, data *StreamData) error) {
	log := plog.Ctx(ctx)
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		log.Fatal().Err(err).Str("streamName", streamName).Str("context", "redis.Subscribe.Stream").Msg("Failed to get redis pool")
		return
	}
	defer conn.Close() //nolint:errcheck

	lastID := "0" // Mulai membaca dari awal. Gunakan "$" untuk hanya pesan baru.

	failedCounter := 0
	log.Info().Msg("Subscribed stream started")
	data := new(StreamData)
	for {
		select {
		case <-ctx.Done():
			// info the stream listening should be stopped gracefully
			log.Info().Msg("Subscribed stream stopped, because context is done (shutdown gracefully)")
			return
		default:
			if failedCounter > 10 {
				log.Info().Msg("Subscribed stream stopped, because of reached limit of failed process")
				return
			}
			// BLOCK 0 artinya tunggu selamanya sampai ada pesan masuk
			// STREAMS mystream 0 -> baca dari stream 'mystream' mulai dari ID '0'
			reply, err := redigo.Values(conn.Do("XREAD", "BLOCK", 0, "STREAMS", streamName, lastID))
			if err != nil {
				if errors.Is(err, redigo.ErrNil) {
					continue // Timeout BLOCK biasa, lanjut loop
				}
				log.Err(err).Str("streamName", streamName).Str("context", "redis.Subscribe.Stream").Msg("Failed to read stream, will try again")
				time.Sleep(time.Second) // Backoff singkat jika error koneksi
				failedCounter++
				continue
			}

			// Struktur reply XREAD cukup kompleks: [ [streamName, [ [id, [fields]] ] ] ]
			streams := reply[0].([]any)    //nolint:errcheck
			messages := streams[1].([]any) //nolint:errcheck

			for _, msg := range messages {
				item := msg.([]any)            //nolint:errcheck
				id := string(item[0].([]byte)) //nolint:errcheck
				fields, errCast := redigo.StringMap(item[1], nil)
				if errCast != nil {
					log.Err(errCast).
						Str("streamName", streamName).
						Str("context", "redis.Subscribe.Stream.readMessages").
						Msg("Failed cast fields to map string")
					failedCounter++
					continue
				}

				data.ID = id
				data.Values = fields

				if errAction := actionFn(ctx, data); errAction != nil {
					log.Err(errAction).
						Any("data", data).
						Str("context", "cache.redis.Subscribe.Stream.actionFn").
						Msg("Failed in action functions")
					failedCounter++
					continue
				}

				failedCounter = 0
				// Update lastID agar pembacaan berikutnya mulai setelah pesan ini
				lastID = id
			}
		}
	}
}

// HSet set has map
func (r *Store) HSet(ctx context.Context, key, field string, value any, expire ...int) error {
	if _, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		log := plog.Ctx(ctx)
		key = fmt.Sprintf("%s%s", r.prefixKey, key)
		_, span := monitoring.StartSpan(ctx, "Redis-HSET")
		defer span.End()
		span.SetAttributes(attribute.String("key", key))
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HSET.GetPoolContext")
		}
		defer conn.Close() //nolint:errcheck

		params := []any{key, field}
		if val, isStringType := value.(string); isStringType {
			params = append(params, val)
		} else {
			byteValue, _ := json.Marshal(value)
			params = append(params, string(byteValue))
		}

		_, err = redigo.Int64(conn.Do("HSET", params...))
		if err != nil {
			return nil, err
		}

		expireTime := 0
		if len(expire) > 0 {
			expireTime = int(10 * time.Minute.Seconds())
			if expire[0] > 0 {
				expireTime = expire[0]
			}
		}

		if expireTime > 0 {
			_, err = redigo.Int64(conn.Do("EXPIRE", key, expireTime))
			if err != nil {
				log.Err(err).Str("key", key).Str("field", field).Str("command", "HSET").Msg("Failed to set Expire")
			}
		}
		return nil, nil
	}); err != nil {
		return err
	}

	return nil
}

// HGet set has map
func (r *Store) HGet(ctx context.Context, key, field string, typeDestination any, fallbackFn ...FallbackFn) error {
	// validate is `typeDestination` is a pointer
	reflectVal := reflect.ValueOf(typeDestination)
	if reflectVal.Kind() != reflect.Ptr {
		return errors.Wrap(errors.New("type destination params should be a pointer"), "phastos.cache.redis.Get.CheckTypeDestinationParam")
	}
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		segmentName := "Redis-HGET"
		if len(fallbackFn) > 0 {
			segmentName = fmt.Sprintf("%sWithFallback", segmentName)
		}
		_, span := monitoring.StartSpan(ctx, segmentName)
		defer span.End()
		span.SetAttributes(attribute.String("key", key))
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HGET.GetPoolContext")
		}
		defer conn.Close() //nolint:errcheck
		resp, err := redigo.String(conn.Do("HGET", fmt.Sprintf("%s%s", r.prefixKey, key), field))
		if errors.Is(err, redigo.ErrNil) && len(fallbackFn) > 0 {
			fallbackAction := fallbackFn[0]
			return r.fallbackAction(ctx, key, field, fallbackAction, span, conn)
		}
		return resp, err
	})

	if err != nil {
		return err
	}

	resultStr, validStr := wrapResult.(string)
	if !validStr {
		return errors.New(fmt.Sprintf("[CACHE][REDIS][HGET] - Result is not valid: %v", wrapResult))
	}

	if strVal, isStringType := typeDestination.(*string); isStringType {
		*strVal = resultStr
		return nil
	}
	if err = json.Unmarshal([]byte(resultStr), typeDestination); err != nil {
		unmarshalErr := errors.New(fmt.Sprintf("[CACHE][REDIS][HGET] - Failed Unmarshal result %s with error: %s", resultStr, err.Error()))
		return errors.Wrap(unmarshalErr, "phastos.cache.redis.HGET.UnmarshalValueToTypeDestination")
	}
	return nil
}

// HDel set has map
func (r *Store) HDel(ctx context.Context, key, field string) error {
	if _, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		_, span := monitoring.StartSpan(ctx, "Redis-HDEL")
		defer span.End()
		span.SetAttributes(attribute.String("key", key))
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HDEL.GetPoolContext")
		}
		defer conn.Close() //nolint:errcheck
		resp, err := redigo.Int64(conn.Do("HDEL", fmt.Sprintf("%s%s", r.prefixKey, key), field))
		if err != nil {
			return int64(0), errors.Wrap(err, "phastos.cache.redis.HDEL")
		}
		return resp, err
	}); err != nil {
		return err
	}

	return nil
}

// Set ill be used to set the value
func (r *Store) Set(ctx context.Context, key string, value any, expire ...int) error {
	_, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		_, span := monitoring.StartSpan(ctx, "Redis-Set")
		defer span.End()
		span.SetAttributes(attribute.String("key", key))
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return "", errors.Wrap(err, "cache.redis.Set.GetContext")
		}
		defer conn.Close() //nolint:errcheck
		var setParams []any
		setParams = append(setParams, fmt.Sprintf("%s%s", r.prefixKey, key))

		if val, isString := value.(string); isString {
			setParams = append(setParams, val)
		} else {
			byteValue, _ := json.Marshal(value)
			setParams = append(setParams, string(byteValue))
		}
		expireTime := int(10 * time.Minute.Seconds())
		if len(expire) > 0 {
			expireTime = expire[0]
		}
		span.SetAttributes(attribute.Int("expire", expireTime))

		setParams = append(setParams, "EX")
		setParams = append(setParams, expireTime)
		return redigo.String(conn.Do("SET", setParams...))
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *Store) WrapToHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := context.WithValue(request.Context(), entity.CacheContext{}, r)
		*request = *request.WithContext(ctx)

		next.ServeHTTP(writer, request)
	})
}

func (r *Store) WrapToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, entity.CacheContext{}, r)
}

func (r *Store) HGetAll(ctx context.Context, key string, dest interface{}) error {
	reflectVal := reflect.ValueOf(dest)
	if reflectVal.Kind() != reflect.Ptr {
		return errors.Wrap(errors.New("type destination params should be a pointer"), "phastos.cache.redis.HGetAll.CheckTypeDestinationParam")
	}
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result interface{}, err error) {
		_, span := monitoring.StartSpan(ctx, "Redis-HGETALL")
		defer span.End()
		span.SetAttributes(attribute.String("key", key))
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HGetAll.GetPoolContext")
		}
		defer conn.Close()
		resp, err := redigo.StringMap(conn.Do("HGETALL", fmt.Sprintf("%s%s", r.prefixKey, key)))
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
	if err != nil {
		return err
	}
	resultMap, ok := wrapResult.(map[string]string)
	if !ok {
		return errors.New("phastos.cache.redis.HGetAll: invalid result type")
	}
	if m, ok := dest.(*map[string]string); ok {
		*m = resultMap
		return nil
	}
	jsonBytes, _ := json.Marshal(resultMap)
	if err = json.Unmarshal(jsonBytes, dest); err != nil {
		return errors.Wrap(err, "phastos.cache.redis.HGetAll.UnmarshalValueToTypeDestination")
	}
	return nil
}

func (r *Store) HSetBulk(ctx context.Context, key string, fields map[string]interface{}, expire ...int) error {
	_, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result interface{}, err error) {
		_, span := monitoring.StartSpan(ctx, "Redis-HSET-BULK")
		defer span.End()
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HSetBulk.GetPoolContext")
		}
		defer conn.Close()
		fullKey := fmt.Sprintf("%s%s", r.prefixKey, key)
		for field, value := range fields {
			var val string
			if s, ok := value.(string); ok {
				val = s
			} else {
				b, _ := json.Marshal(value)
				val = string(b)
			}
			if err := conn.Send("HSET", fullKey, field, val); err != nil {
				return nil, err
			}
		}
		if len(expire) > 0 && expire[0] > 0 {
			if err := conn.Send("EXPIRE", fullKey, expire[0]); err != nil {
				return nil, err
			}
		}
		if err := conn.Flush(); err != nil {
			return nil, err
		}
		for i := 0; i < len(fields); i++ {
			if _, err := conn.Receive(); err != nil {
				return nil, err
			}
		}
		if len(expire) > 0 && expire[0] > 0 {
			if _, err := conn.Receive(); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (r *Store) XGroupCreateMkStream(ctx context.Context, streamKey, group, startID string) error {
	_, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result interface{}, err error) {
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.XGroupCreateMkStream.GetPoolContext")
		}
		defer conn.Close()
		fullKey := fmt.Sprintf("%s%s", r.prefixKey, streamKey)
		_, err = conn.Do("XGROUP", "CREATE", fullKey, group, startID, "MKSTREAM")
		return nil, err
	})
	return err
}

func (r *Store) XReadGroup(ctx context.Context, group, consumer string, streams []string, ids []string, block time.Duration, count int64) ([]StreamMessages, error) {
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result interface{}, err error) {
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.XReadGroup.GetPoolContext")
		}
		defer conn.Close()
		prefixedStreams := make([]string, len(streams))
		for i, s := range streams {
			prefixedStreams[i] = fmt.Sprintf("%s%s", r.prefixKey, s)
		}
		args := []interface{}{"GROUP", group, consumer, "COUNT", count}
		if block > 0 {
			args = append(args, "BLOCK", int(block.Milliseconds()))
		}
		args = append(args, "STREAMS")
		for _, s := range prefixedStreams {
			args = append(args, s)
		}
		for _, id := range ids {
			args = append(args, id)
		}
		reply, err := redigo.Values(conn.Do("XREADGROUP", args...))
		if err != nil {
			if errors.Is(err, redigo.ErrNil) {
				return []StreamMessages{}, nil
			}
			return nil, err
		}
		var output []StreamMessages
		for _, streamEntry := range reply {
			entry := streamEntry.([]interface{})
			streamName := string(entry[0].([]byte))
			messages := entry[1].([]interface{})
			var msgs []StreamData
			for _, msg := range messages {
				item := msg.([]interface{})
				id := string(item[0].([]byte))
				fields, _ := redigo.StringMap(item[1], nil)
				msgs = append(msgs, StreamData{ID: id, Values: fields})
			}
			output = append(output, StreamMessages{Stream: streamName, Messages: msgs})
		}
		return output, nil
	})
	if err != nil {
		return nil, err
	}
	return wrapResult.([]StreamMessages), nil
}

func (r *Store) XAck(ctx context.Context, streamKey, group, id string) (int64, error) {
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result interface{}, err error) {
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.XAck.GetPoolContext")
		}
		defer conn.Close()
		fullKey := fmt.Sprintf("%s%s", r.prefixKey, streamKey)
		resp, err := redigo.Int64(conn.Do("XACK", fullKey, group, id))
		return resp, err
	})
	if err != nil {
		return 0, err
	}
	return wrapResult.(int64), nil
}
