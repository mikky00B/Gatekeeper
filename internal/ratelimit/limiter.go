package ratelimit

import (
	"math"
	"sync"
	"time"
)

type Clock func() time.Time

type Limiter struct {
	mu       sync.Mutex
	capacity float64
	refill   float64
	buckets  map[string]*bucket
	now      Clock
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

func NewLimiter(capacity int, refillPerSecond float64) *Limiter {
	return NewLimiterWithClock(capacity, refillPerSecond, time.Now)
}

func NewLimiterWithClock(capacity int, refillPerSecond float64, now Clock) *Limiter {
	if capacity < 1 {
		capacity = 1
	}
	if refillPerSecond <= 0 {
		refillPerSecond = float64(capacity)
	}
	if now == nil {
		now = time.Now
	}

	return &Limiter{
		capacity: float64(capacity),
		refill:   refillPerSecond,
		buckets:  make(map[string]*bucket),
		now:      now,
	}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b := l.buckets[key]
	if b == nil {
		b = &bucket{
			tokens:     l.capacity,
			lastRefill: now,
		}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.capacity, b.tokens+elapsed*l.refill)
		b.lastRefill = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	missing := 1 - b.tokens
	retryAfter := time.Duration(math.Ceil(missing/l.refill)) * time.Second
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return false, retryAfter
}
