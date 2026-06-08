package ratelimit

import (
	"log"
	"net/http"
	"strconv"
)

func RedisMiddleware(limiter *RedisLimiter, keyFunc KeyFunc) func(http.Handler) http.Handler {
	if keyFunc == nil {
		keyFunc = IPKey
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter, err := limiter.Allow(r.Context(), keyFunc(r))
			if err != nil {
				log.Printf("redis rate limiter failed: %v", err)
				http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
