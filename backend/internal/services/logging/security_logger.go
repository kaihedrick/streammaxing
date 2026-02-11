package logging

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// SecurityLogger provides structured security event logging.
// Events are logged as JSON to stdout (captured by CloudWatch Logs in Lambda).
type SecurityLogger struct{}

// SecurityEvent represents a structured security event.
type SecurityEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	EventType string                 `json:"event_type"`
	Severity  string                 `json:"severity"` // INFO, WARNING, CRITICAL
	UserID    string                 `json:"user_id,omitempty"`
	IPAddress string                 `json:"ip_address,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Success   bool                   `json:"success"`
}

// NewSecurityLogger creates a new security logger.
func NewSecurityLogger() *SecurityLogger {
	return &SecurityLogger{}
}

// LogEvent logs a security event as structured JSON.
func (sl *SecurityLogger) LogEvent(_ context.Context, event SecurityEvent) {
	event.Timestamp = time.Now()
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("[SECURITY_ERROR] Failed to marshal event: %v", err)
		return
	}
	log.Printf("[SECURITY] %s", string(eventJSON))
}

// LogAuthSuccess logs a successful authentication.
func (sl *SecurityLogger) LogAuthSuccess(ctx context.Context, userID, ipAddress string) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "auth_success",
		Severity:  "INFO",
		UserID:    userID,
		IPAddress: ipAddress,
		Success:   true,
	})
}

// LogAuthFailure logs a failed authentication attempt.
func (sl *SecurityLogger) LogAuthFailure(ctx context.Context, userID, ipAddress, reason string) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "auth_failure",
		Severity:  "WARNING",
		UserID:    userID,
		IPAddress: ipAddress,
		Success:   false,
		Details: map[string]interface{}{
			"reason": reason,
		},
	})
}

// LogPermissionDenied logs a permission denial event.
func (sl *SecurityLogger) LogPermissionDenied(ctx context.Context, userID, guildID, action string) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "permission_denied",
		Severity:  "WARNING",
		UserID:    userID,
		Success:   false,
		Details: map[string]interface{}{
			"guild_id": guildID,
			"action":   action,
		},
	})
}

// LogRateLimitExceeded logs a rate limit violation.
func (sl *SecurityLogger) LogRateLimitExceeded(ctx context.Context, userID, ipAddress, endpoint string) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "rate_limit_exceeded",
		Severity:  "WARNING",
		UserID:    userID,
		IPAddress: ipAddress,
		Success:   false,
		Details: map[string]interface{}{
			"endpoint": endpoint,
		},
	})
}

// LogWebhookSignatureFailure logs a failed webhook signature verification.
func (sl *SecurityLogger) LogWebhookSignatureFailure(ctx context.Context, ipAddress string) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "webhook_signature_failure",
		Severity:  "CRITICAL",
		IPAddress: ipAddress,
		Success:   false,
	})
}

// LogTokenEncryptionFailure logs a token encryption/decryption failure.
func (sl *SecurityLogger) LogTokenEncryptionFailure(ctx context.Context, streamerID string, err error) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "token_encryption_failure",
		Severity:  "CRITICAL",
		Success:   false,
		Details: map[string]interface{}{
			"streamer_id": streamerID,
			"error":       err.Error(),
		},
	})
}

// LogSessionRevoked logs a session revocation event.
func (sl *SecurityLogger) LogSessionRevoked(ctx context.Context, userID, jti string) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "session_revoked",
		Severity:  "INFO",
		UserID:    userID,
		Success:   true,
		Details: map[string]interface{}{
			"jti": jti,
		},
	})
}

// LogAnomalousActivity logs suspected anomalous activity.
func (sl *SecurityLogger) LogAnomalousActivity(ctx context.Context, userID, description string) {
	sl.LogEvent(ctx, SecurityEvent{
		EventType: "anomalous_activity",
		Severity:  "CRITICAL",
		UserID:    userID,
		Success:   false,
		Details: map[string]interface{}{
			"description": description,
		},
	})
}
