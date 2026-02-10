package twitch

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"
)

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

	secret := os.Getenv("TWITCH_WEBHOOK_SECRET")
	if secret == "" {
		return false
	}

	message := messageID + timestamp + string(body)

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	computed := "sha256=" + hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(computed))
}
