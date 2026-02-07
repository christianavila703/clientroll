package middleware

import (
	"fmt"
	"net/http"
	"time"

	"clientroll/internal/database"
)

// RateLimiter provides Redis-based per-IP rate limiting.
type RateLimiter struct {
	redis      *database.Redis
	maxAttempts int
	window      time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(redis *database.Redis, maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redis:      redis,
		maxAttempts: maxAttempts,
		window:      window,
	}
}

// Limit returns middleware that rate-limits requests by IP.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		key := fmt.Sprintf("ratelimit:%s", ip)

		count, err := rl.redis.Client.Incr(r.Context(), rl.redis.Key(key)).Result()
		if err != nil {
			// If Redis fails, allow the request (fail open for availability)
			next.ServeHTTP(w, r)
			return
		}

		if count == 1 {
			// First request in window -- set expiry
			rl.redis.Client.Expire(r.Context(), rl.redis.Key(key), rl.window)
		}

		if count > int64(rl.maxAttempts) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Too many requests. Please try again later.",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// NewAuthRateLimiter creates a general rate limiter for public auth endpoints.
// 120 requests per 15 minutes per IP — generous for signup/reset flows.
func NewAuthRateLimiter(redis *database.Redis) *RateLimiter {
	return NewRateLimiter(redis, 120, 15*time.Minute)
}

// NewLoginRateLimiter creates a rate limiter for login endpoints.
// 200 attempts per 15 minutes per IP.
func NewLoginRateLimiter(redis *database.Redis) *RateLimiter {
	return &RateLimiter{
		redis:       redis,
		maxAttempts: 200,
		window:      15 * time.Minute,
	}
}
