package ratelimit

import (
	"context"
	"errors"
	"time"
)

type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

type RedisLimiter struct {
	client          RedisClient
	capacity        int
	refillPerSecond float64
	now             Clock
}

func NewRedisLimiter(client RedisClient, capacity int, refillPerSecond float64) *RedisLimiter {
	if capacity < 1 {
		capacity = 1
	}
	if refillPerSecond <= 0 {
		refillPerSecond = float64(capacity)
	}
	return &RedisLimiter{
		client:          client,
		capacity:        capacity,
		refillPerSecond: refillPerSecond,
		now:             time.Now,
	}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	if l == nil || l.client == nil {
		return false, 0, errors.New("redis limiter requires a client")
	}
	if key == "" {
		key = "unknown"
	}

	result, err := l.client.Eval(ctx, tokenBucketScript, []string{"rate:" + key}, l.capacity, l.refillPerSecond, l.now().UnixMilli())
	if err != nil {
		return false, 0, err
	}

	values, ok := result.([]any)
	if !ok || len(values) < 2 {
		return false, 0, errors.New("unexpected redis limiter response")
	}
	allowed, ok := toInt64(values[0])
	if !ok {
		return false, 0, errors.New("unexpected redis allowed value")
	}
	retryAfterMS, ok := toInt64(values[1])
	if !ok {
		return false, 0, errors.New("unexpected redis retry value")
	}

	return allowed == 1, time.Duration(retryAfterMS) * time.Millisecond, nil
}

func toInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}

const tokenBucketScript = `
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local state = redis.call("HMGET", KEYS[1], "tokens", "updated_at")
local tokens = tonumber(state[1]) or capacity
local updated_at = tonumber(state[2]) or now
local elapsed = math.max(0, now - updated_at) / 1000
tokens = math.min(capacity, tokens + elapsed * refill)
if tokens >= 1 then
  tokens = tokens - 1
  redis.call("HMSET", KEYS[1], "tokens", tokens, "updated_at", now)
  redis.call("PEXPIRE", KEYS[1], math.ceil((capacity / refill) * 2000))
  return {1, 0}
end
local retry = math.ceil(((1 - tokens) / refill) * 1000)
redis.call("HMSET", KEYS[1], "tokens", tokens, "updated_at", now)
redis.call("PEXPIRE", KEYS[1], math.ceil((capacity / refill) * 2000))
return {0, retry}
`
