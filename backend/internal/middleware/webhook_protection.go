package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// WebhookProtection provides rate limiting and idempotency for webhook endpoints.
type WebhookProtection struct {
	messageIDs  map[string]time.Time // messageID -> processedAt
	mu          sync.RWMutex
	rateLimiter *rate.Limiter
}

// NewWebhookProtection creates a new webhook protection handler.
func NewWebhookProtection() *WebhookProtection {
	wp := &WebhookProtection{
		messageIDs:  make(map[string]time.Time),
		rateLimiter: rate.NewLimiter(rate.Limit(100), 200), // 100 webhooks/sec, burst 200
	}

	// Cleanup old message IDs periodically
	go wp.cleanupLoop()

	return wp
}

// Middleware applies webhook rate limiting and idempotency checking.
func (wp *WebhookProtection) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Rate limiting
		if !wp.rateLimiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Idempotency check using Twitch message ID
		messageID := r.Header.Get("Twitch-Eventsub-Message-Id")
		if messageID != "" {
			if wp.isDuplicate(messageID) {
				// Already processed, return success to prevent Twitch from retrying
				w.WriteHeader(http.StatusOK)
				return
			}
			// Mark as being processed
			wp.markProcessed(messageID)
		}

		next.ServeHTTP(w, r)
	}
}

// isDuplicate checks if a message has already been processed.
func (wp *WebhookProtection) isDuplicate(messageID string) bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	_, exists := wp.messageIDs[messageID]
	return exists
}

// markProcessed records that a message has been processed.
func (wp *WebhookProtection) markProcessed(messageID string) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.messageIDs[messageID] = time.Now()
}

// cleanupLoop removes old message IDs every minute to prevent memory leaks.
func (wp *WebhookProtection) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		wp.mu.Lock()
		cutoff := time.Now().Add(-15 * time.Minute)
		for id, timestamp := range wp.messageIDs {
			if timestamp.Before(cutoff) {
				delete(wp.messageIDs, id)
			}
		}
		wp.mu.Unlock()
	}
}
