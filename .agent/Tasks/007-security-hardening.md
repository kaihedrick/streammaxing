# Task 007: Security Hardening & Production Readiness

## Status
In Progress (Implementation Complete - Pending Deployment & Testing)

## Overview
Comprehensive security hardening of the StreamMaxing v3 system to address critical vulnerabilities including OAuth token encryption, authorization enforcement, rate limiting, secrets management, session hardening, and security monitoring.

---

## Goals
1. Encrypt OAuth tokens at rest using AWS KMS
2. Enforce strict guild authorization on every endpoint with real-time validation
3. Add webhook rate limiting and idempotency tracking
4. Harden cookie and session configuration
5. Centralize secrets management using AWS Secrets Manager
6. Implement comprehensive security event monitoring and alerting
7. Add input validation and sanitization
8. Implement API rate limiting across all endpoints
9. Add audit logging for sensitive operations
10. Implement token rotation and session revocation

---

## Prerequisites
- All previous tasks (001-006) completed
- AWS KMS key created for encryption
- AWS Secrets Manager configured
- CloudWatch access for monitoring
- Database access for schema migrations
- Production environment validated

---

## Security Vulnerabilities Identified

### CRITICAL
1. **OAuth tokens stored in plaintext** - Twitch access/refresh tokens unencrypted in database
2. **No encryption at rest** - Sensitive data exposed if database compromised

### HIGH
1. **No rate limiting** - System vulnerable to abuse and DDoS
2. **Secrets in environment variables** - No rotation, no centralized management
3. **Stale permission caching** - Authorization not re-validated after initial login

### MODERATE
1. **Long JWT expiration (7 days)** - Increased attack window
2. **No security event monitoring** - Cannot detect suspicious activity
3. **No session revocation capability** - Logout only clears cookie
4. **Webhook processing could timeout** - Synchronous fanout in Lambda
5. **CORS wildcard in development** - Potential security issue if deployed

---

## Implementation Plan

### Phase 1: Critical Security Fixes

#### 1.1 Encrypt OAuth Tokens at Rest

**Goal**: Encrypt Twitch OAuth tokens using AWS KMS before storing in database

**AWS KMS Setup**:

```bash
# Create KMS key
aws kms create-key \
  --description "StreamMaxing OAuth token encryption" \
  --key-usage ENCRYPT_DECRYPT \
  --origin AWS_KMS

# Create alias
aws kms create-alias \
  --alias-name alias/streammaxing-oauth \
  --target-key-id <key-id>

# Grant Lambda access to key
aws kms create-grant \
  --key-id <key-id> \
  --grantee-principal arn:aws:iam::ACCOUNT_ID:role/lambda-execution-role \
  --operations Encrypt Decrypt
```

**File**: `backend/internal/services/encryption/kms.go`

```go
package encryption

import (
    "context"
    "encoding/base64"
    "os"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/kms"
)

type KMSService struct {
    client *kms.Client
    keyID  string
}

func NewKMSService() (*KMSService, error) {
    cfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        return nil, err
    }

    return &KMSService{
        client: kms.NewFromConfig(cfg),
        keyID:  os.Getenv("KMS_KEY_ID"), // e.g., "alias/streammaxing-oauth"
    }, nil
}

func (s *KMSService) Encrypt(plaintext string) (string, error) {
    result, err := s.client.Encrypt(context.Background(), &kms.EncryptInput{
        KeyId:     &s.keyID,
        Plaintext: []byte(plaintext),
    })
    if err != nil {
        return "", err
    }

    // Encode to base64 for storage
    return base64.StdEncoding.EncodeToString(result.CiphertextBlob), nil
}

func (s *KMSService) Decrypt(ciphertext string) (string, error) {
    // Decode from base64
    ciphertextBlob, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }

    result, err := s.client.Decrypt(context.Background(), &kms.DecryptInput{
        CiphertextBlob: ciphertextBlob,
    })
    if err != nil {
        return "", err
    }

    return string(result.Plaintext), nil
}
```

**Update Streamer DB**:

**File**: `backend/internal/db/streamers.go`

```go
func (db *StreamerDB) UpsertStreamer(
    broadcasterID, login, displayName, avatarURL, accessToken, refreshToken string,
    kmsService *encryption.KMSService,
) (uuid.UUID, error) {
    // Encrypt tokens before storing
    encryptedAccessToken, err := kmsService.Encrypt(accessToken)
    if err != nil {
        return uuid.Nil, fmt.Errorf("failed to encrypt access token: %w", err)
    }

    encryptedRefreshToken, err := kmsService.Encrypt(refreshToken)
    if err != nil {
        return uuid.Nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
    }

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
    err = db.pool.QueryRow(
        context.Background(),
        query,
        broadcasterID, login, displayName, avatarURL,
        encryptedAccessToken, encryptedRefreshToken,
    ).Scan(&id)

    return id, err
}

func (db *StreamerDB) GetStreamerTokens(streamerID uuid.UUID, kmsService *encryption.KMSService) (accessToken, refreshToken string, err error) {
    query := `
        SELECT twitch_access_token, twitch_refresh_token
        FROM streamers WHERE id = $1
    `

    var encryptedAccess, encryptedRefresh string
    err = db.pool.QueryRow(context.Background(), query, streamerID).Scan(&encryptedAccess, &encryptedRefresh)
    if err != nil {
        return "", "", err
    }

    // Decrypt tokens
    accessToken, err = kmsService.Decrypt(encryptedAccess)
    if err != nil {
        return "", "", fmt.Errorf("failed to decrypt access token: %w", err)
    }

    refreshToken, err = kmsService.Decrypt(encryptedRefresh)
    if err != nil {
        return "", "", fmt.Errorf("failed to decrypt refresh token: %w", err)
    }

    return accessToken, refreshToken, nil
}
```

**Migration Script**:

**File**: `backend/migrations/007_encrypt_existing_tokens.go`

```go
package main

import (
    "context"
    "log"

    "your-module/internal/db"
    "your-module/internal/services/encryption"
)

// This script encrypts existing plaintext tokens in the database
// Run ONCE during deployment
func main() {
    kmsService, err := encryption.NewKMSService()
    if err != nil {
        log.Fatal(err)
    }

    streamerDB := db.NewStreamerDB(dbPool)

    // Get all streamers
    query := `SELECT id, twitch_access_token, twitch_refresh_token FROM streamers`
    rows, err := dbPool.Query(context.Background(), query)
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    for rows.Next() {
        var id uuid.UUID
        var accessToken, refreshToken string
        rows.Scan(&id, &accessToken, &refreshToken)

        // Check if already encrypted (base64 format)
        if isBase64(accessToken) {
            log.Printf("Streamer %s already encrypted, skipping", id)
            continue
        }

        // Encrypt tokens
        encryptedAccess, err := kmsService.Encrypt(accessToken)
        if err != nil {
            log.Printf("Failed to encrypt token for streamer %s: %v", id, err)
            continue
        }

        encryptedRefresh, err := kmsService.Encrypt(refreshToken)
        if err != nil {
            log.Printf("Failed to encrypt refresh token for streamer %s: %v", id, err)
            continue
        }

        // Update database
        updateQuery := `
            UPDATE streamers
            SET twitch_access_token = $1, twitch_refresh_token = $2
            WHERE id = $3
        `
        _, err = dbPool.Exec(context.Background(), updateQuery, encryptedAccess, encryptedRefresh, id)
        if err != nil {
            log.Printf("Failed to update streamer %s: %v", id, err)
            continue
        }

        log.Printf("Encrypted tokens for streamer %s", id)
    }

    log.Println("Token encryption migration completed")
}

func isBase64(s string) bool {
    _, err := base64.StdEncoding.DecodeString(s)
    return err == nil
}
```

---

#### 1.2 Enforce Strict Guild Authorization

**Goal**: Re-validate guild permissions on every request instead of relying on cached data

**File**: `backend/internal/services/authorization/guild_auth.go`

```go
package authorization

import (
    "context"
    "fmt"
    "sync"
    "time"

    "your-module/internal/services/discord"
)

type GuildAuthService struct {
    discordAPI *discord.APIClient
    cache      *guildPermissionCache
}

type guildPermissionCache struct {
    mu    sync.RWMutex
    data  map[string]map[string]guildPermission // userID -> guildID -> permission
    ttl   time.Duration
}

type guildPermission struct {
    hasAccess  bool
    isAdmin    bool
    cachedAt   time.Time
}

func NewGuildAuthService(discordAPI *discord.APIClient) *GuildAuthService {
    return &GuildAuthService{
        discordAPI: discordAPI,
        cache: &guildPermissionCache{
            data: make(map[string]map[string]guildPermission),
            ttl:  5 * time.Minute, // Short TTL for security
        },
    }
}

func (s *GuildAuthService) CheckGuildAdmin(ctx context.Context, userID, guildID string) (bool, error) {
    // Check cache first
    if perm, ok := s.cache.get(userID, guildID); ok {
        if time.Since(perm.cachedAt) < s.cache.ttl {
            return perm.isAdmin, nil
        }
    }

    // Fetch fresh data from Discord API
    guilds, err := s.discordAPI.GetUserGuilds(userID)
    if err != nil {
        return false, fmt.Errorf("failed to fetch user guilds: %w", err)
    }

    // Check if user is in guild and has admin permission
    for _, guild := range guilds {
        if guild.ID == guildID {
            // 0x8 = ADMINISTRATOR permission
            isAdmin := (guild.Permissions & 0x8) != 0

            // Cache the result
            s.cache.set(userID, guildID, guildPermission{
                hasAccess: true,
                isAdmin:   isAdmin,
                cachedAt:  time.Now(),
            })

            return isAdmin, nil
        }
    }

    // User not in guild
    s.cache.set(userID, guildID, guildPermission{
        hasAccess: false,
        isAdmin:   false,
        cachedAt:  time.Now(),
    })

    return false, fmt.Errorf("user not in guild")
}

func (s *GuildAuthService) CheckGuildMember(ctx context.Context, userID, guildID string) (bool, error) {
    // Similar to CheckGuildAdmin but only checks membership
    if perm, ok := s.cache.get(userID, guildID); ok {
        if time.Since(perm.cachedAt) < s.cache.ttl {
            return perm.hasAccess, nil
        }
    }

    guilds, err := s.discordAPI.GetUserGuilds(userID)
    if err != nil {
        return false, err
    }

    for _, guild := range guilds {
        if guild.ID == guildID {
            s.cache.set(userID, guildID, guildPermission{
                hasAccess: true,
                isAdmin:   (guild.Permissions & 0x8) != 0,
                cachedAt:  time.Now(),
            })
            return true, nil
        }
    }

    return false, fmt.Errorf("user not in guild")
}

func (c *guildPermissionCache) get(userID, guildID string) (guildPermission, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    if userGuilds, ok := c.data[userID]; ok {
        if perm, ok := userGuilds[guildID]; ok {
            return perm, true
        }
    }
    return guildPermission{}, false
}

func (c *guildPermissionCache) set(userID, guildID string, perm guildPermission) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if _, ok := c.data[userID]; !ok {
        c.data[userID] = make(map[string]guildPermission)
    }
    c.data[userID][guildID] = perm
}

// Invalidate cache for a user
func (c *guildPermissionCache) Invalidate(userID string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.data, userID)
}
```

**Authorization Middleware**:

**File**: `backend/internal/middleware/guild_auth.go`

```go
package middleware

import (
    "context"
    "net/http"

    "github.com/go-chi/chi/v5"
    "your-module/internal/services/authorization"
)

func GuildAdminMiddleware(authService *authorization.GuildAuthService) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID, ok := r.Context().Value("user_id").(string)
            if !ok {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            guildID := chi.URLParam(r, "guild_id")
            if guildID == "" {
                http.Error(w, "Guild ID required", http.StatusBadRequest)
                return
            }

            isAdmin, err := authService.CheckGuildAdmin(r.Context(), userID, guildID)
            if err != nil || !isAdmin {
                http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
                return
            }

            // Add guild_id to context for handlers
            ctx := context.WithValue(r.Context(), "guild_id", guildID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func GuildMemberMiddleware(authService *authorization.GuildAuthService) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID, ok := r.Context().Value("user_id").(string)
            if !ok {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            guildID := chi.URLParam(r, "guild_id")
            if guildID == "" {
                http.Error(w, "Guild ID required", http.StatusBadRequest)
                return
            }

            isMember, err := authService.CheckGuildMember(r.Context(), userID, guildID)
            if err != nil || !isMember {
                http.Error(w, "Forbidden: Guild membership required", http.StatusForbidden)
                return
            }

            ctx := context.WithValue(r.Context(), "guild_id", guildID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**Apply to Routes**:

**File**: `backend/cmd/lambda/main.go`

```go
func setupRouter(authService *authorization.GuildAuthService) *chi.Mux {
    r := chi.NewRouter()

    // ... existing middleware ...

    // Guild admin endpoints
    r.Route("/api/guilds/{guild_id}", func(r chi.Router) {
        r.Use(middleware.AuthMiddleware)
        r.Use(middleware.GuildAdminMiddleware(authService))

        r.Put("/config", guildHandler.UpdateConfig)
        r.Post("/streamers", streamerHandler.LinkStreamer)
        r.Delete("/streamers/{streamer_id}", streamerHandler.UnlinkStreamer)
    })

    // User preference endpoints (member access)
    r.Route("/api/users/me/guilds/{guild_id}", func(r chi.Router) {
        r.Use(middleware.AuthMiddleware)
        r.Use(middleware.GuildMemberMiddleware(authService))

        r.Get("/preferences", preferencesHandler.GetPreferences)
        r.Put("/preferences/{streamer_id}", preferencesHandler.UpdatePreference)
    })

    return r
}
```

---

### Phase 2: Rate Limiting & Request Protection

#### 2.1 API Rate Limiting

**File**: `backend/internal/middleware/rate_limiter.go`

```go
package middleware

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rps      rate.Limit
    burst    int
}

func NewRateLimiter(requestsPerSecond int, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rps:      rate.Limit(requestsPerSecond),
        burst:    burst,
    }
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
    rl.mu.RLock()
    limiter, exists := rl.limiters[key]
    rl.mu.RUnlock()

    if !exists {
        rl.mu.Lock()
        defer rl.mu.Unlock()

        // Double-check after acquiring write lock
        limiter, exists = rl.limiters[key]
        if !exists {
            limiter = rate.NewLimiter(rl.rps, rl.burst)
            rl.limiters[key] = limiter

            // Cleanup old limiters periodically
            go func() {
                time.Sleep(10 * time.Minute)
                rl.mu.Lock()
                delete(rl.limiters, key)
                rl.mu.Unlock()
            }()
        }
    }

    return limiter
}

// Per-user rate limiting
func (rl *RateLimiter) UserRateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        userID, ok := r.Context().Value("user_id").(string)
        if !ok {
            userID = r.RemoteAddr // Fallback to IP for unauthenticated requests
        }

        limiter := rl.getLimiter(userID)

        if !limiter.Allow() {
            w.Header().Set("Retry-After", "60")
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// Global rate limiting
func (rl *RateLimiter) GlobalRateLimitMiddleware(next http.Handler) http.Handler {
    globalLimiter := rate.NewLimiter(rate.Limit(1000), 2000) // 1000 req/s globally

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !globalLimiter.Allow() {
            w.Header().Set("Retry-After", "1")
            http.Error(w, "System overloaded", http.StatusServiceUnavailable)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

#### 2.2 Webhook Rate Limiting & Idempotency

**File**: `backend/internal/middleware/webhook_protection.go`

```go
package middleware

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "net/http"
    "sync"
    "time"
)

type WebhookProtection struct {
    messageIDs  map[string]time.Time // message_id -> timestamp
    mu          sync.RWMutex
    rateLimiter *rate.Limiter
}

func NewWebhookProtection() *WebhookProtection {
    wp := &WebhookProtection{
        messageIDs:  make(map[string]time.Time),
        rateLimiter: rate.NewLimiter(rate.Limit(100), 200), // 100 webhooks/second
    }

    // Cleanup old message IDs every minute
    go wp.cleanupLoop()

    return wp
}

func (wp *WebhookProtection) WebhookMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Rate limiting
        if !wp.rateLimiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        // Idempotency check
        messageID := r.Header.Get("Twitch-Eventsub-Message-Id")
        if messageID != "" {
            if wp.isDuplicate(messageID) {
                // Already processed, return success
                w.WriteHeader(http.StatusOK)
                return
            }

            // Mark as processed
            wp.markProcessed(messageID)
        }

        next.ServeHTTP(w, r)
    })
}

func (wp *WebhookProtection) isDuplicate(messageID string) bool {
    wp.mu.RLock()
    defer wp.mu.RUnlock()

    _, exists := wp.messageIDs[messageID]
    return exists
}

func (wp *WebhookProtection) markProcessed(messageID string) {
    wp.mu.Lock()
    defer wp.mu.Unlock()

    wp.messageIDs[messageID] = time.Now()
}

func (wp *WebhookProtection) cleanupLoop() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        wp.mu.Lock()
        cutoff := time.Now().Add(-15 * time.Minute)
        for id, timestamp := range wp.messageIDs {
            if timestamp.Before(cutoff) {
                delete(wp.messageIDs, id)
            }
        }
        wp.mu.Unlock()
    }
}
```

---

### Phase 3: Secrets Management & Session Hardening

#### 3.1 AWS Secrets Manager Integration

**AWS Setup**:

```bash
# Create secret for JWT
aws secretsmanager create-secret \
  --name streammaxing/jwt-secret \
  --secret-string '{"jwt_secret":"'$(openssl rand -base64 32)'"}'

# Create secret for Discord OAuth
aws secretsmanager create-secret \
  --name streammaxing/discord-oauth \
  --secret-string '{
    "client_id":"YOUR_DISCORD_CLIENT_ID",
    "client_secret":"YOUR_DISCORD_CLIENT_SECRET"
  }'

# Create secret for Twitch OAuth
aws secretsmanager create-secret \
  --name streammaxing/twitch-oauth \
  --secret-string '{
    "client_id":"YOUR_TWITCH_CLIENT_ID",
    "client_secret":"YOUR_TWITCH_CLIENT_SECRET",
    "webhook_secret":"'$(openssl rand -base64 32)'"
  }'

# Grant Lambda access
aws secretsmanager put-resource-policy \
  --secret-id streammaxing/jwt-secret \
  --resource-policy '{
    "Version":"2012-10-17",
    "Statement":[{
      "Effect":"Allow",
      "Principal":{"AWS":"arn:aws:iam::ACCOUNT_ID:role/lambda-execution-role"},
      "Action":"secretsmanager:GetSecretValue",
      "Resource":"*"
    }]
  }'
```

**File**: `backend/internal/services/secrets/manager.go`

```go
package secrets

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretsManager struct {
    client *secretsmanager.Client
    cache  map[string]cachedSecret
    mu     sync.RWMutex
}

type cachedSecret struct {
    value     interface{}
    fetchedAt time.Time
}

type JWTSecret struct {
    JWTSecret string `json:"jwt_secret"`
}

type DiscordOAuth struct {
    ClientID     string `json:"client_id"`
    ClientSecret string `json:"client_secret"`
}

type TwitchOAuth struct {
    ClientID      string `json:"client_id"`
    ClientSecret  string `json:"client_secret"`
    WebhookSecret string `json:"webhook_secret"`
}

func NewSecretsManager() (*SecretsManager, error) {
    cfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        return nil, err
    }

    return &SecretsManager{
        client: secretsmanager.NewFromConfig(cfg),
        cache:  make(map[string]cachedSecret),
    }, nil
}

func (sm *SecretsManager) GetJWTSecret() (string, error) {
    var secret JWTSecret
    if err := sm.getSecret("streammaxing/jwt-secret", &secret); err != nil {
        return "", err
    }
    return secret.JWTSecret, nil
}

func (sm *SecretsManager) GetDiscordOAuth() (*DiscordOAuth, error) {
    var secret DiscordOAuth
    if err := sm.getSecret("streammaxing/discord-oauth", &secret); err != nil {
        return nil, err
    }
    return &secret, nil
}

func (sm *SecretsManager) GetTwitchOAuth() (*TwitchOAuth, error) {
    var secret TwitchOAuth
    if err := sm.getSecret("streammaxing/twitch-oauth", &secret); err != nil {
        return nil, err
    }
    return &secret, nil
}

func (sm *SecretsManager) getSecret(secretName string, target interface{}) error {
    // Check cache (TTL: 5 minutes)
    sm.mu.RLock()
    if cached, ok := sm.cache[secretName]; ok {
        if time.Since(cached.fetchedAt) < 5*time.Minute {
            // Return cached value
            sm.mu.RUnlock()

            // Copy cached value to target
            cachedJSON, _ := json.Marshal(cached.value)
            return json.Unmarshal(cachedJSON, target)
        }
    }
    sm.mu.RUnlock()

    // Fetch from Secrets Manager
    result, err := sm.client.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
        SecretId: &secretName,
    })
    if err != nil {
        return fmt.Errorf("failed to fetch secret %s: %w", secretName, err)
    }

    // Parse JSON
    if err := json.Unmarshal([]byte(*result.SecretString), target); err != nil {
        return fmt.Errorf("failed to parse secret %s: %w", secretName, err)
    }

    // Update cache
    sm.mu.Lock()
    sm.cache[secretName] = cachedSecret{
        value:     target,
        fetchedAt: time.Now(),
    }
    sm.mu.Unlock()

    return nil
}

// Invalidate cache (call after secret rotation)
func (sm *SecretsManager) InvalidateCache() {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.cache = make(map[string]cachedSecret)
}
```

**Update Auth Middleware**:

**File**: `backend/internal/middleware/auth.go`

```go
func AuthMiddleware(secretsManager *secrets.SecretsManager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            cookie, err := r.Cookie("session")
            if err != nil {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            // Get JWT secret from Secrets Manager
            jwtSecret, err := secretsManager.GetJWTSecret()
            if err != nil {
                http.Error(w, "Internal error", http.StatusInternalServerError)
                return
            }

            token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
                return []byte(jwtSecret), nil
            })

            if err != nil || !token.Valid {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            claims := token.Claims.(jwt.MapClaims)
            userID := claims["user_id"].(string)

            ctx := context.WithValue(r.Context(), "user_id", userID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

#### 3.2 Session Hardening

**File**: `backend/internal/services/auth/session.go`

```go
package auth

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "your-module/internal/services/secrets"
)

type SessionService struct {
    secretsManager *secrets.SecretsManager
    sessionStore   SessionStore // For revocation tracking
}

type SessionStore interface {
    InvalidateSession(ctx context.Context, jti string) error
    IsSessionValid(ctx context.Context, jti string) (bool, error)
}

type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    JTI      string `json:"jti"` // JWT ID for revocation
    jwt.RegisteredClaims
}

func NewSessionService(secretsManager *secrets.SecretsManager, sessionStore SessionStore) *SessionService {
    return &SessionService{
        secretsManager: secretsManager,
        sessionStore:   sessionStore,
    }
}

func (s *SessionService) CreateSession(userID, username string) (string, error) {
    jwtSecret, err := s.secretsManager.GetJWTSecret()
    if err != nil {
        return "", err
    }

    // Generate unique JWT ID
    jtiBytes := make([]byte, 16)
    rand.Read(jtiBytes)
    jti := hex.EncodeToString(jtiBytes)

    // Create claims with shorter expiration
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
    return token.SignedString([]byte(jwtSecret))
}

func (s *SessionService) ValidateSession(ctx context.Context, tokenString string) (*Claims, error) {
    jwtSecret, err := s.secretsManager.GetJWTSecret()
    if err != nil {
        return nil, err
    }

    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(jwtSecret), nil
    })

    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    // Check if session was revoked
    isValid, err := s.sessionStore.IsSessionValid(ctx, claims.JTI)
    if err != nil || !isValid {
        return nil, fmt.Errorf("session revoked")
    }

    return claims, nil
}

func (s *SessionService) RevokeSession(ctx context.Context, jti string) error {
    return s.sessionStore.InvalidateSession(ctx, jti)
}
```

**Session Store (Redis or Database)**:

**File**: `backend/internal/db/sessions.go`

```go
package db

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type SessionDB struct {
    pool *pgxpool.Pool
}

func NewSessionDB(pool *pgxpool.Pool) *SessionDB {
    return &SessionDB{pool: pool}
}

// Database schema for session revocation
// CREATE TABLE revoked_sessions (
//     jti TEXT PRIMARY KEY,
//     revoked_at TIMESTAMPTZ DEFAULT now(),
//     expires_at TIMESTAMPTZ NOT NULL
// );
// CREATE INDEX idx_revoked_sessions_expires ON revoked_sessions(expires_at);

func (db *SessionDB) InvalidateSession(ctx context.Context, jti string) error {
    query := `
        INSERT INTO revoked_sessions (jti, expires_at)
        VALUES ($1, now() + interval '7 days')
        ON CONFLICT (jti) DO NOTHING
    `
    _, err := db.pool.Exec(ctx, query, jti)
    return err
}

func (db *SessionDB) IsSessionValid(ctx context.Context, jti string) (bool, error) {
    query := `SELECT 1 FROM revoked_sessions WHERE jti = $1`

    var exists int
    err := db.pool.QueryRow(ctx, query, jti).Scan(&exists)

    if err == pgx.ErrNoRows {
        return true, nil // Not revoked
    }
    if err != nil {
        return false, err
    }

    return false, nil // Revoked
}

// Cleanup expired revoked sessions (run periodically)
func (db *SessionDB) CleanupExpiredSessions(ctx context.Context) error {
    query := `DELETE FROM revoked_sessions WHERE expires_at < now()`
    _, err := db.pool.Exec(ctx, query)
    return err
}
```

**Harden Cookie Configuration**:

```go
func setSessionCookie(w http.ResponseWriter, token string, isProd bool) {
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    token,
        Path:     "/",
        MaxAge:   86400, // 24 hours (reduced from 7 days)
        HttpOnly: true,
        Secure:   isProd, // HTTPS only in production
        SameSite: http.SameSiteStrictMode, // Strict CSRF protection
        Domain:   "", // Explicit empty to prevent subdomain access
    })
}
```

---

### Phase 4: Security Monitoring & Audit Logging

#### 4.1 Security Event Logging

**File**: `backend/internal/services/logging/security_logger.go`

```go
package logging

import (
    "context"
    "encoding/json"
    "log"
    "time"
)

type SecurityLogger struct {
    // Can integrate with CloudWatch Logs, Datadog, etc.
}

type SecurityEvent struct {
    Timestamp   time.Time              `json:"timestamp"`
    EventType   string                 `json:"event_type"`
    Severity    string                 `json:"severity"` // INFO, WARNING, CRITICAL
    UserID      string                 `json:"user_id,omitempty"`
    IPAddress   string                 `json:"ip_address"`
    Details     map[string]interface{} `json:"details"`
    Success     bool                   `json:"success"`
}

func NewSecurityLogger() *SecurityLogger {
    return &SecurityLogger{}
}

func (sl *SecurityLogger) LogEvent(ctx context.Context, event SecurityEvent) {
    event.Timestamp = time.Now()

    eventJSON, _ := json.Marshal(event)
    log.Printf("[SECURITY] %s", string(eventJSON))

    // Also send to CloudWatch Metrics for alerting
    if event.Severity == "CRITICAL" || !event.Success {
        // Publish metric for failed security events
        publishSecurityMetric(event.EventType, event.Success)
    }
}

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

func (sl *SecurityLogger) LogAuthSuccess(ctx context.Context, userID, ipAddress string) {
    sl.LogEvent(ctx, SecurityEvent{
        EventType: "auth_success",
        Severity:  "INFO",
        UserID:    userID,
        IPAddress: ipAddress,
        Success:   true,
        Details:   map[string]interface{}{},
    })
}

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

func (sl *SecurityLogger) LogWebhookSignatureFailure(ctx context.Context, ipAddress string) {
    sl.LogEvent(ctx, SecurityEvent{
        EventType: "webhook_signature_failure",
        Severity:  "CRITICAL",
        IPAddress: ipAddress,
        Success:   false,
        Details:   map[string]interface{}{},
    })
}

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
```

#### 4.2 CloudWatch Metrics & Alarms

**File**: `backend/internal/services/monitoring/cloudwatch.go`

```go
package monitoring

import (
    "context"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type CloudWatchMonitor struct {
    client *cloudwatch.Client
}

func NewCloudWatchMonitor() (*CloudWatchMonitor, error) {
    cfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        return nil, err
    }

    return &CloudWatchMonitor{
        client: cloudwatch.NewFromConfig(cfg),
    }, nil
}

func (m *CloudWatchMonitor) PublishSecurityMetric(eventType string, success bool) error {
    metricValue := 1.0
    if !success {
        metricValue = 0.0
    }

    _, err := m.client.PutMetricData(context.Background(), &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("StreamMaxing/Security"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String(eventType),
                Value:      aws.Float64(metricValue),
                Unit:       types.StandardUnitCount,
                Timestamp:  aws.Time(time.Now()),
                Dimensions: []types.Dimension{
                    {
                        Name:  aws.String("Environment"),
                        Value: aws.String("production"),
                    },
                },
            },
        },
    })

    return err
}

func (m *CloudWatchMonitor) PublishRateLimitMetric(exceeded bool) error {
    value := 0.0
    if exceeded {
        value = 1.0
    }

    _, err := m.client.PutMetricData(context.Background(), &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("StreamMaxing/RateLimit"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String("RateLimitExceeded"),
                Value:      aws.Float64(value),
                Unit:       types.StandardUnitCount,
                Timestamp:  aws.Time(time.Now()),
            },
        },
    })

    return err
}
```

**CloudWatch Alarms Setup**:

```bash
# Auth failures alarm
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-AuthFailures" \
  --alarm-description "Alert on high authentication failure rate" \
  --metric-name auth_failure \
  --namespace StreamMaxing/Security \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 50 \
  --comparison-operator GreaterThanThreshold \
  --treat-missing-data notBreaching

# Webhook signature failures
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-WebhookSignatureFailures" \
  --alarm-description "Alert on webhook signature validation failures" \
  --metric-name webhook_signature_failure \
  --namespace StreamMaxing/Security \
  --statistic Sum \
  --period 60 \
  --evaluation-periods 1 \
  --threshold 5 \
  --comparison-operator GreaterThanThreshold

# Rate limit exceeded
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-RateLimitExceeded" \
  --alarm-description "Alert on rate limit abuse" \
  --metric-name RateLimitExceeded \
  --namespace StreamMaxing/RateLimit \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 1 \
  --threshold 100 \
  --comparison-operator GreaterThanThreshold

# Permission denied anomalies
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-PermissionDenied" \
  --alarm-description "Alert on unusual permission denied events" \
  --metric-name permission_denied \
  --namespace StreamMaxing/Security \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 1 \
  --threshold 20 \
  --comparison-operator GreaterThanThreshold
```

---

### Phase 5: Input Validation & Additional Hardening

#### 5.1 Input Validation Library

**File**: `backend/internal/validation/validator.go`

```go
package validation

import (
    "fmt"
    "regexp"
    "strings"
)

type Validator struct{}

func NewValidator() *Validator {
    return &Validator{}
}

// ValidateGuildID checks Discord guild ID format
func (v *Validator) ValidateGuildID(guildID string) error {
    if len(guildID) < 17 || len(guildID) > 20 {
        return fmt.Errorf("invalid guild ID length")
    }
    if !regexp.MustCompile(`^\d+$`).MatchString(guildID) {
        return fmt.Errorf("guild ID must be numeric")
    }
    return nil
}

// ValidateUserID checks Discord user ID format
func (v *Validator) ValidateUserID(userID string) error {
    if len(userID) < 17 || len(userID) > 20 {
        return fmt.Errorf("invalid user ID length")
    }
    if !regexp.MustCompile(`^\d+$`).MatchString(userID) {
        return fmt.Errorf("user ID must be numeric")
    }
    return nil
}

// ValidateChannelID checks Discord channel ID format
func (v *Validator) ValidateChannelID(channelID string) error {
    if len(channelID) < 17 || len(channelID) > 20 {
        return fmt.Errorf("invalid channel ID length")
    }
    if !regexp.MustCompile(`^\d+$`).MatchString(channelID) {
        return fmt.Errorf("channel ID must be numeric")
    }
    return nil
}

// ValidateTemplateContent checks message template for injection attempts
func (v *Validator) ValidateTemplateContent(content string) error {
    // Max length
    if len(content) > 4000 {
        return fmt.Errorf("template content too long (max 4000 characters)")
    }

    // Check for script injection attempts
    dangerous := []string{"<script", "javascript:", "onerror=", "onclick="}
    contentLower := strings.ToLower(content)
    for _, pattern := range dangerous {
        if strings.Contains(contentLower, pattern) {
            return fmt.Errorf("template contains potentially dangerous content")
        }
    }

    return nil
}

// SanitizeInput removes potentially dangerous characters
func (v *Validator) SanitizeInput(input string) string {
    // Remove null bytes
    input = strings.ReplaceAll(input, "\x00", "")

    // Trim whitespace
    input = strings.TrimSpace(input)

    return input
}

// ValidateJSONSize checks if JSON payload is within acceptable size
func (v *Validator) ValidateJSONSize(data []byte) error {
    if len(data) > 1024*1024 { // 1MB max
        return fmt.Errorf("request payload too large")
    }
    return nil
}
```

**Apply Validation to Handlers**:

```go
func (h *GuildHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    var input struct {
        ChannelID       string          `json:"channel_id"`
        MentionRoleID   *string         `json:"mention_role_id"`
        MessageTemplate json.RawMessage `json:"message_template"`
    }

    body, _ := io.ReadAll(r.Body)

    // Validate size
    if err := h.validator.ValidateJSONSize(body); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := json.Unmarshal(body, &input); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Validate inputs
    if err := h.validator.ValidateChannelID(input.ChannelID); err != nil {
        http.Error(w, "Invalid channel ID", http.StatusBadRequest)
        return
    }

    // Sanitize and validate template
    templateStr := string(input.MessageTemplate)
    if err := h.validator.ValidateTemplateContent(templateStr); err != nil {
        http.Error(w, "Invalid template content", http.StatusBadRequest)
        return
    }

    // Continue with update...
}
```

---

## Database Migrations

**File**: `backend/migrations/007_security_hardening.sql`

```sql
-- Add session revocation table
CREATE TABLE IF NOT EXISTS revoked_sessions (
    jti TEXT PRIMARY KEY,
    revoked_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_revoked_sessions_expires ON revoked_sessions(expires_at);

-- Add audit log table
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ DEFAULT now(),
    user_id TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    details JSONB,
    ip_address TEXT,
    success BOOLEAN DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);

-- Add security events table
CREATE TABLE IF NOT EXISTS security_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ DEFAULT now(),
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    user_id TEXT,
    ip_address TEXT,
    details JSONB,
    resolved BOOLEAN DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_security_events_timestamp ON security_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_security_events_severity ON security_events(severity);
CREATE INDEX IF NOT EXISTS idx_security_events_unresolved ON security_events(resolved) WHERE resolved = false;

-- Add last_login tracking to users table (if not exists)
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_ip TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
```

---

## Environment Variables

Update `backend/.env`:

```bash
# AWS Services
KMS_KEY_ID=alias/streammaxing-oauth
AWS_REGION=us-east-1

# Remove these (now in Secrets Manager):
# JWT_SECRET=
# DISCORD_CLIENT_ID=
# DISCORD_CLIENT_SECRET=
# TWITCH_CLIENT_ID=
# TWITCH_CLIENT_SECRET=
# TWITCH_WEBHOOK_SECRET=

# Environment
ENVIRONMENT=production

# Feature flags
ENABLE_RATE_LIMITING=true
ENABLE_SECURITY_LOGGING=true
ENABLE_AUDIT_LOG=true
```

---

## Testing Checklist

### Security Testing

- [ ] **Token Encryption**
  - [ ] Twitch tokens encrypted before storage
  - [ ] Tokens decrypted correctly on retrieval
  - [ ] KMS key permissions verified
  - [ ] Migration script tested on copy of production data

- [ ] **Authorization**
  - [ ] Guild admin endpoints reject non-admin users
  - [ ] Guild member endpoints reject non-members
  - [ ] Permission changes reflected within 5 minutes
  - [ ] Cached permissions cleared on logout

- [ ] **Rate Limiting**
  - [ ] Per-user rate limiting works (50 req/min)
  - [ ] Global rate limiting works (1000 req/sec)
  - [ ] Webhook rate limiting works (100 req/sec)
  - [ ] Rate limit headers returned correctly

- [ ] **Session Security**
  - [ ] JWT expiration reduced to 24 hours
  - [ ] Session revocation works immediately
  - [ ] Logout invalidates session
  - [ ] Expired sessions rejected

- [ ] **Secrets Management**
  - [ ] All secrets loaded from Secrets Manager
  - [ ] Secrets cached properly (5 min TTL)
  - [ ] No secrets in environment variables
  - [ ] Secret rotation doesn't break system

- [ ] **Input Validation**
  - [ ] Invalid guild IDs rejected
  - [ ] Oversized payloads rejected
  - [ ] XSS attempts blocked in templates
  - [ ] SQL injection prevented (already tested)

- [ ] **Security Logging**
  - [ ] Failed auth attempts logged
  - [ ] Permission denials logged
  - [ ] Rate limit violations logged
  - [ ] Webhook signature failures logged

- [ ] **Monitoring**
  - [ ] CloudWatch metrics published
  - [ ] Alarms trigger correctly
  - [ ] Security events viewable in dashboard

### Penetration Testing

- [ ] Brute force attack blocked by rate limiting
- [ ] Token replay attack prevented
- [ ] CSRF attack prevented by SameSite cookies
- [ ] XSS attempt in template blocked
- [ ] Unauthorized guild access blocked
- [ ] Expired token rejected
- [ ] Revoked session rejected

---

## Deployment Procedure

### Pre-Deployment

1. **Backup Database**
   ```bash
   pg_dump $DATABASE_URL > backup_$(date +%Y%m%d).sql
   ```

2. **Create KMS Key**
   ```bash
   ./scripts/setup_kms.sh
   ```

3. **Migrate Secrets to Secrets Manager**
   ```bash
   ./scripts/migrate_secrets.sh
   ```

4. **Run Database Migrations**
   ```bash
   psql $DATABASE_URL -f migrations/007_security_hardening.sql
   ```

### Deployment

1. **Deploy Backend with New Dependencies**
   ```bash
   go mod tidy
   GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/lambda/main.go
   zip deployment.zip bootstrap
   aws lambda update-function-code --function-name streammaxing --zip-file fileb://deployment.zip
   ```

2. **Update Lambda Environment Variables**
   ```bash
   aws lambda update-function-configuration \
     --function-name streammaxing \
     --environment "Variables={
       KMS_KEY_ID=alias/streammaxing-oauth,
       AWS_REGION=us-east-1,
       ENVIRONMENT=production,
       DATABASE_URL=$DATABASE_URL,
       FRONTEND_URL=$FRONTEND_URL,
       ENABLE_RATE_LIMITING=true,
       ENABLE_SECURITY_LOGGING=true
     }"
   ```

3. **Grant Lambda IAM Permissions**
   ```bash
   # Attach policies for KMS and Secrets Manager
   aws iam attach-role-policy \
     --role-name lambda-execution-role \
     --policy-arn arn:aws:iam::aws:policy/SecretsManagerReadWrite

   aws iam put-role-policy \
     --role-name lambda-execution-role \
     --policy-name KMSAccess \
     --policy-document file://kms-policy.json
   ```

4. **Run Token Encryption Migration**
   ```bash
   go run migrations/007_encrypt_existing_tokens.go
   ```

5. **Setup CloudWatch Alarms**
   ```bash
   ./scripts/setup_cloudwatch_alarms.sh
   ```

### Post-Deployment

1. **Verify Security Features**
   - Test authentication with new session duration
   - Test rate limiting
   - Test guild authorization
   - Verify secrets loaded from Secrets Manager

2. **Monitor Logs**
   ```bash
   aws logs tail /aws/lambda/streammaxing --follow
   ```

3. **Check CloudWatch Metrics**
   - Verify security metrics publishing
   - Check for errors or anomalies

---

## Rollback Plan

If issues occur:

1. **Revert Lambda Code**
   ```bash
   aws lambda update-function-code \
     --function-name streammaxing \
     --s3-bucket lambda-deployments \
     --s3-key streammaxing-v006.zip
   ```

2. **Restore Database**
   ```bash
   psql $DATABASE_URL < backup_YYYYMMDD.sql
   ```

3. **Revert Secrets**
   - Environment variables can be temporarily used
   - Update Lambda configuration

---

## Monitoring & Maintenance

### Daily Checks

- Review CloudWatch alarms
- Check security event logs for anomalies
- Review failed authentication attempts

### Weekly Tasks

- Review audit logs for suspicious activity
- Check rate limit metrics
- Verify token encryption working correctly

### Monthly Tasks

- Rotate JWT secret in Secrets Manager
- Review and update CloudWatch alarms
- Perform security audit
- Review and clean up revoked sessions table
- Update KMS key policy if needed

---

## Performance Impact

Expected performance changes:

- **Token encryption**: +10-20ms per OAuth flow (acceptable)
- **Authorization checks**: +5-10ms per request (cached after first check)
- **Rate limiting**: <1ms overhead
- **Secrets Manager**: Cached, minimal impact after first fetch
- **Security logging**: Asynchronous, no user-facing impact

---

## Cost Impact

Additional monthly costs:

- **AWS KMS**: ~$1/key + $0.03 per 10,000 requests ≈ $2-5/month
- **Secrets Manager**: $0.40 per secret + $0.05 per 10,000 API calls ≈ $2-3/month
- **CloudWatch Logs** (security events): ~$1-2/month with log retention
- **CloudWatch Alarms**: $0.10 per alarm × 4 = $0.40/month
- **Database** (new tables): Minimal (<100MB)

**Total additional cost**: ~$5-10/month

---

## Next Steps

After completing this task:

1. Conduct security audit with external tool (e.g., OWASP ZAP)
2. Implement Web Application Firewall (WAF) on API Gateway
3. Add DDoS protection using AWS Shield
4. Implement automated security testing in CI/CD
5. Create security incident response playbook
6. Schedule regular penetration testing

---

## Notes

- **KMS encryption** is the industry standard for sensitive data at rest
- **Secrets Manager** enables automatic secret rotation
- **Rate limiting** prevents abuse and reduces costs
- **Session revocation** is critical for security in case of compromise
- **Security logging** enables forensics and anomaly detection
- **Short JWT expiration** reduces attack window
- **Strict authorization checks** prevent privilege escalation
- All changes are **backward compatible** (except JWT expiration change)
- **No breaking changes** to frontend API
- Performance impact is minimal and acceptable

---

## References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [AWS KMS Best Practices](https://docs.aws.amazon.com/kms/latest/developerguide/best-practices.html)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [Discord OAuth Security](https://discord.com/developers/docs/topics/oauth2#security-considerations)
- [Twitch EventSub Security](https://dev.twitch.tv/docs/eventsub/handling-webhook-events#verifying-the-event-message)
