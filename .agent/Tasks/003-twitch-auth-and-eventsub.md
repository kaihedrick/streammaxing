# Task 003: Twitch Auth and EventSub

## Status
Complete

## Overview
Implement Twitch OAuth flow for linking streamers, create EventSub subscriptions for stream.online events, handle webhook verification and notifications, and manage subscription lifecycle.

---

## Goals
1. Implement Twitch OAuth flow for streamer authorization
2. Fetch and store streamer information (broadcaster_id, login, display_name)
3. Generate and manage app access tokens for EventSub API
4. Create EventSub subscriptions for stream.online events
5. Implement webhook endpoint with signature verification
6. Handle webhook challenge response
7. Process stream.online notifications
8. Implement subscription cleanup logic
9. Link streamers to guilds

---

## Prerequisites
- Task 001 (Project Bootstrap) completed
- Task 002 (Discord Auth) completed
- Database schema created (streamers, guild_streamers, eventsub_subscriptions tables)
- Twitch application created in Twitch Developer Console
- Webhook endpoint accessible via HTTPS (use ngrok for local development)

---

## Twitch Application Setup

### Step 1: Create Twitch Application
1. Go to https://dev.twitch.tv/console/apps
2. Click "Register Your Application"
3. Name: "StreamMaxing"
4. OAuth Redirect URLs: `https://your-api-url/api/auth/twitch/callback`
5. For local development: `http://localhost:3000/api/auth/twitch/callback`
6. Category: Website Integration
7. Click "Create"

### Step 2: Get Credentials
1. Click "Manage" on your application
2. Copy Client ID
3. Click "New Secret" and copy Client Secret
4. Generate a random webhook secret (32+ characters)
5. Add to `.env`:
   ```bash
   TWITCH_CLIENT_ID=your_client_id
   TWITCH_CLIENT_SECRET=your_client_secret
   TWITCH_WEBHOOK_SECRET=your_random_secret_min_32_chars
   ```

---

## Backend Implementation

### File Structure
```
backend/
├── internal/
│   ├── handlers/
│   │   ├── twitch_auth.go      # OAuth handlers
│   │   └── webhooks.go         # Webhook handlers
│   ├── services/
│   │   └── twitch/
│   │       ├── oauth.go        # OAuth flow logic
│   │       ├── api.go          # Twitch API client
│   │       ├── eventsub.go     # EventSub subscription management
│   │       └── webhook.go      # Webhook signature verification
│   └── db/
│       ├── streamers.go        # Streamer database operations
│       └── subscriptions.go    # EventSub subscription tracking
```

### 1. Twitch OAuth Service

**File**: `backend/internal/services/twitch/oauth.go`

```go
package twitch

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
)

type OAuthService struct {
    clientID     string
    clientSecret string
    redirectURI  string
}

func NewOAuthService() *OAuthService {
    return &OAuthService{
        clientID:     os.Getenv("TWITCH_CLIENT_ID"),
        clientSecret: os.Getenv("TWITCH_CLIENT_SECRET"),
        redirectURI:  os.Getenv("API_BASE_URL") + "/api/auth/twitch/callback",
    }
}

func (s *OAuthService) GetAuthURL(state string) string {
    params := url.Values{
        "client_id":     {s.clientID},
        "redirect_uri":  {s.redirectURI},
        "response_type": {"code"},
        "scope":         {"user:read:email"},
        "state":         {state},
    }
    return "https://id.twitch.tv/oauth2/authorize?" + params.Encode()
}

type TokenResponse struct {
    AccessToken  string   `json:"access_token"`
    RefreshToken string   `json:"refresh_token"`
    ExpiresIn    int      `json:"expires_in"`
    TokenType    string   `json:"token_type"`
    Scope        []string `json:"scope"`
}

func (s *OAuthService) ExchangeCode(code string) (*TokenResponse, error) {
    data := url.Values{
        "client_id":     {s.clientID},
        "client_secret": {s.clientSecret},
        "grant_type":    {"authorization_code"},
        "code":          {code},
        "redirect_uri":  {s.redirectURI},
    }

    resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", data)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("twitch oauth error: %s", body)
    }

    var tokenResp TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }

    return &tokenResp, nil
}

type TwitchUser struct {
    ID              string `json:"id"`
    Login           string `json:"login"`
    DisplayName     string `json:"display_name"`
    ProfileImageURL string `json:"profile_image_url"`
}

func (s *OAuthService) GetUser(accessToken string) (*TwitchUser, error) {
    req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("Client-Id", s.clientID)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch user: %d", resp.StatusCode)
    }

    var result struct {
        Data []TwitchUser `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    if len(result.Data) == 0 {
        return nil, fmt.Errorf("no user data returned")
    }

    return &result.Data[0], nil
}

func (s *OAuthService) RefreshToken(refreshToken string) (*TokenResponse, error) {
    data := url.Values{
        "client_id":     {s.clientID},
        "client_secret": {s.clientSecret},
        "grant_type":    {"refresh_token"},
        "refresh_token": {refreshToken},
    }

    resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", data)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to refresh token: %d", resp.StatusCode)
    }

    var tokenResp TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }

    return &tokenResp, nil
}
```

### 2. App Access Token Management

**File**: `backend/internal/services/twitch/api.go`

```go
package twitch

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "sync"
    "time"
)

type APIClient struct {
    clientID       string
    clientSecret   string
    appAccessToken string
    tokenExpiry    time.Time
    mu             sync.RWMutex
}

func NewAPIClient() *APIClient {
    return &APIClient{
        clientID:     os.Getenv("TWITCH_CLIENT_ID"),
        clientSecret: os.Getenv("TWITCH_CLIENT_SECRET"),
    }
}

func (c *APIClient) GetAppAccessToken() (string, error) {
    c.mu.RLock()
    if c.appAccessToken != "" && time.Now().Before(c.tokenExpiry) {
        token := c.appAccessToken
        c.mu.RUnlock()
        return token, nil
    }
    c.mu.RUnlock()

    c.mu.Lock()
    defer c.mu.Unlock()

    // Double-check after acquiring write lock
    if c.appAccessToken != "" && time.Now().Before(c.tokenExpiry) {
        return c.appAccessToken, nil
    }

    data := url.Values{
        "client_id":     {c.clientID},
        "client_secret": {c.clientSecret},
        "grant_type":    {"client_credentials"},
    }

    resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", data)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("failed to get app access token: %s", body)
    }

    var tokenResp TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return "", err
    }

    c.appAccessToken = tokenResp.AccessToken
    c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-300) * time.Second) // Refresh 5 min early

    return c.appAccessToken, nil
}

type StreamData struct {
    ID           string    `json:"id"`
    UserID       string    `json:"user_id"`
    UserLogin    string    `json:"user_login"`
    UserName     string    `json:"user_name"`
    GameID       string    `json:"game_id"`
    GameName     string    `json:"game_name"`
    Title        string    `json:"title"`
    ViewerCount  int       `json:"viewer_count"`
    ThumbnailURL string    `json:"thumbnail_url"`
    StartedAt    time.Time `json:"started_at"`
}

func (c *APIClient) GetStreamData(broadcasterID string) (*StreamData, error) {
    token, err := c.GetAppAccessToken()
    if err != nil {
        return nil, err
    }

    url := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_id=%s", broadcasterID)
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Client-Id", c.clientID)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch stream data: %d", resp.StatusCode)
    }

    var result struct {
        Data []StreamData `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    if len(result.Data) == 0 {
        return nil, fmt.Errorf("stream not found or offline")
    }

    return &result.Data[0], nil
}
```

### 3. EventSub Service

**File**: `backend/internal/services/twitch/eventsub.go`

```go
package twitch

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

type EventSubService struct {
    apiClient *APIClient
}

func NewEventSubService(apiClient *APIClient) *EventSubService {
    return &EventSubService{
        apiClient: apiClient,
    }
}

type Subscription struct {
    ID        string                 `json:"id"`
    Status    string                 `json:"status"`
    Type      string                 `json:"type"`
    Version   string                 `json:"version"`
    Condition map[string]interface{} `json:"condition"`
    Transport Transport              `json:"transport"`
    CreatedAt string                 `json:"created_at"`
}

type Transport struct {
    Method   string `json:"method"`
    Callback string `json:"callback"`
    Secret   string `json:"secret,omitempty"`
}

type CreateSubscriptionRequest struct {
    Type      string                 `json:"type"`
    Version   string                 `json:"version"`
    Condition map[string]interface{} `json:"condition"`
    Transport Transport              `json:"transport"`
}

func (s *EventSubService) CreateStreamOnlineSubscription(broadcasterID string) (*Subscription, error) {
    token, err := s.apiClient.GetAppAccessToken()
    if err != nil {
        return nil, err
    }

    reqBody := CreateSubscriptionRequest{
        Type:    "stream.online",
        Version: "1",
        Condition: map[string]interface{}{
            "broadcaster_user_id": broadcasterID,
        },
        Transport: Transport{
            Method:   "webhook",
            Callback: os.Getenv("API_BASE_URL") + "/webhooks/twitch",
            Secret:   os.Getenv("TWITCH_WEBHOOK_SECRET"),
        },
    }

    body, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", "https://api.twitch.tv/helix/eventsub/subscriptions", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Client-Id", s.apiClient.clientID)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusConflict {
        return nil, fmt.Errorf("subscription already exists")
    }

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to create subscription: %d - %s", resp.StatusCode, body)
    }

    var result struct {
        Data []Subscription `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    if len(result.Data) == 0 {
        return nil, fmt.Errorf("no subscription data returned")
    }

    return &result.Data[0], nil
}

func (s *EventSubService) DeleteSubscription(subscriptionID string) error {
    token, err := s.apiClient.GetAppAccessToken()
    if err != nil {
        return err
    }

    url := fmt.Sprintf("https://api.twitch.tv/helix/eventsub/subscriptions?id=%s", subscriptionID)
    req, _ := http.NewRequest("DELETE", url, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Client-Id", s.apiClient.clientID)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusNoContent {
        return fmt.Errorf("failed to delete subscription: %d", resp.StatusCode)
    }

    return nil
}

func (s *EventSubService) ListSubscriptions() ([]Subscription, error) {
    token, err := s.apiClient.GetAppAccessToken()
    if err != nil {
        return nil, err
    }

    req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/eventsub/subscriptions", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Client-Id", s.apiClient.clientID)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to list subscriptions: %d", resp.StatusCode)
    }

    var result struct {
        Data []Subscription `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Data, nil
}
```

### 4. Webhook Verification

**File**: `backend/internal/services/twitch/webhook.go`

```go
package twitch

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "os"
    "time"
)

func VerifyWebhookSignature(messageID, timestamp, signature string, body []byte) bool {
    // Check timestamp (reject if older than 10 minutes)
    t, err := time.Parse(time.RFC3339, timestamp)
    if err != nil || time.Since(t) > 10*time.Minute {
        return false
    }

    secret := os.Getenv("TWITCH_WEBHOOK_SECRET")
    message := messageID + timestamp + string(body)

    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(message))
    computed := "sha256=" + hex.EncodeToString(h.Sum(nil))

    return hmac.Equal([]byte(signature), []byte(computed))
}
```

### 5. Twitch Auth Handler

**File**: `backend/internal/handlers/twitch_auth.go`

```go
package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "net/http"
    "strings"

    "your-module/internal/db"
    "your-module/internal/services/twitch"
)

type TwitchAuthHandler struct {
    oauth      *twitch.OAuthService
    eventsub   *twitch.EventSubService
    streamerDB *db.StreamerDB
    subDB      *db.SubscriptionDB
    guildDB    *db.GuildStreamerDB
}

func NewTwitchAuthHandler(
    streamerDB *db.StreamerDB,
    subDB *db.SubscriptionDB,
    guildDB *db.GuildStreamerDB,
) *TwitchAuthHandler {
    apiClient := twitch.NewAPIClient()
    return &TwitchAuthHandler{
        oauth:      twitch.NewOAuthService(),
        eventsub:   twitch.NewEventSubService(apiClient),
        streamerDB: streamerDB,
        subDB:      subDB,
        guildDB:    guildDB,
    }
}

func (h *TwitchAuthHandler) InitiateStreamerLink(w http.ResponseWriter, r *http.Request) {
    guildID := chi.URLParam(r, "guild_id")

    // TODO: Check user permission for guild

    // Generate state with guild_id
    randomBytes := make([]byte, 16)
    rand.Read(randomBytes)
    state := fmt.Sprintf("guild_id:%s:%s", guildID, hex.EncodeToString(randomBytes))

    // Store state in session
    http.SetCookie(w, &http.Cookie{
        Name:     "twitch_oauth_state",
        Value:    state,
        Path:     "/",
        MaxAge:   600,
        HttpOnly: true,
        Secure:   os.Getenv("ENVIRONMENT") == "production",
        SameSite: http.SameSiteStrictMode,
    })

    authURL := h.oauth.GetAuthURL(state)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "url": authURL,
    })
}

func (h *TwitchAuthHandler) TwitchCallback(w http.ResponseWriter, r *http.Request) {
    // Verify state
    stateCookie, err := r.Cookie("twitch_oauth_state")
    if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
        http.Error(w, "Invalid state", http.StatusBadRequest)
        return
    }

    // Extract guild_id from state
    stateParts := strings.Split(stateCookie.Value, ":")
    if len(stateParts) < 2 {
        http.Error(w, "Invalid state format", http.StatusBadRequest)
        return
    }
    guildID := stateParts[1]

    // Exchange code for token
    code := r.URL.Query().Get("code")
    tokenResp, err := h.oauth.ExchangeCode(code)
    if err != nil {
        http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
        return
    }

    // Fetch streamer info
    user, err := h.oauth.GetUser(tokenResp.AccessToken)
    if err != nil {
        http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
        return
    }

    // Store streamer in database
    streamerID, err := h.streamerDB.UpsertStreamer(
        user.ID,
        user.Login,
        user.DisplayName,
        user.ProfileImageURL,
        tokenResp.AccessToken,
        tokenResp.RefreshToken,
    )
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    // Create EventSub subscription
    subscription, err := h.eventsub.CreateStreamOnlineSubscription(user.ID)
    if err != nil {
        // Log error but don't fail (can retry later)
        log.Printf("Failed to create EventSub subscription: %v", err)
    } else {
        // Store subscription in database
        h.subDB.UpsertSubscription(streamerID, subscription.ID, subscription.Status)
    }

    // Link streamer to guild
    userID := r.Context().Value("user_id").(string) // From auth middleware
    if err := h.guildDB.LinkStreamerToGuild(guildID, streamerID, userID); err != nil {
        http.Error(w, "Failed to link streamer", http.StatusInternalServerError)
        return
    }

    // Clear state cookie
    http.SetCookie(w, &http.Cookie{
        Name:   "twitch_oauth_state",
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })

    // Redirect to dashboard
    frontendURL := os.Getenv("FRONTEND_URL")
    http.Redirect(w, r, fmt.Sprintf("%s/dashboard/guilds/%s", frontendURL, guildID), http.StatusTemporaryRedirect)
}
```

### 6. Webhook Handler

**File**: `backend/internal/handlers/webhooks.go`

```go
package handlers

import (
    "encoding/json"
    "io"
    "net/http"

    "your-module/internal/services/twitch"
)

type WebhookHandler struct {
    // Will be used in Task 004 for notification fanout
}

func NewWebhookHandler() *WebhookHandler {
    return &WebhookHandler{}
}

type WebhookPayload struct {
    Subscription Subscription           `json:"subscription"`
    Challenge    string                 `json:"challenge,omitempty"`
    Event        map[string]interface{} `json:"event,omitempty"`
}

type Subscription struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Version   string                 `json:"version"`
    Condition map[string]interface{} `json:"condition"`
}

func (h *WebhookHandler) HandleTwitchWebhook(w http.ResponseWriter, r *http.Request) {
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

    if !twitch.VerifyWebhookSignature(messageID, timestamp, signature, body) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Parse payload
    var payload WebhookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Handle challenge response
    if payload.Challenge != "" {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(payload.Challenge))
        return
    }

    // Handle notification (will implement in Task 004)
    if payload.Subscription.Type == "stream.online" {
        // TODO: Implement notification fanout
        log.Printf("Stream online event: %+v", payload.Event)
    }

    w.WriteHeader(http.StatusOK)
}
```

### 7. Database Layer

**File**: `backend/internal/db/streamers.go`

```go
package db

import (
    "context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type StreamerDB struct {
    pool *pgxpool.Pool
}

func NewStreamerDB(pool *pgxpool.Pool) *StreamerDB {
    return &StreamerDB{pool: pool}
}

func (db *StreamerDB) UpsertStreamer(
    broadcasterID, login, displayName, avatarURL, accessToken, refreshToken string,
) (uuid.UUID, error) {
    query := `
        INSERT INTO streamers (
            twitch_broadcaster_id, twitch_login, twitch_display_name,
            twitch_avatar_url, twitch_access_token, twitch_refresh_token
        )
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (twitch_broadcaster_id)
        DO UPDATE SET
            twitch_login = $2,
            twitch_display_name = $3,
            twitch_avatar_url = $4,
            twitch_access_token = $5,
            twitch_refresh_token = $6,
            last_updated = now()
        RETURNING id
    `

    var id uuid.UUID
    err := db.pool.QueryRow(
        context.Background(),
        query,
        broadcasterID, login, displayName, avatarURL, accessToken, refreshToken,
    ).Scan(&id)

    return id, err
}

func (db *StreamerDB) GetStreamerByBroadcasterID(broadcasterID string) (*Streamer, error) {
    query := `
        SELECT id, twitch_broadcaster_id, twitch_login, twitch_display_name, twitch_avatar_url
        FROM streamers WHERE twitch_broadcaster_id = $1
    `

    var streamer Streamer
    err := db.pool.QueryRow(context.Background(), query, broadcasterID).Scan(
        &streamer.ID,
        &streamer.BroadcasterID,
        &streamer.Login,
        &streamer.DisplayName,
        &streamer.AvatarURL,
    )

    return &streamer, err
}

type Streamer struct {
    ID            uuid.UUID
    BroadcasterID string
    Login         string
    DisplayName   string
    AvatarURL     string
}
```

**File**: `backend/internal/db/subscriptions.go`

```go
package db

import (
    "context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type SubscriptionDB struct {
    pool *pgxpool.Pool
}

func NewSubscriptionDB(pool *pgxpool.Pool) *SubscriptionDB {
    return &SubscriptionDB{pool: pool}
}

func (db *SubscriptionDB) UpsertSubscription(streamerID uuid.UUID, subscriptionID, status string) error {
    query := `
        INSERT INTO eventsub_subscriptions (streamer_id, subscription_id, status, last_verified)
        VALUES ($1, $2, $3, now())
        ON CONFLICT (subscription_id)
        DO UPDATE SET status = $3, last_verified = now()
    `
    _, err := db.pool.Exec(context.Background(), query, streamerID, subscriptionID, status)
    return err
}

func (db *SubscriptionDB) GetSubscriptionByStreamerID(streamerID uuid.UUID) (*EventSubSubscription, error) {
    query := `
        SELECT streamer_id, subscription_id, status, created_at, last_verified
        FROM eventsub_subscriptions WHERE streamer_id = $1
    `

    var sub EventSubSubscription
    err := db.pool.QueryRow(context.Background(), query, streamerID).Scan(
        &sub.StreamerID,
        &sub.SubscriptionID,
        &sub.Status,
        &sub.CreatedAt,
        &sub.LastVerified,
    )

    return &sub, err
}

func (db *SubscriptionDB) DeleteSubscription(subscriptionID string) error {
    query := `DELETE FROM eventsub_subscriptions WHERE subscription_id = $1`
    _, err := db.pool.Exec(context.Background(), query, subscriptionID)
    return err
}

type EventSubSubscription struct {
    StreamerID     uuid.UUID
    SubscriptionID string
    Status         string
    CreatedAt      time.Time
    LastVerified   time.Time
}
```

**File**: `backend/internal/db/guild_streamers.go`

```go
package db

import (
    "context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type GuildStreamerDB struct {
    pool *pgxpool.Pool
}

func NewGuildStreamerDB(pool *pgxpool.Pool) *GuildStreamerDB {
    return &GuildStreamerDB{pool: pool}
}

func (db *GuildStreamerDB) LinkStreamerToGuild(guildID string, streamerID uuid.UUID, addedBy string) error {
    query := `
        INSERT INTO guild_streamers (guild_id, streamer_id, enabled, added_by)
        VALUES ($1, $2, true, $3)
        ON CONFLICT (guild_id, streamer_id) DO NOTHING
    `
    _, err := db.pool.Exec(context.Background(), query, guildID, streamerID, addedBy)
    return err
}

func (db *GuildStreamerDB) GetGuildStreamers(guildID string) ([]Streamer, error) {
    query := `
        SELECT s.id, s.twitch_broadcaster_id, s.twitch_login, s.twitch_display_name, s.twitch_avatar_url
        FROM streamers s
        JOIN guild_streamers gs ON s.id = gs.streamer_id
        WHERE gs.guild_id = $1 AND gs.enabled = true
    `

    rows, err := db.pool.Query(context.Background(), query, guildID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var streamers []Streamer
    for rows.Next() {
        var s Streamer
        if err := rows.Scan(&s.ID, &s.BroadcasterID, &s.Login, &s.DisplayName, &s.AvatarURL); err != nil {
            return nil, err
        }
        streamers = append(streamers, s)
    }

    return streamers, nil
}

func (db *GuildStreamerDB) UnlinkStreamerFromGuild(guildID string, streamerID uuid.UUID) error {
    query := `DELETE FROM guild_streamers WHERE guild_id = $1 AND streamer_id = $2`
    _, err := db.pool.Exec(context.Background(), query, guildID, streamerID)
    return err
}
```

---

## API Endpoints

### Twitch Auth Routes

**GET /api/guilds/:guild_id/streamers/link**
- Description: Initiate Twitch OAuth flow
- Auth: Required (JWT + guild admin permission)
- Response: `{ "url": "https://id.twitch.tv/..." }`

**GET /api/auth/twitch/callback**
- Description: Handle Twitch OAuth callback
- Auth: Required (JWT)
- Query Parameters:
  - `code`: Authorization code
  - `state`: Contains guild_id
- Response: Redirect to dashboard

### Webhook Routes

**POST /webhooks/twitch**
- Description: Receive Twitch EventSub webhooks
- Auth: Signature verification (HMAC-SHA256)
- Headers:
  - `Twitch-Eventsub-Message-Id`
  - `Twitch-Eventsub-Message-Timestamp`
  - `Twitch-Eventsub-Message-Signature`
- Response: 200 OK or challenge string

### Streamer Management Routes (for Task 004)

**GET /api/guilds/:guild_id/streamers**
- Description: List streamers linked to guild
- Auth: Required (JWT + guild admin permission)
- Response: Array of streamer objects

**DELETE /api/guilds/:guild_id/streamers/:streamer_id**
- Description: Unlink streamer from guild
- Auth: Required (JWT + guild admin permission)
- Response: `{ "message": "Streamer unlinked" }`

---

## Testing Checklist

### Local Development Setup
- [ ] Use ngrok to expose local server: `ngrok http 3000`
- [ ] Update Twitch callback URL to ngrok URL
- [ ] Update EventSub webhook callback to ngrok URL

### Manual Testing
- [ ] Twitch OAuth flow completes successfully
- [ ] Streamer information stored in database
- [ ] Tokens stored securely
- [ ] EventSub subscription created successfully
- [ ] Subscription status is "enabled" after challenge response
- [ ] Webhook signature verification works
- [ ] Challenge response returns correct string
- [ ] Stream.online event logged correctly
- [ ] Streamer linked to guild in database

### Twitch CLI Testing
Use Twitch CLI for local webhook testing:

```bash
# Install Twitch CLI
brew install twitchdev/twitch/twitch-cli

# Configure
twitch configure

# Trigger test event
twitch event trigger stream.online --forward-address=http://localhost:3000/webhooks/twitch
```

### Security Testing
- [ ] Invalid webhook signature rejected
- [ ] Old timestamp rejected (> 10 minutes)
- [ ] Duplicate message IDs handled correctly
- [ ] CSRF protection works (invalid state rejected)

---

## Cleanup Logic (for Task 006)

### Orphaned Streamers
Streamers not linked to any guilds should be cleaned up:

```sql
DELETE FROM streamers
WHERE id NOT IN (SELECT DISTINCT streamer_id FROM guild_streamers);
```

### Failed Subscriptions
Subscriptions with "failed" status should be retried or deleted:

```go
func (h *CleanupHandler) RetryFailedSubscriptions() {
    subs, _ := h.subDB.GetFailedSubscriptions()
    for _, sub := range subs {
        streamer, _ := h.streamerDB.GetStreamerByID(sub.StreamerID)
        newSub, err := h.eventsub.CreateStreamOnlineSubscription(streamer.BroadcasterID)
        if err == nil {
            h.subDB.UpsertSubscription(sub.StreamerID, newSub.ID, newSub.Status)
        }
    }
}
```

---

## Environment Variables

Add to `backend/.env`:
```bash
# Twitch OAuth
TWITCH_CLIENT_ID=your_client_id
TWITCH_CLIENT_SECRET=your_client_secret
TWITCH_WEBHOOK_SECRET=your_webhook_secret_min_32_chars

# API URL (must be HTTPS for production)
API_BASE_URL=https://your-api-gateway-url
```

---

## Next Steps

After completing this task:
1. Task 004: Implement notification fanout system
2. Test end-to-end flow: Link streamer → Stream goes live → Webhook received
3. Implement token refresh logic for expired Twitch tokens

---

## Notes

- App access tokens expire after ~60 days (refresh automatically)
- User access tokens expire after ~4 hours (use refresh token)
- EventSub subscriptions auto-delete after repeated delivery failures
- Consider encrypting Twitch tokens at rest using AWS KMS (future enhancement)
- Use Twitch CLI for local testing (no need to actually go live)
- Webhook signature verification is CRITICAL for security
