package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourusername/streammaxing/internal/services/secrets"
)

// SessionService manages JWT session creation, validation, and revocation.
type SessionService struct {
	secretsManager *secrets.Manager
	sessionStore   SessionStore
}

// SessionStore defines the interface for session revocation tracking.
type SessionStore interface {
	InvalidateSession(ctx context.Context, jti string) error
	IsSessionValid(ctx context.Context, jti string) (bool, error)
	CleanupExpiredSessions(ctx context.Context) error
}

// Claims represents the JWT claims for a user session.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	JTI      string `json:"jti"` // JWT ID for revocation
	jwt.RegisteredClaims
}

// NewSessionService creates a new session service.
func NewSessionService(secretsManager *secrets.Manager, sessionStore SessionStore) *SessionService {
	return &SessionService{
		secretsManager: secretsManager,
		sessionStore:   sessionStore,
	}
}

// CreateSession generates a new JWT session token with a 24-hour expiration.
func (s *SessionService) CreateSession(userID, username string) (string, string, error) {
	jwtSecret, err := s.secretsManager.GetJWTSecret()
	if err != nil {
		return "", "", fmt.Errorf("failed to get JWT secret: %w", err)
	}

	// Generate unique JWT ID for revocation tracking
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate JTI: %w", err)
	}
	jti := hex.EncodeToString(jtiBytes)

	claims := Claims{
		UserID:   userID,
		Username: username,
		JTI:      jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Reduced from 7 days
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, jti, nil
}

// ValidateSession parses and validates a JWT token, including revocation check.
func (s *SessionService) ValidateSession(ctx context.Context, tokenString string) (*Claims, error) {
	jwtSecret, err := s.secretsManager.GetJWTSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to get JWT secret: %w", err)
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check if session was revoked
	if claims.JTI != "" {
		isValid, err := s.sessionStore.IsSessionValid(ctx, claims.JTI)
		if err != nil {
			return nil, fmt.Errorf("failed to check session validity: %w", err)
		}
		if !isValid {
			return nil, fmt.Errorf("session has been revoked")
		}
	}

	return claims, nil
}

// RevokeSession invalidates a session by its JTI.
func (s *SessionService) RevokeSession(ctx context.Context, jti string) error {
	if jti == "" {
		return nil // No JTI to revoke (legacy token)
	}
	return s.sessionStore.InvalidateSession(ctx, jti)
}
