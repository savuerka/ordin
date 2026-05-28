package framework

import (
	"context"
	"crypto/tls"
	"errors"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// Cache is the small ORDIN abstraction over Redis-like key/value storage.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	GetBytes(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	RememberBytes(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error)
	Close() error
}

// RedisConfig configures a Redis cache/client connection.
type RedisConfig struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	Prefix       string
	TLS          bool
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// RedisConfigFromEnv reads REDIS_* variables by default.
func RedisConfigFromEnv(prefix string) RedisConfig {
	prefix = strings.Trim(strings.ToUpper(prefix), "_")
	if prefix == "" {
		prefix = "REDIS"
	}

	db, _ := strconv.Atoi(getenv(prefix+"_DB", "0"))
	return RedisConfig{
		Addr:         getenv(prefix+"_ADDR", "localhost:6379"),
		Username:     getenv(prefix+"_USERNAME", ""),
		Password:     getenv(prefix+"_PASSWORD", ""),
		DB:           db,
		Prefix:       getenv(prefix+"_PREFIX", ""),
		TLS:          getEnvironmentBool(prefix+"_TLS", false),
		DialTimeout:  getenvDuration(prefix+"_DIAL_TIMEOUT", 5*time.Second),
		ReadTimeout:  getenvDuration(prefix+"_READ_TIMEOUT", 3*time.Second),
		WriteTimeout: getenvDuration(prefix+"_WRITE_TIMEOUT", 3*time.Second),
	}
}

// RedisCache is the Redis implementation of Cache.
type RedisCache struct {
	client *redis.Client
	prefix string
}

// NewRedisCache creates a Redis-backed cache.
func NewRedisCache(config RedisConfig) (*RedisCache, error) {
	if strings.TrimSpace(config.Addr) == "" {
		return nil, errors.New("redis addr is empty")
	}

	options := &redis.Options{
		Addr:         config.Addr,
		Username:     config.Username,
		Password:     config.Password,
		DB:           config.DB,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}
	if config.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return &RedisCache{client: redis.NewClient(options), prefix: strings.TrimSpace(config.Prefix)}, nil
}

// MustRedisCache creates a Redis cache or panics.
func MustRedisCache(config RedisConfig) *RedisCache {
	cache, err := NewRedisCache(config)
	if err != nil {
		panic(err)
	}
	return cache
}

// Client exposes the underlying go-redis client for advanced workflows.
func (r *RedisCache) Client() *redis.Client {
	if r == nil {
		return nil
	}
	return r.client
}

// Ping checks Redis availability.
func (r *RedisCache) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return errors.New("redis cache is not configured")
	}
	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) key(key string) string {
	key = strings.TrimSpace(key)
	if r == nil || r.prefix == "" || strings.HasPrefix(key, r.prefix) {
		return key
	}
	return r.prefix + key
}

// Get reads a string value.
func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	if r == nil || r.client == nil {
		return "", errors.New("redis cache is not configured")
	}
	value, err := r.client.Get(ctx, r.key(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	return value, err
}

// GetBytes reads a binary value.
func (r *RedisCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	value, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

// Set writes a value with optional TTL. Use ttl=0 for no expiration.
func (r *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if r == nil || r.client == nil {
		return errors.New("redis cache is not configured")
	}
	return r.client.Set(ctx, r.key(key), value, ttl).Err()
}

// SetBytes writes a binary value with optional TTL. Use ttl=0 for no expiration.
func (r *RedisCache) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.Set(ctx, key, value, ttl)
}

// Delete removes one or more keys.
func (r *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if r == nil || r.client == nil {
		return errors.New("redis cache is not configured")
	}
	if len(keys) == 0 {
		return nil
	}
	prefixed := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) != "" {
			prefixed = append(prefixed, r.key(key))
		}
	}
	if len(prefixed) == 0 {
		return nil
	}
	return r.client.Del(ctx, prefixed...).Err()
}

// Exists checks if key exists.
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if r == nil || r.client == nil {
		return false, errors.New("redis cache is not configured")
	}
	count, err := r.client.Exists(ctx, r.key(key)).Result()
	return count > 0, err
}

// RememberBytes returns cached bytes or stores the result of fn with the given TTL.
func (r *RedisCache) RememberBytes(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	if fn == nil {
		return nil, errors.New("remember callback is nil")
	}
	value, err := r.GetBytes(ctx, key)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, ErrCacheMiss) {
		return nil, err
	}
	value, err = fn(ctx)
	if err != nil {
		return nil, err
	}
	return value, r.SetBytes(ctx, key, value, ttl)
}

// Close closes the Redis client.
func (r *RedisCache) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

// ErrCacheMiss is returned when a cache key is not found.
var ErrCacheMiss = errors.New("cache miss")
