package middleware

import (
	"log"
	"net/http"
)

// Package-level CORS config set once at startup via SetCORSConfig.
var corsFrontendURL string
var corsIsProduction bool

// SetCORSConfig sets the CORS configuration for the middleware.
// Must be called before any request is served.
func SetCORSConfig(frontendURL string, isProduction bool) {
	corsFrontendURL = frontendURL
	corsIsProduction = isProduction
}

// CORSMiddleware adds CORS headers for frontend requests.
// SECURITY: No longer falls back to "*" wildcard. FRONTEND_URL must be set.
// In development, defaults to "http://localhost:5173".
func CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		frontendURL := corsFrontendURL
		if frontendURL == "" {
			if corsIsProduction {
				// SECURITY: In production, FRONTEND_URL must be set.
				// Reject request if not configured.
				log.Printf("[CORS_ERROR] FRONTEND_URL not set in production")
				http.Error(w, "Server misconfiguration", http.StatusInternalServerError)
				return
			}
			// Development fallback - explicit localhost, NOT wildcard
			frontendURL = "http://localhost:5173"
		}

		origin := r.Header.Get("Origin")

		// Only allow the configured origin (not wildcard)
		if origin == frontendURL {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}
