package utils

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// RateLimiter implements a sliding window rate limiter
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	config   RateLimitConfig
}

// RateLimitConfig defines rate limit settings
type RateLimitConfig struct {
	PublicLimit      int           // requests per window for public endpoints
	AuthLimit        int           // requests per window for authenticated
	ExternalAPILimit int           // requests per window for external API
	WindowDuration   time.Duration // sliding window duration
}

var (
	limiter     *RateLimiter
	limiterOnce sync.Once
)

// GetRateLimiter returns the singleton rate limiter instance
func GetRateLimiter() *RateLimiter {
	limiterOnce.Do(func() {
		limiter = &RateLimiter{
			requests: make(map[string][]time.Time),
			config: RateLimitConfig{
				PublicLimit:      60,  // 60 req/min for public endpoints
				AuthLimit:        120, // 120 req/min for authenticated users
				ExternalAPILimit: 30,  // 30 req/min for external API
				WindowDuration:   time.Minute,
			},
		}
		// Start cleanup goroutine
		go limiter.cleanup()
		log.Printf("[RateLimit] Initialized with public=%d, auth=%d, external=%d per minute",
			limiter.config.PublicLimit, limiter.config.AuthLimit, limiter.config.ExternalAPILimit)
	})
	return limiter
}

// Allow checks if a request should be allowed based on rate limits
func (rl *RateLimiter) Allow(key string, limit int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.config.WindowDuration)

	// Filter requests within the sliding window
	var valid []time.Time
	for _, t := range rl.requests[key] {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= limit {
		rl.requests[key] = valid
		return false
	}

	rl.requests[key] = append(valid, now)
	return true
}

// cleanup periodically removes stale entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		windowStart := now.Add(-rl.config.WindowDuration)
		for key, times := range rl.requests {
			var valid []time.Time
			for _, t := range times {
				if t.After(windowStart) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = valid
			}
		}
		rl.mu.Unlock()
	}
}

// rateLimitResponse returns a 429 response with Retry-After header
func rateLimitResponse(e *core.RequestEvent) error {
	e.Response.Header().Set("Retry-After", "60")
	return e.JSON(http.StatusTooManyRequests, map[string]string{
		"error": "Rate limit exceeded. Please try again later.",
	})
}

// RateLimitPublic is middleware for public endpoints (tracks by IP)
func RateLimitPublic(e *core.RequestEvent) error {
	rl := GetRateLimiter()
	key := "public:" + e.RealIP()

	if !rl.Allow(key, rl.config.PublicLimit) {
		log.Printf("[RateLimit] Public limit exceeded for IP %s", e.RealIP())
		return rateLimitResponse(e)
	}
	return e.Next()
}

// RateLimitAuth is middleware for authenticated endpoints (tracks by user ID or IP)
func RateLimitAuth(e *core.RequestEvent) error {
	rl := GetRateLimiter()

	var key string
	if e.Auth != nil {
		key = "auth:" + e.Auth.Id
	} else {
		key = "auth:" + e.RealIP()
	}

	if !rl.Allow(key, rl.config.AuthLimit) {
		log.Printf("[RateLimit] Auth limit exceeded for %s", key)
		return rateLimitResponse(e)
	}
	return e.Next()
}

// RateLimitExternalAPI is middleware for external API endpoints (tracks by IP)
func RateLimitExternalAPI(e *core.RequestEvent) error {
	rl := GetRateLimiter()
	key := "external:" + e.RealIP()

	if !rl.Allow(key, rl.config.ExternalAPILimit) {
		log.Printf("[RateLimit] External API limit exceeded for IP %s", e.RealIP())
		return rateLimitResponse(e)
	}
	return e.Next()
}
