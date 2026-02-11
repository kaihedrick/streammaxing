package twitch

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// webhookSecret is the shared secret used for webhook signature verification.
// Set via SetWebhookSecret at startup rather than reading from env vars.
var webhookSecret string

// SetWebhookSecret configures the webhook secret used for signature verification.
// Must be called before handling any webhooks.
func SetWebhookSecret(secret string) {
	webhookSecret = secret
}

// VerifyWebhookSignature verifies the HMAC-SHA256 signature of a Twitch webhook request
func VerifyWebhookSignature(messageID, timestamp, signature string, body []byte) bool {
	// Check timestamp (reject if older than 10 minutes to prevent replay attacks)
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return false
	}
	if time.Since(t) > 10*time.Minute {
		return false
	}

	if webhookSecret == "" {
		return false
	}

	message := messageID + timestamp + string(body)

	h := hmac.New(sha256.New, []byte(webhookSecret))
	h.Write([]byte(message))
	computed := "sha256=" + hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(computed))
}
