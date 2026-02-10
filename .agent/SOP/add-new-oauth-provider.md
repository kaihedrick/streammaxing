# SOP: Add New OAuth Provider

## Overview
This document describes how to add a new OAuth 2.0 provider to StreamMaxing (e.g., YouTube, Kick, Twitter).

---

## Prerequisites
- OAuth application created with the provider
- Client ID and Client Secret obtained
- Redirect URI configured with provider
- Understanding of OAuth 2.0 authorization code flow

---

## Steps

### 1. Create OAuth Application

Each provider has different setup:

**Example: YouTube/Google**
1. Go to Google Cloud Console
2. Create new project
3. Enable YouTube Data API v3
4. Create OAuth 2.0 credentials
5. Add authorized redirect URI: `https://your-api-url/api/auth/youtube/callback`
6. Copy Client ID and Client Secret

---

### 2. Add Environment Variables

**File**: `backend/.env`

```bash
# YouTube OAuth
YOUTUBE_CLIENT_ID=your_client_id
YOUTUBE_CLIENT_SECRET=your_client_secret
```

---

### 3. Create OAuth Service

**File**: `backend/internal/services/youtube/oauth.go`

```go
package youtube

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
        clientID:     os.Getenv("YOUTUBE_CLIENT_ID"),
        clientSecret: os.Getenv("YOUTUBE_CLIENT_SECRET"),
        redirectURI:  os.Getenv("API_BASE_URL") + "/api/auth/youtube/callback",
    }
}

// GetAuthURL generates the authorization URL
func (s *OAuthService) GetAuthURL(state string) string {
    params := url.Values{
        "client_id":     {s.clientID},
        "redirect_uri":  {s.redirectURI},
        "response_type": {"code"},
        "scope":         {"https://www.googleapis.com/auth/youtube.readonly"},
        "state":         {state},
        "access_type":   {"offline"}, // To get refresh token
        "prompt":        {"consent"},  // Force consent screen
    }
    return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`
    TokenType    string `json:"token_type"`
    Scope        string `json:"scope"`
}

// ExchangeCode exchanges authorization code for access token
func (s *OAuthService) ExchangeCode(code string) (*TokenResponse, error) {
    data := url.Values{
        "client_id":     {s.clientID},
        "client_secret": {s.clientSecret},
        "code":          {code},
        "grant_type":    {"authorization_code"},
        "redirect_uri":  {s.redirectURI},
    }

    resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("oauth error: %s", body)
    }

    var tokenResp TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }

    return &tokenResp, nil
}

type YouTubeChannel struct {
    ID          string `json:"id"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Thumbnail   string `json:"thumbnail"`
}

// GetChannel fetches authenticated user's YouTube channel
func (s *OAuthService) GetChannel(accessToken string) (*YouTubeChannel, error) {
    req, _ := http.NewRequest(
        "GET",
        "https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true",
        nil,
    )
    req.Header.Set("Authorization", "Bearer "+accessToken)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch channel: %d", resp.StatusCode)
    }

    var result struct {
        Items []struct {
            ID      string `json:"id"`
            Snippet struct {
                Title       string `json:"title"`
                Description string `json:"description"`
                Thumbnails  struct {
                    Default struct {
                        URL string `json:"url"`
                    } `json:"default"`
                } `json:"thumbnails"`
            } `json:"snippet"`
        } `json:"items"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    if len(result.Items) == 0 {
        return nil, fmt.Errorf("no channel found")
    }

    item := result.Items[0]
    return &YouTubeChannel{
        ID:          item.ID,
        Title:       item.Snippet.Title,
        Description: item.Snippet.Description,
        Thumbnail:   item.Snippet.Thumbnails.Default.URL,
    }, nil
}

// RefreshToken refreshes an expired access token
func (s *OAuthService) RefreshToken(refreshToken string) (*TokenResponse, error) {
    data := url.Values{
        "client_id":     {s.clientID},
        "client_secret": {s.clientSecret},
        "refresh_token": {refreshToken},
        "grant_type":    {"refresh_token"},
    }

    resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("refresh failed: %d", resp.StatusCode)
    }

    var tokenResp TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }

    return &tokenResp, nil
}
```

---

### 4. Create Auth Handler

**File**: `backend/internal/handlers/youtube_auth.go`

```go
package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "net/http"
    "os"

    "your-module/internal/db"
    "your-module/internal/services/youtube"
)

type YouTubeAuthHandler struct {
    oauth      *youtube.OAuthService
    channelDB  *db.YouTubeChannelDB
}

func NewYouTubeAuthHandler(channelDB *db.YouTubeChannelDB) *YouTubeAuthHandler {
    return &YouTubeAuthHandler{
        oauth:     youtube.NewOAuthService(),
        channelDB: channelDB,
    }
}

func (h *YouTubeAuthHandler) InitiateAuth(w http.ResponseWriter, r *http.Request) {
    guildID := r.URL.Query().Get("guild_id")

    // Generate state with guild_id
    randomBytes := make([]byte, 16)
    rand.Read(randomBytes)
    state := fmt.Sprintf("guild_id:%s:%s", guildID, hex.EncodeToString(randomBytes))

    // Store state in session
    http.SetCookie(w, &http.Cookie{
        Name:     "youtube_oauth_state",
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

func (h *YouTubeAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
    // Verify state
    stateCookie, err := r.Cookie("youtube_oauth_state")
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

    // Fetch channel info
    channel, err := h.oauth.GetChannel(tokenResp.AccessToken)
    if err != nil {
        http.Error(w, "Failed to fetch channel", http.StatusInternalServerError)
        return
    }

    // Store channel in database
    channelID, err := h.channelDB.UpsertChannel(
        channel.ID,
        channel.Title,
        channel.Description,
        channel.Thumbnail,
        tokenResp.AccessToken,
        tokenResp.RefreshToken,
    )
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    // Link channel to guild
    userID := r.Context().Value("user_id").(string)
    if err := h.channelDB.LinkChannelToGuild(guildID, channelID, userID); err != nil {
        http.Error(w, "Failed to link channel", http.StatusInternalServerError)
        return
    }

    // Clear state cookie
    http.SetCookie(w, &http.Cookie{
        Name:   "youtube_oauth_state",
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })

    // Redirect to dashboard
    frontendURL := os.Getenv("FRONTEND_URL")
    http.Redirect(w, r, fmt.Sprintf("%s/dashboard/guilds/%s", frontendURL, guildID), http.StatusTemporaryRedirect)
}
```

---

### 5. Create Database Schema

**File**: `backend/migrations/00X_add_youtube_channels.sql`

```sql
BEGIN;

-- Create YouTube channels table
CREATE TABLE IF NOT EXISTS youtube_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    youtube_channel_id TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    thumbnail_url TEXT,
    access_token TEXT,  -- Consider encrypting
    refresh_token TEXT, -- Consider encrypting
    created_at TIMESTAMPTZ DEFAULT now(),
    last_updated TIMESTAMPTZ DEFAULT now()
);

-- Create junction table for guild-channel relationship
CREATE TABLE IF NOT EXISTS guild_youtube_channels (
    guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
    channel_id UUID REFERENCES youtube_channels(id) ON DELETE CASCADE,
    enabled BOOLEAN DEFAULT true,
    added_by TEXT REFERENCES users(user_id),
    added_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (guild_id, channel_id)
);

-- Add indexes
CREATE INDEX idx_guild_youtube_channels_guild ON guild_youtube_channels(guild_id);
CREATE INDEX idx_guild_youtube_channels_channel ON guild_youtube_channels(channel_id);

COMMIT;
```

---

### 6. Create Database Layer

**File**: `backend/internal/db/youtube_channels.go`

```go
package db

import (
    "context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type YouTubeChannelDB struct {
    pool *pgxpool.Pool
}

func NewYouTubeChannelDB(pool *pgxpool.Pool) *YouTubeChannelDB {
    return &YouTubeChannelDB{pool: pool}
}

func (db *YouTubeChannelDB) UpsertChannel(
    channelID, title, description, thumbnailURL, accessToken, refreshToken string,
) (uuid.UUID, error) {
    query := `
        INSERT INTO youtube_channels (
            youtube_channel_id, title, description, thumbnail_url,
            access_token, refresh_token
        )
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (youtube_channel_id)
        DO UPDATE SET
            title = $2,
            description = $3,
            thumbnail_url = $4,
            access_token = $5,
            refresh_token = $6,
            last_updated = now()
        RETURNING id
    `

    var id uuid.UUID
    err := db.pool.QueryRow(
        context.Background(),
        query,
        channelID, title, description, thumbnailURL, accessToken, refreshToken,
    ).Scan(&id)

    return id, err
}

func (db *YouTubeChannelDB) LinkChannelToGuild(guildID string, channelID uuid.UUID, addedBy string) error {
    query := `
        INSERT INTO guild_youtube_channels (guild_id, channel_id, enabled, added_by)
        VALUES ($1, $2, true, $3)
        ON CONFLICT (guild_id, channel_id) DO NOTHING
    `
    _, err := db.pool.Exec(context.Background(), query, guildID, channelID, addedBy)
    return err
}

func (db *YouTubeChannelDB) GetGuildChannels(guildID string) ([]YouTubeChannel, error) {
    query := `
        SELECT c.id, c.youtube_channel_id, c.title, c.description, c.thumbnail_url
        FROM youtube_channels c
        JOIN guild_youtube_channels gc ON c.id = gc.channel_id
        WHERE gc.guild_id = $1 AND gc.enabled = true
    `

    rows, err := db.pool.Query(context.Background(), query, guildID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var channels []YouTubeChannel
    for rows.Next() {
        var c YouTubeChannel
        if err := rows.Scan(&c.ID, &c.ChannelID, &c.Title, &c.Description, &c.ThumbnailURL); err != nil {
            return nil, err
        }
        channels = append(channels, c)
    }

    return channels, nil
}

type YouTubeChannel struct {
    ID           uuid.UUID
    ChannelID    string
    Title        string
    Description  string
    ThumbnailURL string
}
```

---

### 7. Add Routes

**File**: `backend/cmd/lambda/main.go`

```go
func setupRouter() *chi.Mux {
    r := chi.NewRouter()

    // ... existing routes ...

    // YouTube OAuth routes
    r.Get("/api/auth/youtube/initiate", youtubeAuthHandler.InitiateAuth)
    r.Get("/api/auth/youtube/callback", youtubeAuthHandler.Callback)

    // YouTube channel management routes
    r.Route("/api/guilds/{guild_id}/youtube-channels", func(r chi.Router) {
        r.Use(middleware.AuthMiddleware)
        r.Get("/", youtubeHandler.List)
        r.Delete("/{channel_id}", youtubeHandler.Unlink)
    })

    return r
}
```

---

### 8. Add Frontend API Client

**File**: `frontend/src/services/api.ts`

```typescript
export async function initiateYouTubeAuth(guildId: string): Promise<{ url: string }> {
  return fetchAPI(`/api/auth/youtube/initiate?guild_id=${guildId}`);
}

export async function getYouTubeChannels(guildId: string): Promise<YouTubeChannel[]> {
  return fetchAPI(`/api/guilds/${guildId}/youtube-channels`);
}

export async function unlinkYouTubeChannel(guildId: string, channelId: string) {
  return fetchAPI(`/api/guilds/${guildId}/youtube-channels/${channelId}`, {
    method: 'DELETE',
  });
}

interface YouTubeChannel {
  id: string;
  youtube_channel_id: string;
  title: string;
  description: string;
  thumbnail_url: string;
}
```

---

### 9. Add Frontend UI

**File**: `frontend/src/components/Dashboard/YouTubeChannelManager.tsx`

```tsx
import { useState, useEffect } from 'react';
import { getYouTubeChannels, initiateYouTubeAuth, unlinkYouTubeChannel } from '../../services/api';

interface Props {
  guildId: string;
}

export function YouTubeChannelManager({ guildId }: Props) {
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadChannels();
  }, [guildId]);

  const loadChannels = async () => {
    setLoading(true);
    try {
      const data = await getYouTubeChannels(guildId);
      setChannels(data);
    } finally {
      setLoading(false);
    }
  };

  const handleAddChannel = async () => {
    const { url } = await initiateYouTubeAuth(guildId);
    window.location.href = url;
  };

  const handleRemoveChannel = async (channelId: string) => {
    if (confirm('Remove this YouTube channel?')) {
      await unlinkYouTubeChannel(guildId, channelId);
      loadChannels();
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <h3>YouTube Channels</h3>
      <button onClick={handleAddChannel}>+ Add YouTube Channel</button>

      {channels.map((channel) => (
        <div key={channel.id}>
          <img src={channel.thumbnail_url} alt={channel.title} />
          <h4>{channel.title}</h4>
          <p>{channel.description}</p>
          <button onClick={() => handleRemoveChannel(channel.id)}>Remove</button>
        </div>
      ))}
    </div>
  );
}
```

---

## Provider-Specific Notes

### Discord
- Scopes: `identify`, `guilds`
- Auth URL: `https://discord.com/api/oauth2/authorize`
- Token URL: `https://discord.com/api/oauth2/token`
- User API: `https://discord.com/api/users/@me`

### Twitch
- Scopes: `user:read:email` (or none for EventSub)
- Auth URL: `https://id.twitch.tv/oauth2/authorize`
- Token URL: `https://id.twitch.tv/oauth2/token`
- User API: `https://api.twitch.tv/helix/users`
- Requires `Client-Id` header for API calls

### YouTube/Google
- Scopes: `https://www.googleapis.com/auth/youtube.readonly`
- Auth URL: `https://accounts.google.com/o/oauth2/v2/auth`
- Token URL: `https://oauth2.googleapis.com/token`
- Channel API: `https://www.googleapis.com/youtube/v3/channels`
- Use `access_type=offline` to get refresh token

### Twitter
- Uses OAuth 1.0a (more complex, requires request tokens)
- Consider using OAuth 2.0 if available
- Scopes: `tweet.read`, `users.read`

---

## Testing

### Manual Testing
1. Click "Add [Provider]" button
2. Redirected to provider authorization page
3. Grant permissions
4. Redirected back to app
5. Provider account linked successfully
6. Remove provider account
7. Verify database cleanup

### Token Refresh Testing
1. Wait for token to expire
2. Trigger API call that uses token
3. Verify automatic refresh
4. Verify new token stored in database

---

## Security Considerations

### 1. Encrypt Tokens at Rest
Use AWS KMS or similar:

```go
func encryptToken(token string) (string, error) {
    // Use AWS KMS to encrypt token
    svc := kms.New(session.Must(session.NewSession()))
    result, err := svc.Encrypt(&kms.EncryptInput{
        KeyId:     aws.String(os.Getenv("KMS_KEY_ID")),
        Plaintext: []byte(token),
    })
    return base64.StdEncoding.EncodeToString(result.CiphertextBlob), err
}
```

### 2. Validate State Parameter
Always verify state matches to prevent CSRF:

```go
if stateCookie.Value != r.URL.Query().Get("state") {
    http.Error(w, "Invalid state", http.StatusBadRequest)
    return
}
```

### 3. Use HTTPS Only
Ensure redirect URIs use HTTPS in production.

---

## Checklist

- [ ] OAuth application created with provider
- [ ] Environment variables added
- [ ] OAuth service implemented
- [ ] Auth handler created
- [ ] Database schema created
- [ ] Database layer implemented
- [ ] Routes added to router
- [ ] Frontend API client added
- [ ] Frontend UI added
- [ ] Token refresh logic implemented
- [ ] Security measures implemented (CSRF, encryption)
- [ ] Manual testing completed
- [ ] Documentation updated
