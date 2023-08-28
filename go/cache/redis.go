package cache

import (
	"context"
	"log"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
)

// Store object
type Store struct {
	Pool Handler
}

type Options func(*RedisCfg)

type RedisCfg struct {
	Address   string `yaml:"address"`
	Timeout   int    `yaml:"timeout"`
	MaxIdle   int    `yaml:"max_iddle"`
	MaxActive int    `yaml:"max_active"`
}

// Handler handler for cache
type Handler interface {
	Get() redigo.Conn
	GetContext(context.Context) (redigo.Conn, error)
}

type Caches interface {
	Get(key string) (string, error)
	Del(key string) (int64, error)
	HSet(key, field, value string) (string, error)
	Set(key, value string, expire int) (string, error)
	AddInSet(key, value string) (int, error)
	GetSetMembers(key string) ([]string, error)
	GetSetLength(key string) (int, error)
	GetNElementOfSet(key string, n int) ([]string, error)
	PushNElementToSet(values []interface{}) (int, error)
}

func New(options ...Options) *Store {
	var cfg RedisCfg
	for _, opt := range options {
		opt(&cfg)
	}

	return &Store{
		Pool: &redigo.Pool{
			MaxIdle:     cfg.MaxIdle,
			MaxActive:   cfg.MaxActive,
			IdleTimeout: time.Duration(cfg.Timeout) * time.Second,
			Dial: func() (redigo.Conn, error) {
				c, err := redigo.Dial("tcp", cfg.Address)
				if err != nil {
					log.Fatalln("Can't connect to redis: ", err.Error())
				}
				return c, nil
			},
			TestOnBorrow: func(c redigo.Conn, t time.Time) error {
				_, err := c.Do("PING")
				return err
			},
		},
	}
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

// Get string value
func (r *Store) Get(ctx context.Context, key string) (string, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return "", errors.Wrap(err, "cache.redis.Get.GetContext")
	}
	defer conn.Close()
	resp, err := redigo.String(conn.Do("GET", key))
	if err == redigo.ErrNil {
		return "", errors.Wrap(err, "infrastructure.cache.redis.Get")
	}
	return resp, err
}

// Del key value
func (r *Store) Del(ctx context.Context, key string) (int64, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "cache.redis.Del.GetContext")
	}
	defer conn.Close()
	resp, err := redigo.Int64(conn.Do("DEL", key))
	if err == redigo.ErrNil {
		return 0, errors.Wrap(err, "infrastructure.cache.redis.Del")
	}
	return resp, err
}

// HSet set has map
func (r *Store) HSet(ctx context.Context, key, field, value string) (string, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return "", errors.Wrap(err, "cache.redis.HSet.GetContext")
	}
	defer conn.Close()
	return redigo.String(conn.Do("HSET", key, field, value))
}

// Set ill be used to set the value
func (r *Store) Set(ctx context.Context, key, value string, expire int) (string, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return "", errors.Wrap(err, "cache.redis.Set.GetContext")
	}
	defer conn.Close()
	return redigo.String(conn.Do("SET", key, value, "EX", expire))
}

// AddInSet will be used to add value in set
func (r *Store) AddInSet(ctx context.Context, key, value string) (int, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "cache.redis.AddInSet.GetContext")
	}
	defer conn.Close()
	return redigo.Int(conn.Do("SADD", key, value))
}

// GetSetMembers will be used to get the set memebers
func (r *Store) GetSetMembers(ctx context.Context, key string) ([]string, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cache.redis.GetSetMembers.GetContext")
	}
	defer conn.Close()
	return redigo.Strings(conn.Do("SMEMBERS", key))
}

// GetSetLength will be used to get the set length
func (r *Store) GetSetLength(ctx context.Context, key string) (int, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "cache.redis.GetSetLength.GetContext")
	}
	defer conn.Close()
	return redigo.Int(conn.Do("SCARD", key))
}

// GetNElementOfSet to get the first N elements of set
func (r *Store) GetNElementOfSet(ctx context.Context, key string, n int) ([]string, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cache.redis.GetNElementOfSet.GetContext")
	}
	defer conn.Close()
	return redigo.Strings(conn.Do("SPOP", key, n))
}

// PushNElementToSet will be used to push n elements to set
func (r *Store) PushNElementToSet(ctx context.Context, values []interface{}) (int, error) {
	conn, err := r.Pool.GetContext(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "cache.redis.PushNElementToSet.GetContext")
	}
	defer conn.Close()
	return redigo.Int(conn.Do("SADD", values...))
}
