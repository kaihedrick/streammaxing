package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-key rate limiting using the token bucket algorithm.
// NOTE: In AWS Lambda, each instance maintains its own rate limiter state.
// For stricter cross-instance rate limiting, consider using DynamoDB or ElastiCache.
type RateLimiter struct {
	limiters map[string]*rateLimiterEntry
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter with the given requests-per-second and burst size.
func NewRateLimiter(requestsPerSecond int, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rps:      rate.Limit(requestsPerSecond),
		burst:    burst,
	}

	// Cleanup stale entries every 5 minutes
	go rl.cleanupLoop()

	return rl
}

// getLimiter returns (or creates) a rate limiter for the given key.
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	entry, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		rl.mu.Lock()
		entry.lastSeen = time.Now()
		rl.mu.Unlock()
		return entry.limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists = rl.limiters[key]; exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rl.rps, rl.burst)
	rl.limiters[key] = &rateLimiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	return limiter
}

// UserRateLimitMiddleware applies per-user rate limiting.
// Uses user_id from context if available, falls back to IP address.
func (rl *RateLimiter) UserRateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := GetUserID(r)
		if key == "" {
			key = r.RemoteAddr // Fallback to IP for unauthenticated requests
		}

		limiter := rl.getLimiter("user:" + key)

		if !limiter.Allow() {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// cleanupLoop removes stale rate limiter entries every 5 minutes.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for key, entry := range rl.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(rl.limiters, key)
			}
		}
		rl.mu.Unlock()
	}
}

// GlobalRateLimiter provides a single global rate limiter for all requests.
type GlobalRateLimiter struct {
	limiter *rate.Limiter
}

// NewGlobalRateLimiter creates a global rate limiter.
func NewGlobalRateLimiter(requestsPerSecond int, burst int) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
	}
}

// Middleware applies global rate limiting.
func (gl *GlobalRateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !gl.limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}

		next.ServeHTTP(w, r)
	}
}
