package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/yourusername/streammaxing/internal/services/logging"
	"github.com/yourusername/streammaxing/internal/services/notifications"
	"github.com/yourusername/streammaxing/internal/services/twitch"
)

// WebhookHandler handles incoming webhook events
type WebhookHandler struct {
	FanoutService  *notifications.FanoutService
	securityLogger *logging.SecurityLogger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(fanoutService *notifications.FanoutService, securityLogger *logging.SecurityLogger) *WebhookHandler {
	return &WebhookHandler{
		FanoutService:  fanoutService,
		securityLogger: securityLogger,
	}
}

// WebhookPayload represents the Twitch EventSub webhook payload
type WebhookPayload struct {
	Subscription WebhookSubscription    `json:"subscription"`
	Challenge    string                 `json:"challenge,omitempty"`
	Event        map[string]interface{} `json:"event,omitempty"`
}

// WebhookSubscription represents the subscription info in a webhook payload
type WebhookSubscription struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Version   string                 `json:"version"`
	Status    string                 `json:"status"`
	Condition map[string]interface{} `json:"condition"`
}

// HandleTwitchWebhook processes incoming Twitch EventSub webhook events
func (h *WebhookHandler) HandleTwitchWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit request body size to prevent abuse
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB max

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to read body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature
	messageID := r.Header.Get("Twitch-Eventsub-Message-Id")
	timestamp := r.Header.Get("Twitch-Eventsub-Message-Timestamp")
	signature := r.Header.Get("Twitch-Eventsub-Message-Signature")

	if !twitch.VerifyWebhookSignature(messageID, timestamp, signature, body) {
		log.Printf("[WEBHOOK_ERROR] Invalid signature for message %s from %s", messageID, r.RemoteAddr)
		// SECURITY: Log webhook signature failures
		if h.securityLogger != nil {
			h.securityLogger.LogWebhookSignatureFailure(r.Context(), r.RemoteAddr)
		}
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse payload
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[WEBHOOK_ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle challenge response (subscription verification)
	if payload.Challenge != "" {
		log.Printf("[WEBHOOK] Challenge response for subscription %s", payload.Subscription.ID)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(payload.Challenge))
		return
	}

	// Handle stream.online notification
	if payload.Subscription.Type == "stream.online" && payload.Event != nil {
		event := notifications.StreamOnlineEvent{
			BroadcasterUserID:    getStringFromMap(payload.Event, "broadcaster_user_id"),
			BroadcasterUserLogin: getStringFromMap(payload.Event, "broadcaster_user_login"),
			BroadcasterUserName:  getStringFromMap(payload.Event, "broadcaster_user_name"),
			Type:                 getStringFromMap(payload.Event, "type"),
			StartedAt:            getStringFromMap(payload.Event, "started_at"),
		}
		eventID := getStringFromMap(payload.Event, "id")
		if eventID == "" {
			eventID = messageID // fallback to message ID
		}

		log.Printf("[WEBHOOK] stream.online: %s (%s)", event.BroadcasterUserName, event.BroadcasterUserID)

		// Process notification fanout synchronously.
		// In Lambda, goroutines get frozen after the handler returns,
		// so we must complete the fanout before returning 200 to Twitch.
		// Twitch allows 10s for a response; fanout typically takes 1-3s.
		ctx := r.Context()
		if err := h.FanoutService.HandleStreamOnline(ctx, eventID, event); err != nil {
			log.Printf("[WEBHOOK_ERROR] Fanout failed: %v", err)
		}
	}

	// Always return 200 OK to Twitch
	w.WriteHeader(http.StatusOK)
}

// getStringFromMap safely extracts a string value from a map
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
