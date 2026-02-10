# Authentication & Authorization

## Overview

StreamMaxing uses OAuth 2.0 for both Discord and Twitch authentication. Discord OAuth authenticates users and retrieves their server list, while Twitch OAuth authorizes the system to create EventSub subscriptions for streamers.

---

## Discord OAuth Flow

### Purpose
- Authenticate users (admins and regular users)
- Retrieve user's Discord guilds (servers)
- Verify user permissions in guilds

### OAuth Configuration

**Application Settings** (Discord Developer Portal):
- **Client ID**: Public identifier for the application
- **Client Secret**: Secret for exchanging authorization codes
- **Redirect URI**: `https://your-api-url/api/auth/discord/callback`
- **Scopes**: `identify guilds`
- **Permissions**: None needed for OAuth (bot permissions separate)

### Authorization Flow

```
1. User clicks "Login with Discord"
   Frontend → Discord OAuth URL:
   https://discord.com/api/oauth2/authorize?
     client_id=YOUR_CLIENT_ID&
     redirect_uri=https://your-api-url/api/auth/discord/callback&
     response_type=code&
     scope=identify%20guilds

2. User authorizes application on Discord

3. Discord redirects to callback URL with code
   Discord → Frontend:
   https://your-api-url/api/auth/discord/callback?code=ABC123

4. Backend exchanges code for access token
   Backend → Discord API:
   POST https://discord.com/api/oauth2/token
   Body: {
     client_id: YOUR_CLIENT_ID,
     client_secret: YOUR_CLIENT_SECRET,
     grant_type: "authorization_code",
     code: "ABC123",
     redirect_uri: "https://your-api-url/api/auth/discord/callback"
   }

   Response: {
     access_token: "xyz789",
     token_type: "Bearer",
     expires_in: 604800,
     refresh_token: "refresh123",
     scope: "identify guilds"
   }

5. Backend fetches user info
   Backend → Discord API:
   GET https://discord.com/api/users/@me
   Headers: { Authorization: "Bearer xyz789" }

   Response: {
     id: "123456789",
     username: "ExampleUser",
     avatar: "abc123",
     ...
   }

6. Backend fetches user's guilds
   Backend → Discord API:
   GET https://discord.com/api/users/@me/guilds
   Headers: { Authorization: "Bearer xyz789" }

   Response: [
     {
       id: "guild123",
       name: "My Gaming Server",
       icon: "icon123",
       owner: true,
       permissions: "2147483647"
     },
     ...
   ]

7. Backend creates session
   - Generate JWT token with user_id and expiration
   - Store minimal data in JWT (user_id, username)
   - Insert/update user in database
   - Set HTTP-only cookie with JWT

8. Backend redirects to frontend dashboard
   Response: Set-Cookie: session=JWT_TOKEN; HttpOnly; Secure; SameSite=Strict
   Redirect: https://your-cloudfront-url/dashboard
```

### Endpoints

**GET /api/auth/discord/login**
- Generates Discord OAuth URL with state parameter (CSRF protection)
- Redirects user to Discord authorization page

**GET /api/auth/discord/callback**
- Receives authorization code from Discord
- Exchanges code for access token
- Fetches user info and guilds
- Creates session (JWT)
- Redirects to dashboard

**POST /api/auth/logout**
- Clears session cookie
- Returns success response

---

## Twitch OAuth Flow

### Purpose
- Authorize the system to create EventSub subscriptions for a streamer
- Obtain streamer's broadcaster_user_id
- Store access token for API calls (e.g., fetching stream data)

### OAuth Configuration

**Application Settings** (Twitch Developer Console):
- **Client ID**: Public identifier
- **Client Secret**: Secret for token exchange
- **Redirect URI**: `https://your-api-url/api/auth/twitch/callback`
- **Scopes**: None required for EventSub (scopes optional for future features)

### Authorization Flow

```
1. Admin clicks "Add Streamer" in dashboard
   Frontend → Backend: GET /api/guilds/:guild_id/streamers/link
   Backend generates Twitch OAuth URL and returns it

2. Frontend redirects to Twitch OAuth URL
   https://id.twitch.tv/oauth2/authorize?
     client_id=YOUR_CLIENT_ID&
     redirect_uri=https://your-api-url/api/auth/twitch/callback&
     response_type=code&
     scope=user:read:email&
     state=guild_id:guild123

3. Streamer authorizes application on Twitch

4. Twitch redirects to callback URL
   Twitch → Backend:
   https://your-api-url/api/auth/twitch/callback?code=DEF456&state=guild_id:guild123

5. Backend exchanges code for access token
   Backend → Twitch API:
   POST https://id.twitch.tv/oauth2/token
   Body: {
     client_id: YOUR_CLIENT_ID,
     client_secret: YOUR_CLIENT_SECRET,
     grant_type: "authorization_code",
     code: "DEF456",
     redirect_uri: "https://your-api-url/api/auth/twitch/callback"
   }

   Response: {
     access_token: "twitch_token_123",
     expires_in: 14367,
     refresh_token: "refresh456",
     token_type: "bearer"
   }

6. Backend fetches streamer info
   Backend → Twitch API:
   GET https://api.twitch.tv/helix/users
   Headers: {
     Authorization: "Bearer twitch_token_123",
     Client-Id: YOUR_CLIENT_ID
   }

   Response: {
     data: [{
       id: "987654321",
       login: "example_streamer",
       display_name: "ExampleStreamer",
       profile_image_url: "https://..."
     }]
   }

7. Backend stores streamer in database
   INSERT INTO streamers (twitch_broadcaster_id, twitch_login, ...)
   VALUES ('987654321', 'example_streamer', ...)

8. Backend creates EventSub subscription
   Backend → Twitch API:
   POST https://api.twitch.tv/helix/eventsub/subscriptions
   Headers: {
     Authorization: "Bearer APP_ACCESS_TOKEN",
     Client-Id: YOUR_CLIENT_ID,
     Content-Type: "application/json"
   }
   Body: {
     type: "stream.online",
     version: "1",
     condition: {
       broadcaster_user_id: "987654321"
     },
     transport: {
       method: "webhook",
       callback: "https://your-api-url/webhooks/twitch",
       secret: "YOUR_WEBHOOK_SECRET"
     }
   }

   Response: {
     data: [{
       id: "sub123",
       status: "webhook_callback_verification_pending",
       type: "stream.online",
       ...
     }]
   }

9. Backend stores subscription in database
   INSERT INTO eventsub_subscriptions (streamer_id, subscription_id, status)
   VALUES (streamer_id, 'sub123', 'pending')

10. Backend links streamer to guild
    INSERT INTO guild_streamers (guild_id, streamer_id, enabled)
    VALUES ('guild123', streamer_id, true)

11. Backend redirects to dashboard
    Response: Redirect to frontend with success message
```

### Endpoints

**GET /api/guilds/:guild_id/streamers/link**
- Generates Twitch OAuth URL with state parameter containing guild_id
- Returns OAuth URL to frontend

**GET /api/auth/twitch/callback**
- Receives authorization code from Twitch
- Exchanges code for access token
- Fetches streamer info
- Creates streamer record
- Creates EventSub subscription
- Links streamer to guild
- Redirects to dashboard

---

## Session Management

### JWT Token Structure

**Payload**:
```json
{
  "user_id": "123456789",
  "username": "ExampleUser",
  "iat": 1609459200,
  "exp": 1610064000
}
```

**Signing**:
- Algorithm: HS256 (HMAC with SHA-256)
- Secret: Environment variable `JWT_SECRET` (min 32 characters)

**Storage**:
- HTTP-only cookie (prevents XSS)
- Secure flag (HTTPS only)
- SameSite=Strict (CSRF protection)
- Max-Age: 7 days (604800 seconds)

### Token Validation Middleware

**Flow**:
```
1. Extract JWT from cookie
2. Verify signature using JWT_SECRET
3. Check expiration (exp claim)
4. Extract user_id from payload
5. Optionally: Query database to verify user still exists
6. Attach user_id to request context
7. Continue to handler
```

**Implementation** (Go):
```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("session")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
            return []byte(os.Getenv("JWT_SECRET")), nil
        })

        if err != nil || !token.Valid {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        claims := token.Claims.(jwt.MapClaims)
        userID := claims["user_id"].(string)

        // Attach user_id to context
        ctx := context.WithValue(r.Context(), "user_id", userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## Authorization (Permissions)

### Guild Permissions

**Admin Actions** (require owner or admin role):
- Update guild configuration (channel, role, template)
- Add/remove streamers
- Enable/disable streamer notifications

**Permission Check**:
```
1. User requests action on guild X
2. Backend fetches user's guilds from Discord API (cached for 5 minutes)
3. Check if user has guild X in their guilds list
4. Check if user has ADMINISTRATOR permission (bitwise check)
5. If yes, allow action
6. If no, return 403 Forbidden
```

**Implementation**:
```go
func CheckGuildPermission(userID, guildID string) (bool, error) {
    // Fetch user's guilds from Discord API
    guilds, err := fetchUserGuilds(userID)
    if err != nil {
        return false, err
    }

    // Check if user is in guild and has admin permission
    for _, guild := range guilds {
        if guild.ID == guildID {
            // 0x8 = ADMINISTRATOR permission
            if guild.Permissions & 0x8 != 0 {
                return true, nil
            }
        }
    }

    return false, nil
}
```

### User Preferences

**User Actions** (require user to be in guild):
- Update notification preferences for streamers in guild X

**Permission Check**:
```
1. User requests to update preferences in guild X
2. Backend checks if user is member of guild X (Discord API)
3. If yes, allow action
4. If no, return 403 Forbidden
```

---

## Token Refresh

### Discord Token Refresh
- Discord access tokens expire after 7 days
- Refresh tokens can be used to get new access tokens
- **Current implementation**: User re-authenticates (no refresh logic)
- **Future enhancement**: Implement refresh token flow

### Twitch Token Refresh
- Twitch access tokens expire after ~4 hours
- Refresh tokens can be used indefinitely
- **Implementation**:
  ```go
  func refreshTwitchToken(refreshToken string) (string, error) {
      resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", url.Values{
          "client_id":     {os.Getenv("TWITCH_CLIENT_ID")},
          "client_secret": {os.Getenv("TWITCH_CLIENT_SECRET")},
          "grant_type":    {"refresh_token"},
          "refresh_token": {refreshToken},
      })
      // Parse response and return new access token
  }
  ```

---

## Security Considerations

### CSRF Protection
- **State Parameter**: Random string stored in session, verified on callback
- **SameSite Cookie**: Prevents CSRF attacks

### XSS Protection
- **HTTP-only Cookies**: JWT not accessible via JavaScript
- **Content Security Policy**: Restrict inline scripts

### Token Storage
- **Never store tokens in localStorage**: Use HTTP-only cookies
- **Encrypt sensitive tokens**: Twitch access/refresh tokens (future: AWS KMS)

### Rate Limiting
- **Discord API**: 10,000 requests per 10 minutes (per bot)
- **Twitch API**: ~800 requests per minute (per client)
- **Mitigation**: Cache user guilds, avoid redundant API calls

### Secret Rotation
- **JWT Secret**: Rotate every 90 days (invalidates all sessions)
- **Webhook Secret**: Rotate every 90 days (update EventSub subscriptions)
- **OAuth Secrets**: Rotate when compromised

---

## Error Handling

### OAuth Errors

**Discord OAuth Error**:
- User denies authorization → Redirect to login page with error message
- Invalid code → Log error, show "Authentication failed" message
- Network error → Retry with exponential backoff

**Twitch OAuth Error**:
- User denies authorization → Redirect to dashboard with error message
- EventSub subscription fails → Store error in database, show retry button

### Session Errors

**Expired JWT**:
- Return 401 Unauthorized
- Frontend redirects to login page

**Invalid JWT**:
- Return 401 Unauthorized
- Clear cookie
- Frontend redirects to login page

**Database Error** (user lookup fails):
- Return 500 Internal Server Error
- Log error for debugging

---

## Testing

### Local Testing
- Use ngrok for webhook callbacks (Twitch requires HTTPS)
- Test Discord OAuth with test server
- Test Twitch OAuth with test account

### Automated Testing
- Mock Discord/Twitch API responses
- Test JWT generation and validation
- Test permission checks

### Security Testing
- Test CSRF protection (state parameter)
- Test token expiration
- Test unauthorized access (missing/invalid JWT)
