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
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/entity"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

// Store object
type Store struct {
	Pool      Handler
	prefixKey string
	maxRetry  int
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
}

func New(options ...Options) *Store {
	var cfg RedisCfg
	for _, opt := range options {
		opt(&cfg)
	}

	log := plog.Get()

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
	maxRetry := cfg.MaxRetry
	if maxRetry == 0 {
		maxRetry = 10
	}
	store.maxRetry = maxRetry
	log.Info().Msg("Successful connect to redis")

	return store
}

func WithAddress(address string) Options {
	return func(cfg *RedisCfg) {
		cfg.Address = address
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
		txn := monitoring.BeginTrxFromContext(ctx)
		segmentName := "Redis-Get"
		if fallbackFn != nil && len(fallbackFn) > 0 {
			segmentName = fmt.Sprintf("%sWithFallback", segmentName)
		}
		segment := txn.StartSegment(segmentName)
		if txn != nil {
			segment.AddAttribute("key", key)
			defer segment.End()
		}
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return "", errors.Wrap(err, "cache.redis.Get.GetContext")
		}

		defer conn.Close()
		resp, err := redigo.String(conn.Do("GET", fmt.Sprintf("%s%s", r.prefixKey, key)))
		if errors.Is(err, redigo.ErrNil) {
			if fallbackFn != nil && len(fallbackFn) > 0 {
				fallbackAction := fallbackFn[0]
				return r.fallbackAction(ctx, key, "", fallbackAction, segment, conn)
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

func (r *Store) fallbackAction(ctx context.Context, key, field string, fallbackFn FallbackFn, segment *newrelic.Segment, conn redigo.Conn) (string, error) {
	log := plog.Ctx(ctx)
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
	var setParams []interface{}
	key = fmt.Sprintf("%s%s", r.prefixKey, key)
	setParams = append(setParams, key)
	if field != "" {
		setParams = append(setParams, field)
	}
	setParams = append(setParams, cacheValue)
	if fallbackExpire == 0 {
		// set default expired time to 10 minutes
		fallbackExpire = int64(10 * time.Minute.Seconds())
	}
	if segment != nil {
		segment.AddAttribute("expire", fallbackExpire)
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

	// override the fallback expire (HGET only) when not exists !
	// to prevent re-assign the EXPIRE of session key
	if field != "" && fallbackExpire > 0 && !isHSETTTLAlreadyExist {
		if _, err := conn.Do("EXPIRE", key, fallbackExpire); err != nil {
			log.Err(err).Str("key", key).Str("field", field).Msg("Failed to set Expire")
		}
	}
	return cacheValue, nil
}

// Del key value
func (r *Store) Del(ctx context.Context, key string) (int64, error) {
	wrapResult, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		txn := monitoring.BeginTrxFromContext(ctx)
		if txn != nil {
			segment := txn.StartSegment("Redis-Delete")
			segment.AddAttribute("key", key)
			defer segment.End()
		}
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return 0, errors.Wrap(err, "cache.redis.Del.GetContext")
		}
		defer conn.Close()
		resp, err := redigo.Int64(conn.Do("DEL", fmt.Sprintf("%s%s", r.prefixKey, key)))
		if err != nil {
			return int64(0), errors.Wrap(err, "infrastructure.cache.redis.Del")
		}
		return resp, err
	})
	if err != nil {
		return 0, err
	}

	return wrapResult.(int64), nil
}

// HSet set has map
func (r *Store) HSet(ctx context.Context, key, field string, value any, expire ...int) error {
	if _, err := r.wrapWithRetries(ctx, func(ctx context.Context) (result any, err error) {
		log := plog.Ctx(ctx)
		txn := monitoring.BeginTrxFromContext(ctx)
		segmentName := "Redis-HSET"
		segment := txn.StartSegment(segmentName)
		key = fmt.Sprintf("%s%s", r.prefixKey, key)
		if txn != nil {
			segment.AddAttribute("key", key)
			defer segment.End()
		}
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HSET.GetPoolContext")
		}
		defer conn.Close()

		params := []interface{}{key, field}
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
		if expire != nil && len(expire) > 0 {
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
		txn := monitoring.BeginTrxFromContext(ctx)
		segmentName := "Redis-HGET"
		if fallbackFn != nil && len(fallbackFn) > 0 {
			segmentName = fmt.Sprintf("%sWithFallback", segmentName)
		}
		segment := txn.StartSegment(segmentName)
		if txn != nil {
			segment.AddAttribute("key", key)
			defer segment.End()
		}
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HGET.GetPoolContext")
		}
		defer conn.Close()
		resp, err := redigo.String(conn.Do("HGET", fmt.Sprintf("%s%s", r.prefixKey, key), field))
		if errors.Is(err, redigo.ErrNil) && (fallbackFn != nil && len(fallbackFn) > 0) {
			fallbackAction := fallbackFn[0]
			return r.fallbackAction(ctx, key, field, fallbackAction, segment, conn)
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
		txn := monitoring.BeginTrxFromContext(ctx)
		segmentName := "Redis-HDEL"
		segment := txn.StartSegment(segmentName)
		if txn != nil {
			segment.AddAttribute("key", key)
			defer segment.End()
		}
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "cache.redis.HDEL.GetPoolContext")
		}
		defer conn.Close()
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
		txn := monitoring.BeginTrxFromContext(ctx)
		var segment *newrelic.Segment
		if txn != nil {
			segment = txn.StartSegment("Redis-Set")
			segment.AddAttribute("key", key)
			defer segment.End()
		}
		conn, err := r.Pool.GetContext(ctx)
		if err != nil {
			return "", errors.Wrap(err, "cache.redis.Set.GetContext")
		}
		defer conn.Close()
		var setParams []interface{}
		setParams = append(setParams, fmt.Sprintf("%s%s", r.prefixKey, key))

		if val, isString := value.(string); isString {
			setParams = append(setParams, val)
		} else {
			byteValue, _ := json.Marshal(value)
			setParams = append(setParams, string(byteValue))
		}
		expireTime := int(10 * time.Minute.Seconds())
		if expire != nil && len(expire) > 0 {
			expireTime = expire[0]
		}
		if segment != nil {
			segment.AddAttribute("expire", expireTime)
		}

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
