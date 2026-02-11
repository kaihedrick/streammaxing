package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// SessionDB provides session revocation tracking using the database.
// This implements the auth.SessionStore interface.
type SessionDB struct{}

// NewSessionDB creates a new session database accessor.
func NewSessionDB() *SessionDB {
	return &SessionDB{}
}

// InvalidateSession marks a session as revoked by its JTI.
func (s *SessionDB) InvalidateSession(ctx context.Context, jti string) error {
	query := `
		INSERT INTO revoked_sessions (jti, expires_at)
		VALUES ($1, now() + interval '25 hours')
		ON CONFLICT (jti) DO NOTHING
	`
	_, err := Pool.Exec(ctx, query, jti)
	return err
}

// IsSessionValid checks if a session has been revoked.
// Returns true if the session is still valid (not revoked).
func (s *SessionDB) IsSessionValid(ctx context.Context, jti string) (bool, error) {
	query := `SELECT 1 FROM revoked_sessions WHERE jti = $1`

	var exists int
	err := Pool.QueryRow(ctx, query, jti).Scan(&exists)
	if err == pgx.ErrNoRows {
		return true, nil // Not revoked = valid
	}
	if err != nil {
		return false, err
	}
	return false, nil // Found in revoked table = invalid
}

// CleanupExpiredSessions removes revoked sessions that have expired.
// Should be called periodically (e.g., daily) to keep the table small.
func (s *SessionDB) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM revoked_sessions WHERE expires_at < now()`
	_, err := Pool.Exec(ctx, query)
	return err
}
