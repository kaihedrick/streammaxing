package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourusername/streammaxing/internal/services/auth"
	"github.com/yourusername/streammaxing/internal/services/logging"
)

// ContextKey is a custom type for context keys
type ContextKey string

const (
	UserIDKey   ContextKey = "user_id"
	UsernameKey ContextKey = "username"
	JTIKey      ContextKey = "jti"
)

// sessionService is the shared session service used by AuthMiddleware.
// Set via SetSessionService before handling requests.
var sessionService *auth.SessionService

// securityLogger is the shared security logger.
var securityLogger *logging.SecurityLogger

// legacyJWTSecret is the JWT secret used for legacy token validation.
// Set via SetLegacyJWTSecret at startup from the centralized config.
var legacyJWTSecret string

// SetSessionService configures the session service used by AuthMiddleware.
func SetSessionService(ss *auth.SessionService) {
	sessionService = ss
}

// SetSecurityLogger configures the security logger used by middleware.
func SetSecurityLogger(sl *logging.SecurityLogger) {
	securityLogger = sl
}

// SetLegacyJWTSecret configures the JWT secret for legacy token fallback.
// This is loaded from the centralized config, not from os.Getenv.
func SetLegacyJWTSecret(secret string) {
	legacyJWTSecret = secret
}

// AuthMiddleware validates JWT tokens from cookies using the session service.
// Supports both new (JTI-enabled) and legacy (no JTI) tokens.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract JWT from cookie
		cookie, err := r.Cookie("session")
		if err != nil {
			if securityLogger != nil {
				securityLogger.LogAuthFailure(r.Context(), "", r.RemoteAddr, "missing_session_cookie")
			}
			// Debug: Log all cookies
			allCookies := r.Cookies()
			cookieNames := ""
			for i, c := range allCookies {
				if i > 0 {
					cookieNames += ", "
				}
				cookieNames += c.Name
			}
			if len(allCookies) == 0 {
				cookieNames = "none"
			}
			log.Printf("[AUTH_DEBUG] Missing session cookie. Available cookies: %s, Origin: %s", cookieNames, r.Header.Get("Origin"))
			http.Error(w, "Unauthorized: missing session cookie", http.StatusUnauthorized)
			return
		}

		// Use session service for validation (includes revocation check)
		if sessionService != nil {
			claims, err := sessionService.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				if securityLogger != nil {
					securityLogger.LogAuthFailure(r.Context(), "", r.RemoteAddr, err.Error())
				}
				http.Error(w, "Unauthorized: invalid or revoked session", http.StatusUnauthorized)
				return
			}

			// Add claims to request context
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UsernameKey, claims.Username)
			ctx = context.WithValue(ctx, JTIKey, claims.JTI)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Fallback: legacy JWT validation (when session service is not yet initialized)
		if legacyJWTSecret == "" {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(legacyJWTSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
			return
		}

		userID, ok := claims["user_id"].(string)
		if !ok || userID == "" {
			http.Error(w, "Unauthorized: missing user_id", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// GetUserID extracts user_id from request context
func GetUserID(r *http.Request) string {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		return ""
	}
	return userID
}

// GetJTI extracts the JWT ID from request context (for session revocation)
func GetJTI(r *http.Request) string {
	jti, ok := r.Context().Value(JTIKey).(string)
	if !ok {
		return ""
	}
	return jti
}
