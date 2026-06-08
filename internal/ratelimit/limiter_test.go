package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllowsUntilCapacityIsExhausted(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	limiter := NewLimiterWithClock(2, 1, func() time.Time { return now })

	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("first request was rejected")
	}
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("second request was rejected")
	}
	if ok, retryAfter := limiter.Allow("client"); ok {
		t.Fatal("third request was allowed")
	} else if retryAfter != time.Second {
		t.Fatalf("retry after = %s, want 1s", retryAfter)
	}
}

func TestLimiterRefillsOverTime(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	limiter := NewLimiterWithClock(1, 1, func() time.Time { return now })

	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("first request was rejected")
	}
	if ok, _ := limiter.Allow("client"); ok {
		t.Fatal("second request was allowed before refill")
	}

	now = now.Add(time.Second)
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("request was rejected after refill")
	}
}

func TestLimiterTracksKeysIndependently(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	limiter := NewLimiterWithClock(1, 1, func() time.Time { return now })

	if ok, _ := limiter.Allow("client-a"); !ok {
		t.Fatal("client-a first request was rejected")
	}
	if ok, _ := limiter.Allow("client-b"); !ok {
		t.Fatal("client-b first request was rejected")
	}
}
