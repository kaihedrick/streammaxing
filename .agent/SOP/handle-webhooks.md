# SOP: Handle Webhooks

## Overview
This document describes how to implement and handle webhooks from external services (Twitch EventSub, Discord, etc.) in StreamMaxing.

---

## Prerequisites
- HTTPS endpoint accessible from internet (use ngrok for local development)
- Webhook secret configured with provider
- Understanding of webhook security (signature verification)

---

## Webhook Flow

```
Provider (Twitch/Discord) → API Gateway → Lambda → Handler
                                                    ↓
                                            Verify Signature
                                                    ↓
                                            Process Event
                                                    ↓
                                            Return 200 OK
```

---

## General Webhook Handler Pattern

### 1. Create Webhook Handler

**File**: `backend/internal/handlers/webhooks.go`

```go
package handlers

import (
    "encoding/json"
    "io"
    "net/http"
    "time"
)

type WebhookHandler struct {
    // Dependencies
}

func NewWebhookHandler() *WebhookHandler {
    return &WebhookHandler{}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    // 1. Read request body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", http.StatusBadRequest)
        return
    }

    // 2. Verify signature
    if !h.verifySignature(r, body) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // 3. Parse payload
    var payload WebhookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // 4. Handle event (asynchronously to avoid timeout)
    go h.processEvent(payload)

    // 5. Return 200 OK immediately
    w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) processEvent(payload WebhookPayload) {
    // Process event in background
    log.Printf("Processing webhook event: %+v", payload)
}
```

---

## Twitch EventSub Webhooks

### 1. Webhook Handler

**File**: `backend/internal/handlers/twitch_webhook.go`

```go
package handlers

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io"
    "net/http"
    "os"
    "time"

    "your-module/internal/services/notifications"
)

type TwitchWebhookHandler struct {
    fanoutService *notifications.FanoutService
}

func NewTwitchWebhookHandler(fanoutService *notifications.FanoutService) *TwitchWebhookHandler {
    return &TwitchWebhookHandler{
        fanoutService: fanoutService,
    }
}

type TwitchWebhookPayload struct {
    Subscription TwitchSubscription     `json:"subscription"`
    Challenge    string                 `json:"challenge,omitempty"`
    Event        map[string]interface{} `json:"event,omitempty"`
}

type TwitchSubscription struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Version   string                 `json:"version"`
    Condition map[string]interface{} `json:"condition"`
}

func (h *TwitchWebhookHandler) HandleTwitchWebhook(w http.ResponseWriter, r *http.Request) {
    // Read body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", http.StatusBadRequest)
        return
    }

    // Verify signature
    messageID := r.Header.Get("Twitch-Eventsub-Message-Id")
    timestamp := r.Header.Get("Twitch-Eventsub-Message-Timestamp")
    signature := r.Header.Get("Twitch-Eventsub-Message-Signature")

    if !h.verifyTwitchSignature(messageID, timestamp, signature, body) {
        log.Printf("Invalid Twitch signature: %s", messageID)
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Parse payload
    var payload TwitchWebhookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Handle challenge response
    if payload.Challenge != "" {
        log.Printf("Twitch challenge received: %s", payload.Challenge)
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(payload.Challenge))
        return
    }

    // Handle notification
    if payload.Subscription.Type == "stream.online" {
        event := StreamOnlineEvent{
            ID:                   payload.Event["id"].(string),
            BroadcasterUserID:    payload.Event["broadcaster_user_id"].(string),
            BroadcasterUserLogin: payload.Event["broadcaster_user_login"].(string),
            BroadcasterUserName:  payload.Event["broadcaster_user_name"].(string),
            Type:                 payload.Event["type"].(string),
            StartedAt:            payload.Event["started_at"].(string),
        }

        // Process asynchronously
        go func() {
            ctx := context.Background()
            if err := h.fanoutService.HandleStreamOnline(ctx, messageID, event); err != nil {
                log.Printf("Fanout failed: %v", err)
            }
        }()
    }

    w.WriteHeader(http.StatusOK)
}

func (h *TwitchWebhookHandler) verifyTwitchSignature(messageID, timestamp, signature string, body []byte) bool {
    // Check timestamp (reject if older than 10 minutes)
    t, err := time.Parse(time.RFC3339, timestamp)
    if err != nil || time.Since(t) > 10*time.Minute {
        return false
    }

    // Verify HMAC-SHA256 signature
    secret := os.Getenv("TWITCH_WEBHOOK_SECRET")
    message := messageID + timestamp + string(body)

    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(message))
    computed := "sha256=" + hex.EncodeToString(h.Sum(nil))

    return hmac.Equal([]byte(signature), []byte(computed))
}

type StreamOnlineEvent struct {
    ID                   string `json:"id"`
    BroadcasterUserID    string `json:"broadcaster_user_id"`
    BroadcasterUserLogin string `json:"broadcaster_user_login"`
    BroadcasterUserName  string `json:"broadcaster_user_name"`
    Type                 string `json:"type"`
    StartedAt            string `json:"started_at"`
}
```

---

### 2. Add Webhook Route

**File**: `backend/cmd/lambda/main.go`

```go
func setupRouter() *chi.Mux {
    r := chi.NewRouter()

    // ... existing routes ...

    // Webhook routes (no auth middleware)
    r.Post("/webhooks/twitch", twitchWebhookHandler.HandleTwitchWebhook)

    return r
}
```

---

### 3. Test with Twitch CLI

**Install Twitch CLI**:
```bash
# macOS
brew install twitchdev/twitch/twitch-cli

# Windows
scoop install twitch-cli

# Linux
# Download from GitHub releases
```

**Configure**:
```bash
twitch configure
```

**Trigger Test Event**:
```bash
twitch event trigger stream.online \
  --broadcaster-user-id=123456789 \
  --broadcaster-user-login=test_user \
  --forward-address=http://localhost:3000/webhooks/twitch
```

---

## Discord Webhooks (Optional)

### 1. Webhook Handler

**File**: `backend/internal/handlers/discord_webhook.go`

```go
package handlers

import (
    "crypto/ed25519"
    "encoding/hex"
    "encoding/json"
    "io"
    "net/http"
    "os"
)

type DiscordWebhookHandler struct {
    cleanupHandler *CleanupHandler
}

func NewDiscordWebhookHandler(cleanupHandler *CleanupHandler) *DiscordWebhookHandler {
    return &DiscordWebhookHandler{
        cleanupHandler: cleanupHandler,
    }
}

type DiscordWebhookPayload struct {
    Type    int                    `json:"t"`
    OpCode  int                    `json:"op"`
    Data    map[string]interface{} `json:"d"`
    EventType string               `json:"t"`
}

func (h *DiscordWebhookHandler) HandleDiscordWebhook(w http.ResponseWriter, r *http.Request) {
    // Read body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", http.StatusBadRequest)
        return
    }

    // Verify signature (Discord uses Ed25519)
    signature := r.Header.Get("X-Signature-Ed25519")
    timestamp := r.Header.Get("X-Signature-Timestamp")

    if !h.verifyDiscordSignature(signature, timestamp, body) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Parse payload
    var payload DiscordWebhookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Handle ping (type 1)
    if payload.Type == 1 {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]int{"type": 1})
        return
    }

    // Handle events
    switch payload.EventType {
    case "GUILD_DELETE":
        guildID := payload.Data["id"].(string)
        go h.cleanupHandler.HandleBotRemoved(guildID)

    case "GUILD_MEMBER_REMOVE":
        guildID := payload.Data["guild_id"].(string)
        userID := payload.Data["user"].(map[string]interface{})["id"].(string)
        go h.cleanupHandler.HandleUserLeftGuild(guildID, userID)
    }

    w.WriteHeader(http.StatusOK)
}

func (h *DiscordWebhookHandler) verifyDiscordSignature(signature, timestamp string, body []byte) bool {
    // Get public key from environment
    publicKeyHex := os.Getenv("DISCORD_PUBLIC_KEY")
    publicKey, err := hex.DecodeString(publicKeyHex)
    if err != nil {
        return false
    }

    // Decode signature
    sig, err := hex.DecodeString(signature)
    if err != nil {
        return false
    }

    // Verify signature
    message := append([]byte(timestamp), body...)
    return ed25519.Verify(publicKey, message, sig)
}
```

---

## Webhook Security Best Practices

### 1. Always Verify Signatures

**Never trust webhook payload without verification**:

```go
if !verifySignature(r, body) {
    http.Error(w, "Invalid signature", http.StatusUnauthorized)
    return
}
```

---

### 2. Check Timestamp

Prevent replay attacks:

```go
t, err := time.Parse(time.RFC3339, timestamp)
if err != nil || time.Since(t) > 10*time.Minute {
    return false // Reject old webhooks
}
```

---

### 3. Use HTTPS Only

Webhooks must be delivered over HTTPS in production:

```
API Gateway → Force HTTPS
CloudFront → HTTPS only
```

---

### 4. Return 200 OK Quickly

Process events asynchronously:

```go
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    // ... verification ...

    // Return 200 OK immediately
    w.WriteHeader(http.StatusOK)

    // Process in background
    go h.processEvent(payload)
}
```

---

### 5. Implement Idempotency

Prevent duplicate processing:

```go
// Check if event already processed
isDuplicate, err := h.db.CheckDuplicate(messageID)
if isDuplicate {
    log.Printf("Duplicate event: %s", messageID)
    return nil
}

// Process event
// ...

// Record event
h.db.RecordEvent(messageID)
```

---

### 6. Log All Webhook Requests

```go
log.Printf("[WEBHOOK] Provider: %s, Event: %s, ID: %s", provider, eventType, messageID)
```

---

### 7. Rate Limit Webhook Endpoint

Prevent abuse:

```go
func RateLimitMiddleware(next http.Handler) http.Handler {
    limiter := rate.NewLimiter(100, 200) // 100 req/s, burst 200

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## Local Development with ngrok

### 1. Install ngrok

```bash
# macOS
brew install ngrok

# Windows
choco install ngrok

# Linux
wget https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-amd64.tgz
tar xvzf ngrok-*.tgz
sudo mv ngrok /usr/local/bin
```

---

### 2. Start ngrok Tunnel

```bash
ngrok http 3000
```

Output:
```
Forwarding  https://abc123.ngrok.io -> http://localhost:3000
```

---

### 3. Update Webhook URLs

**Twitch EventSub**:
```
Callback URL: https://abc123.ngrok.io/webhooks/twitch
```

**Discord Interactions**:
```
Interactions Endpoint URL: https://abc123.ngrok.io/webhooks/discord
```

---

### 4. Test Webhook

```bash
# Use Twitch CLI
twitch event trigger stream.online \
  --forward-address=https://abc123.ngrok.io/webhooks/twitch

# Or use curl
curl -X POST https://abc123.ngrok.io/webhooks/twitch \
  -H "Content-Type: application/json" \
  -d '{"test":"data"}'
```

---

## Error Handling

### 1. Invalid Signature

```go
if !verifySignature(r, body) {
    log.Printf("[WEBHOOK_ERROR] Invalid signature from %s", r.RemoteAddr)
    http.Error(w, "Invalid signature", http.StatusUnauthorized)
    return
}
```

---

### 2. Malformed JSON

```go
if err := json.Unmarshal(body, &payload); err != nil {
    log.Printf("[WEBHOOK_ERROR] Invalid JSON: %v", err)
    http.Error(w, "Invalid JSON", http.StatusBadRequest)
    return
}
```

---

### 3. Processing Error

```go
go func() {
    if err := h.processEvent(payload); err != nil {
        log.Printf("[WEBHOOK_ERROR] Processing failed: %v", err)
        // Don't return error to provider (already returned 200 OK)
    }
}()
```

---

## Monitoring

### CloudWatch Logs

```go
import "log"

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    log.Printf("[WEBHOOK_RECEIVED] Provider: Twitch, IP: %s", r.RemoteAddr)

    // ... process webhook ...

    log.Printf("[WEBHOOK_PROCESSED] ID: %s, Duration: %dms", messageID, duration)
}
```

---

### CloudWatch Metrics

```go
import "github.com/aws/aws-sdk-go/service/cloudwatch"

func PublishWebhookMetric(provider, status string) {
    svc := cloudwatch.New(session.Must(session.NewSession()))
    svc.PutMetricData(&cloudwatch.PutMetricDataInput{
        Namespace: aws.String("StreamMaxing/Webhooks"),
        MetricData: []*cloudwatch.MetricDatum{
            {
                MetricName: aws.String("WebhookReceived"),
                Value:      aws.Float64(1),
                Unit:       aws.String("Count"),
                Dimensions: []*cloudwatch.Dimension{
                    {
                        Name:  aws.String("Provider"),
                        Value: aws.String(provider),
                    },
                    {
                        Name:  aws.String("Status"),
                        Value: aws.String(status),
                    },
                },
            },
        },
    })
}
```

---

## Testing Checklist

- [ ] Signature verification works
- [ ] Challenge response works (Twitch)
- [ ] Ping response works (Discord)
- [ ] Event processing works
- [ ] Async processing doesn't block response
- [ ] Idempotency prevents duplicates
- [ ] Old webhooks rejected (timestamp check)
- [ ] Invalid signatures rejected
- [ ] Malformed JSON rejected
- [ ] Logs captured in CloudWatch
- [ ] Metrics published to CloudWatch

---

## Troubleshooting

### Issue: Webhook not received

**Possible Causes**:
- Firewall blocking requests
- Incorrect webhook URL
- SSL certificate issues

**Solution**:
```bash
# Test with curl
curl -X POST https://your-api-url/webhooks/twitch \
  -H "Content-Type: application/json" \
  -d '{"test":"data"}'

# Check CloudWatch Logs
aws logs tail /aws/lambda/streammaxing --follow
```

---

### Issue: Signature verification fails

**Possible Causes**:
- Wrong webhook secret
- Incorrect signature algorithm
- Body read twice (body consumed)

**Solution**:
```go
// Read body once and reuse
body, _ := io.ReadAll(r.Body)

// Verify signature with body
verifySignature(r, body)

// Parse JSON with body
json.Unmarshal(body, &payload)
```

---

### Issue: Lambda timeout

**Possible Causes**:
- Processing takes too long
- Not returning 200 OK quickly

**Solution**:
```go
// Return 200 OK immediately
w.WriteHeader(http.StatusOK)

// Process asynchronously
go h.processEvent(payload)
```

---

## Checklist

- [ ] Webhook handler implemented
- [ ] Signature verification implemented
- [ ] Timestamp check implemented
- [ ] Challenge/ping response implemented
- [ ] Async processing implemented
- [ ] Idempotency implemented
- [ ] Error handling implemented
- [ ] Logging added
- [ ] Metrics published
- [ ] Local testing with ngrok completed
- [ ] Production webhook URL configured
- [ ] CloudWatch logs verified
- [ ] Documentation updated
