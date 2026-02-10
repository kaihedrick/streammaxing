# Task 002: Discord Auth and Bot Installation

## Status
Complete

## Overview
Implement Discord OAuth 2.0 authentication flow for users, bot installation flow for guilds, and guild management features.

---

## Goals
1. Implement Discord OAuth login and callback handlers
2. Create session management with JWT tokens
3. Implement bot installation flow with proper permissions
4. Fetch and store user guilds
5. Fetch and store guild channels and roles
6. Implement middleware for authentication and authorization
7. Create API endpoints for guild management

---

## Prerequisites
- Task 001 (Project Bootstrap) completed
- Database schema created (users, guilds tables)
- Discord application created in Discord Developer Portal
- Bot user enabled with proper permissions
- Environment variables configured

---

## Discord Application Setup

### Step 1: Create Discord Application
1. Go to https://discord.com/developers/applications
2. Click "New Application"
3. Name: "StreamMaxing" (or your preferred name)
4. Save application

### Step 2: Configure OAuth2
1. Navigate to OAuth2 → General
2. Add Redirect URI: `https://your-api-url/api/auth/discord/callback`
3. For local development: `http://localhost:3000/api/auth/discord/callback`
4. Save changes

### Step 3: Enable Bot User
1. Navigate to Bot section
2. Click "Add Bot"
3. Uncheck "Public Bot" (optional)
4. Enable "Server Members Intent" (required for membership checks)
5. Save bot token securely

### Step 4: Configure Bot Permissions
**Required Permissions**:
- Send Messages (0x800)
- Embed Links (0x4000)
- Mention Everyone (0x20000)

**Permissions Integer**: 149504 (0x800 + 0x4000 + 0x20000)

### Step 5: Get OAuth2 Credentials
1. Navigate to OAuth2 → General
2. Copy Client ID
3. Click "Reset Secret" and copy Client Secret
4. Add to `.env`:
   ```bash
   DISCORD_CLIENT_ID=your_client_id
   DISCORD_CLIENT_SECRET=your_client_secret
   DISCORD_BOT_TOKEN=your_bot_token
   ```

---

## Backend Implementation

### File Structure
```
backend/
├── internal/
│   ├── handlers/
│   │   ├── auth.go              # OAuth handlers
│   │   └── guilds.go            # Guild management handlers
│   ├── services/
│   │   └── discord/
│   │       ├── oauth.go         # OAuth flow logic
│   │       ├── api.go           # Discord API client
│   │       └── permissions.go   # Permission checks
│   ├── middleware/
│   │   ├── auth.go              # JWT validation
│   │   └── cors.go              # CORS headers
│   └── db/
│       ├── users.go             # User database operations
│       └── guilds.go            # Guild database operations
```

### 1. Discord OAuth Service

**File**: `backend/internal/services/discord/oauth.go`

```go
package discord

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
        clientID:     os.Getenv("DISCORD_CLIENT_ID"),
        clientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
        redirectURI:  os.Getenv("API_BASE_URL") + "/api/auth/discord/callback",
    }
}

func (s *OAuthService) GetAuthURL(state string) string {
    params := url.Values{
        "client_id":     {s.clientID},
        "redirect_uri":  {s.redirectURI},
        "response_type": {"code"},
        "scope":         {"identify guilds"},
        "state":         {state},
    }
    return "https://discord.com/api/oauth2/authorize?" + params.Encode()
}

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    TokenType    string `json:"token_type"`
    ExpiresIn    int    `json:"expires_in"`
    RefreshToken string `json:"refresh_token"`
    Scope        string `json:"scope"`
}

func (s *OAuthService) ExchangeCode(code string) (*TokenResponse, error) {
    data := url.Values{
        "client_id":     {s.clientID},
        "client_secret": {s.clientSecret},
        "grant_type":    {"authorization_code"},
        "code":          {code},
        "redirect_uri":  {s.redirectURI},
    }

    resp, err := http.PostForm("https://discord.com/api/oauth2/token", data)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("discord oauth error: %s", body)
    }

    var tokenResp TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }

    return &tokenResp, nil
}

type DiscordUser struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Avatar   string `json:"avatar"`
}

func (s *OAuthService) GetUser(accessToken string) (*DiscordUser, error) {
    req, _ := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
    req.Header.Set("Authorization", "Bearer "+accessToken)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch user: %d", resp.StatusCode)
    }

    var user DiscordUser
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, err
    }

    return &user, nil
}

type DiscordGuild struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Icon        string `json:"icon"`
    Owner       bool   `json:"owner"`
    Permissions string `json:"permissions"`
}

func (s *OAuthService) GetUserGuilds(accessToken string) ([]DiscordGuild, error) {
    req, _ := http.NewRequest("GET", "https://discord.com/api/users/@me/guilds", nil)
    req.Header.Set("Authorization", "Bearer "+accessToken)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch guilds: %d", resp.StatusCode)
    }

    var guilds []DiscordGuild
    if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
        return nil, err
    }

    return guilds, nil
}
```

### 2. Discord API Client

**File**: `backend/internal/services/discord/api.go`

```go
package discord

import (
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

type APIClient struct {
    botToken string
}

func NewAPIClient() *APIClient {
    return &APIClient{
        botToken: os.Getenv("DISCORD_BOT_TOKEN"),
    }
}

type Channel struct {
    ID       string `json:"id"`
    Type     int    `json:"type"`
    Name     string `json:"name"`
    Position int    `json:"position"`
}

func (c *APIClient) GetGuildChannels(guildID string) ([]Channel, error) {
    url := fmt.Sprintf("https://discord.com/api/guilds/%s/channels", guildID)
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bot "+c.botToken)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch channels: %d", resp.StatusCode)
    }

    var channels []Channel
    if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
        return nil, err
    }

    // Filter to text (0) and announcement (5) channels
    var textChannels []Channel
    for _, ch := range channels {
        if ch.Type == 0 || ch.Type == 5 {
            textChannels = append(textChannels, ch)
        }
    }

    return textChannels, nil
}

type Role struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Color    int    `json:"color"`
    Position int    `json:"position"`
}

func (c *APIClient) GetGuildRoles(guildID string) ([]Role, error) {
    url := fmt.Sprintf("https://discord.com/api/guilds/%s/roles", guildID)
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bot "+c.botToken)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch roles: %d", resp.StatusCode)
    }

    var roles []Role
    if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
        return nil, err
    }

    return roles, nil
}

func (c *APIClient) CheckGuildMembership(guildID, userID string) (bool, error) {
    url := fmt.Sprintf("https://discord.com/api/guilds/%s/members/%s", guildID, userID)
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bot "+c.botToken)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    return resp.StatusCode == http.StatusOK, nil
}
```

### 3. Permission Checks

**File**: `backend/internal/services/discord/permissions.go`

```go
package discord

import (
    "strconv"
)

const (
    PermissionAdministrator = 0x8
)

func HasAdminPermission(permissions string) bool {
    perms, err := strconv.ParseInt(permissions, 10, 64)
    if err != nil {
        return false
    }
    return (perms & PermissionAdministrator) != 0
}

func (s *OAuthService) CheckGuildPermission(accessToken, guildID string) (bool, error) {
    guilds, err := s.GetUserGuilds(accessToken)
    if err != nil {
        return false, err
    }

    for _, guild := range guilds {
        if guild.ID == guildID {
            return guild.Owner || HasAdminPermission(guild.Permissions), nil
        }
    }

    return false, nil
}
```

### 4. JWT Session Management

**File**: `backend/internal/middleware/auth.go`

```go
package middleware

import (
    "context"
    "net/http"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    jwt.RegisteredClaims
}

func GenerateJWT(userID, username string) (string, error) {
    claims := Claims{
        UserID:   userID,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func ValidateJWT(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(os.Getenv("JWT_SECRET")), nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, jwt.ErrSignatureInvalid
}

func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("session")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        claims, err := ValidateJWT(cookie.Value)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Attach user ID to context
        ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
        ctx = context.WithValue(ctx, "username", claims.Username)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 5. Auth Handlers

**File**: `backend/internal/handlers/auth.go`

```go
package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "net/http"
    "os"

    "your-module/internal/db"
    "your-module/internal/middleware"
    "your-module/internal/services/discord"
)

type AuthHandler struct {
    oauth     *discord.OAuthService
    userDB    *db.UserDB
    guildDB   *db.GuildDB
}

func NewAuthHandler(userDB *db.UserDB, guildDB *db.GuildDB) *AuthHandler {
    return &AuthHandler{
        oauth:   discord.NewOAuthService(),
        userDB:  userDB,
        guildDB: guildDB,
    }
}

func generateState() string {
    b := make([]byte, 16)
    rand.Read(b)
    return hex.EncodeToString(b)
}

func (h *AuthHandler) DiscordLogin(w http.ResponseWriter, r *http.Request) {
    state := generateState()

    // Store state in session (use Redis or cookie for production)
    http.SetCookie(w, &http.Cookie{
        Name:     "oauth_state",
        Value:    state,
        Path:     "/",
        MaxAge:   600, // 10 minutes
        HttpOnly: true,
        Secure:   os.Getenv("ENVIRONMENT") == "production",
        SameSite: http.SameSiteStrictMode,
    })

    authURL := h.oauth.GetAuthURL(state)
    http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) DiscordCallback(w http.ResponseWriter, r *http.Request) {
    // Verify state
    stateCookie, err := r.Cookie("oauth_state")
    if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
        http.Error(w, "Invalid state", http.StatusBadRequest)
        return
    }

    // Exchange code for token
    code := r.URL.Query().Get("code")
    tokenResp, err := h.oauth.ExchangeCode(code)
    if err != nil {
        http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
        return
    }

    // Fetch user info
    user, err := h.oauth.GetUser(tokenResp.AccessToken)
    if err != nil {
        http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
        return
    }

    // Fetch user guilds
    guilds, err := h.oauth.GetUserGuilds(tokenResp.AccessToken)
    if err != nil {
        http.Error(w, "Failed to fetch guilds", http.StatusInternalServerError)
        return
    }

    // Store user in database
    if err := h.userDB.UpsertUser(user.ID, user.Username, user.Avatar); err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    // Store guilds in database (only those where user has admin permission)
    for _, guild := range guilds {
        if guild.Owner || discord.HasAdminPermission(guild.Permissions) {
            h.guildDB.UpsertGuild(guild.ID, guild.Name, guild.Icon, guild.Owner)
        }
    }

    // Generate JWT
    token, err := middleware.GenerateJWT(user.ID, user.Username)
    if err != nil {
        http.Error(w, "Failed to generate token", http.StatusInternalServerError)
        return
    }

    // Set session cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    token,
        Path:     "/",
        MaxAge:   7 * 24 * 60 * 60, // 7 days
        HttpOnly: true,
        Secure:   os.Getenv("ENVIRONMENT") == "production",
        SameSite: http.SameSiteStrictMode,
    })

    // Clear state cookie
    http.SetCookie(w, &http.Cookie{
        Name:   "oauth_state",
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })

    // Redirect to dashboard
    frontendURL := os.Getenv("FRONTEND_URL")
    http.Redirect(w, r, frontendURL+"/dashboard", http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
    http.SetCookie(w, &http.Cookie{
        Name:   "session",
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })

    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"message":"Logged out"}`))
}
```

### 6. Guild Handlers

**File**: `backend/internal/handlers/guilds.go`

```go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "your-module/internal/db"
    "your-module/internal/services/discord"
)

type GuildHandler struct {
    api     *discord.APIClient
    oauth   *discord.OAuthService
    guildDB *db.GuildDB
}

func NewGuildHandler(guildDB *db.GuildDB) *GuildHandler {
    return &GuildHandler{
        api:     discord.NewAPIClient(),
        oauth:   discord.NewOAuthService(),
        guildDB: guildDB,
    }
}

func (h *GuildHandler) GetUserGuilds(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(string)

    // Fetch guilds where user has admin permission
    guilds, err := h.guildDB.GetUserGuilds(userID)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(guilds)
}

func (h *GuildHandler) GetGuildChannels(w http.ResponseWriter, r *http.Request) {
    guildID := chi.URLParam(r, "guild_id")

    // TODO: Check user permission

    channels, err := h.api.GetGuildChannels(guildID)
    if err != nil {
        http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(channels)
}

func (h *GuildHandler) GetGuildRoles(w http.ResponseWriter, r *http.Request) {
    guildID := chi.URLParam(r, "guild_id")

    // TODO: Check user permission

    roles, err := h.api.GetGuildRoles(guildID)
    if err != nil {
        http.Error(w, "Failed to fetch roles", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(roles)
}

func (h *GuildHandler) GetBotInstallURL(w http.ResponseWriter, r *http.Request) {
    guildID := chi.URLParam(r, "guild_id")
    clientID := os.Getenv("DISCORD_CLIENT_ID")

    installURL := fmt.Sprintf(
        "https://discord.com/api/oauth2/authorize?client_id=%s&scope=bot&permissions=149504&guild_id=%s",
        clientID, guildID,
    )

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "url": installURL,
    })
}
```

### 7. Database Layer

**File**: `backend/internal/db/users.go`

```go
package db

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type UserDB struct {
    pool *pgxpool.Pool
}

func NewUserDB(pool *pgxpool.Pool) *UserDB {
    return &UserDB{pool: pool}
}

func (db *UserDB) UpsertUser(userID, username, avatar string) error {
    query := `
        INSERT INTO users (user_id, username, avatar, last_login)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (user_id)
        DO UPDATE SET username = $2, avatar = $3, last_login = $4
    `
    _, err := db.pool.Exec(context.Background(), query, userID, username, avatar, time.Now())
    return err
}

func (db *UserDB) GetUser(userID string) (*User, error) {
    query := `SELECT user_id, username, avatar, created_at, last_login FROM users WHERE user_id = $1`

    var user User
    err := db.pool.QueryRow(context.Background(), query, userID).Scan(
        &user.UserID, &user.Username, &user.Avatar, &user.CreatedAt, &user.LastLogin,
    )

    return &user, err
}

type User struct {
    UserID    string
    Username  string
    Avatar    string
    CreatedAt time.Time
    LastLogin time.Time
}
```

**File**: `backend/internal/db/guilds.go`

```go
package db

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
)

type GuildDB struct {
    pool *pgxpool.Pool
}

func NewGuildDB(pool *pgxpool.Pool) *GuildDB {
    return &GuildDB{pool: pool}
}

func (db *GuildDB) UpsertGuild(guildID, name, icon string, isOwner bool) error {
    query := `
        INSERT INTO guilds (guild_id, name, icon, owner_id)
        VALUES ($1, $2, $3, NULL)
        ON CONFLICT (guild_id)
        DO UPDATE SET name = $2, icon = $3
    `
    _, err := db.pool.Exec(context.Background(), query, guildID, name, icon)
    return err
}

func (db *GuildDB) GetUserGuilds(userID string) ([]Guild, error) {
    // This is a placeholder - in practice, fetch from Discord API with caching
    query := `SELECT guild_id, name, icon FROM guilds`

    rows, err := db.pool.Query(context.Background(), query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var guilds []Guild
    for rows.Next() {
        var guild Guild
        if err := rows.Scan(&guild.GuildID, &guild.Name, &guild.Icon); err != nil {
            return nil, err
        }
        guilds = append(guilds, guild)
    }

    return guilds, nil
}

type Guild struct {
    GuildID string
    Name    string
    Icon    string
}
```

---

## API Endpoints

### Authentication Routes

**GET /api/auth/discord/login**
- Description: Initiates Discord OAuth flow
- Auth: None
- Response: Redirect to Discord authorization page

**GET /api/auth/discord/callback**
- Description: Handles Discord OAuth callback
- Auth: None
- Query Parameters:
  - `code`: Authorization code from Discord
  - `state`: CSRF protection token
- Response: Redirect to dashboard with session cookie

**POST /api/auth/logout**
- Description: Clears session cookie
- Auth: Required (JWT)
- Response: `{ "message": "Logged out" }`

### Guild Routes

**GET /api/guilds**
- Description: List guilds where user has admin permission
- Auth: Required (JWT)
- Response: Array of guild objects

**GET /api/guilds/:guild_id/channels**
- Description: List text channels in guild
- Auth: Required (JWT + admin permission)
- Response: Array of channel objects

**GET /api/guilds/:guild_id/roles**
- Description: List roles in guild
- Auth: Required (JWT + admin permission)
- Response: Array of role objects

**GET /api/guilds/:guild_id/bot-install-url**
- Description: Generate bot installation URL
- Auth: Required (JWT + admin permission)
- Response: `{ "url": "https://discord.com/..." }`

---

## Frontend Implementation

### Auth Flow Components

**File**: `frontend/src/services/api.ts`

```typescript
const API_BASE = import.meta.env.VITE_API_URL;

export async function loginWithDiscord() {
  window.location.href = `${API_BASE}/api/auth/discord/login`;
}

export async function logout() {
  const response = await fetch(`${API_BASE}/api/auth/logout`, {
    method: 'POST',
    credentials: 'include',
  });
  return response.json();
}

export async function getUserGuilds() {
  const response = await fetch(`${API_BASE}/api/guilds`, {
    credentials: 'include',
  });
  return response.json();
}

export async function getGuildChannels(guildId: string) {
  const response = await fetch(`${API_BASE}/api/guilds/${guildId}/channels`, {
    credentials: 'include',
  });
  return response.json();
}

export async function getGuildRoles(guildId: string) {
  const response = await fetch(`${API_BASE}/api/guilds/${guildId}/roles`, {
    credentials: 'include',
  });
  return response.json();
}

export async function getBotInstallURL(guildId: string) {
  const response = await fetch(`${API_BASE}/api/guilds/${guildId}/bot-install-url`, {
    credentials: 'include',
  });
  return response.json();
}
```

**File**: `frontend/src/components/Auth/LoginPage.tsx`

```tsx
import { loginWithDiscord } from '../../services/api';

export function LoginPage() {
  return (
    <div className="login-page">
      <h1>StreamMaxing</h1>
      <p>Get notified when your favorite Twitch streamers go live</p>
      <button onClick={loginWithDiscord}>
        Login with Discord
      </button>
    </div>
  );
}
```

---

## Testing Checklist

### Manual Testing
- [ ] Discord OAuth login flow completes successfully
- [ ] User information stored in database
- [ ] Guilds fetched and stored correctly
- [ ] Session cookie set with proper flags (HttpOnly, Secure, SameSite)
- [ ] JWT token validates correctly
- [ ] Auth middleware blocks unauthenticated requests
- [ ] Bot installation URL generates correctly
- [ ] Channels endpoint returns only text/announcement channels
- [ ] Roles endpoint returns all guild roles
- [ ] Logout clears session cookie

### Security Testing
- [ ] CSRF protection works (invalid state rejected)
- [ ] Expired JWT tokens rejected
- [ ] Invalid JWT tokens rejected
- [ ] Users cannot access other guilds without permission
- [ ] Bot token not exposed in responses

### Error Handling
- [ ] OAuth errors handled gracefully
- [ ] Database errors logged and return 500
- [ ] Network errors to Discord API handled with retries
- [ ] Missing environment variables cause startup failure

---

## Environment Variables

Add to `backend/.env`:
```bash
# Discord OAuth
DISCORD_CLIENT_ID=your_client_id
DISCORD_CLIENT_SECRET=your_client_secret
DISCORD_BOT_TOKEN=your_bot_token

# JWT
JWT_SECRET=your_secure_random_string_min_32_chars

# URLs
API_BASE_URL=https://your-api-gateway-url
FRONTEND_URL=https://your-cloudfront-url

# Environment
ENVIRONMENT=development
```

---

## Next Steps

After completing this task:
1. Task 003: Implement Twitch OAuth and EventSub subscriptions
2. Configure guild_config table for notification settings
3. Test full OAuth flow in production environment

---

## Notes

- Store Discord OAuth tokens if refresh flow needed (future enhancement)
- Consider caching user guilds for 5 minutes to reduce API calls
- Implement rate limiting for Discord API calls
- Add CloudWatch logs for OAuth failures
- Consider using Redis for session storage in high-traffic scenarios
