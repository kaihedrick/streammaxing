package middleware

import "net/http"

// SecurityHeadersMiddleware sets standard security response headers on every response.
// Apply early in the middleware chain (before CORS) so headers are present on all responses.
func SecurityHeadersMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Prevent browsers from MIME-sniffing the content-type
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Deny all framing to prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Control how much referrer information is sent
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Basic CSP — API responses are JSON; the SPA is served separately via S3/CloudFront
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		next.ServeHTTP(w, r)
	}
}
