package middleware

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// ContextKey is a custom type for context keys
type ContextKey string

const UserIDKey ContextKey = "user_id"

// AuthMiddleware validates JWT tokens from cookies
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract JWT from cookie
		cookie, err := r.Cookie("session")
		if err != nil {
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

		// Parse and validate JWT
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
			// Verify signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// Extract user_id from claims
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

		// Add user_id to request context
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
