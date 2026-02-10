# External Integrations

## Overview

StreamMaxing integrates with Discord and Twitch APIs to provide real-time stream notifications. This document covers API usage, webhook handling, and integration best practices.

---

## Discord Integration

### Discord Bot Setup

**Bot Configuration** (Discord Developer Portal):
1. Create application
2. Enable bot user
3. Add bot scopes: `bot`, `applications.commands`
4. Add bot permissions:
   - `Send Messages` (0x800)
   - `Embed Links` (0x4000)
   - `Mention Everyone` (0x20000)
5. Generate bot token
6. Calculate permissions integer: 0x800 + 0x4000 + 0x20000 = 149504

**Bot Installation URL**:
```
https://discord.com/api/oauth2/authorize?
  client_id=YOUR_CLIENT_ID&
  scope=bot&
  permissions=149504&
  guild_id=GUILD_ID
```

### Discord API Endpoints Used

#### Fetch User Guilds
**Endpoint**: `GET /users/@me/guilds`
**Auth**: Bearer token (user OAuth token)
**Response**:
```json
[
  {
    "id": "guild123",
    "name": "My Gaming Server",
    "icon": "icon_hash",
    "owner": true,
    "permissions": "2147483647"
  }
]
```

**Use Case**: List servers user can configure

#### Fetch Guild Channels
**Endpoint**: `GET /guilds/:guild_id/channels`
**Auth**: Bot token
**Response**:
```json
[
  {
    "id": "channel123",
    "type": 0,
    "name": "general",
    "position": 0
  }
]
```

**Use Case**: Populate channel dropdown in dashboard

**Filter**: Only show text channels (type = 0) and announcement channels (type = 5)

#### Fetch Guild Roles
**Endpoint**: `GET /guilds/:guild_id/roles`
**Auth**: Bot token
**Response**:
```json
[
  {
    "id": "role123",
    "name": "Streamers",
    "color": 3447003,
    "position": 5
  }
]
```

**Use Case**: Populate role dropdown for mention configuration

#### Send Message
**Endpoint**: `POST /channels/:channel_id/messages`
**Auth**: Bot token
**Body**:
```json
{
  "content": "@Streamers ExampleStreamer is now live!",
  "embeds": [
    {
      "title": "ExampleStreamer is streaming Just Chatting",
      "description": "Chill vibes and good times",
      "url": "https://twitch.tv/example_streamer",
      "color": 6570404,
      "thumbnail": {
        "url": "https://static-cdn.jtvnw.net/jtv_user_pictures/..."
      },
      "image": {
        "url": "https://static-cdn.jtvnw.net/previews-ttv/..."
      },
      "fields": [
        {
          "name": "Viewers",
          "value": "1,234",
          "inline": true
        },
        {
          "name": "Game",
          "value": "Just Chatting",
          "inline": true
        }
      ],
      "footer": {
        "text": "Twitch Notification"
      },
      "timestamp": "2024-01-15T10:30:00.000Z"
    }
  ]
}
```

**Use Case**: Send notification when streamer goes live

#### Check Guild Membership
**Endpoint**: `GET /guilds/:guild_id/members/:user_id`
**Auth**: Bot token
**Response**: 200 if member, 404 if not

**Use Case**: Verify user still in guild before updating preferences

### Discord Webhooks (Optional)

#### GUILD_DELETE Event
**Trigger**: Bot removed from guild
**Payload**:
```json
{
  "id": "guild123",
  "unavailable": false
}
```

**Action**: Cleanup guild data, orphaned streamers, EventSub subscriptions

#### GUILD_MEMBER_REMOVE Event
**Trigger**: User leaves guild
**Payload**:
```json
{
  "guild_id": "guild123",
  "user": {
    "id": "user456"
  }
}
```

**Action**: Delete user_preferences for (user_id, guild_id)

**Note**: These webhooks require maintaining a WebSocket connection or using Discord Interactions endpoint. **Recommended approach**: Lazy cleanup (check membership when needed) instead of real-time webhooks.

### Rate Limits

**Global Rate Limit**: 50 requests per second
**Per-Route Rate Limit**: Varies by endpoint

**Headers**:
- `X-RateLimit-Limit`: Total requests allowed
- `X-RateLimit-Remaining`: Requests remaining
- `X-RateLimit-Reset`: Timestamp when limit resets

**Handling**:
```go
func sendDiscordMessage(channelID, content string) error {
    resp, err := http.Post(...)
    if resp.StatusCode == 429 {
        retryAfter := resp.Header.Get("Retry-After")
        // Wait and retry
    }
}
```

### Error Handling

**Common Errors**:
- `401 Unauthorized`: Invalid bot token
- `403 Forbidden`: Missing permissions
- `404 Not Found`: Channel/guild doesn't exist
- `429 Too Many Requests`: Rate limited

**Mitigation**:
- Validate bot token on startup
- Check bot permissions before sending messages
- Handle rate limits with exponential backoff
- Cache channel/guild data to reduce API calls

---

## Twitch Integration

### Twitch EventSub

**What is EventSub?**
Twitch's webhook-based event subscription system for receiving real-time notifications.

**Subscription Types**:
- `stream.online` - Stream goes live (used in this project)
- `stream.offline` - Stream goes offline (future feature)
- `channel.update` - Stream metadata changes (future feature)

### Creating EventSub Subscriptions

**Endpoint**: `POST /helix/eventsub/subscriptions`
**Auth**: App access token (not user token)
**Headers**:
```
Authorization: Bearer APP_ACCESS_TOKEN
Client-Id: YOUR_CLIENT_ID
Content-Type: application/json
```

**Body**:
```json
{
  "type": "stream.online",
  "version": "1",
  "condition": {
    "broadcaster_user_id": "987654321"
  },
  "transport": {
    "method": "webhook",
    "callback": "https://your-api-url/webhooks/twitch",
    "secret": "YOUR_WEBHOOK_SECRET"
  }
}
```

**Response**:
```json
{
  "data": [
    {
      "id": "sub123",
      "status": "webhook_callback_verification_pending",
      "type": "stream.online",
      "version": "1",
      "condition": {
        "broadcaster_user_id": "987654321"
      },
      "created_at": "2024-01-15T10:00:00Z",
      "transport": {
        "method": "webhook",
        "callback": "https://your-api-url/webhooks/twitch"
      }
    }
  ]
}
```

**Status Flow**:
1. `webhook_callback_verification_pending` - Waiting for challenge response
2. `enabled` - Active and receiving events
3. `webhook_callback_verification_failed` - Challenge failed
4. `notification_failures_exceeded` - Too many delivery failures

### Webhook Challenge Response

**When**: Immediately after creating subscription
**Request** (from Twitch):
```json
{
  "subscription": { ... },
  "challenge": "some-random-string"
}
```

**Response** (your API):
- Status: 200 OK
- Body: Raw challenge string (not JSON)

**Implementation**:
```go
func handleTwitchWebhook(w http.ResponseWriter, r *http.Request) {
    var payload struct {
        Subscription map[string]interface{} `json:"subscription"`
        Challenge    string                 `json:"challenge"`
        Event        map[string]interface{} `json:"event"`
    }
    json.NewDecoder(r.Body).Decode(&payload)

    // Challenge response
    if payload.Challenge != "" {
        w.WriteHeader(200)
        w.Write([]byte(payload.Challenge))
        return
    }

    // Process event
    // ...
}
```

### Webhook Signature Verification

**Why**: Prevent spoofed webhook requests

**Headers** (from Twitch):
- `Twitch-Eventsub-Message-Id`: Unique message ID
- `Twitch-Eventsub-Message-Timestamp`: Timestamp
- `Twitch-Eventsub-Message-Signature`: HMAC-SHA256 signature

**Verification Steps**:
```
1. Concatenate: message_id + timestamp + request_body
2. Compute HMAC-SHA256 using webhook secret
3. Compare with signature header (prepend "sha256=")
```

**Implementation**:
```go
func verifyTwitchSignature(messageID, timestamp, signature string, body []byte) bool {
    secret := os.Getenv("TWITCH_WEBHOOK_SECRET")
    message := messageID + timestamp + string(body)

    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(message))
    computed := "sha256=" + hex.EncodeToString(h.Sum(nil))

    return hmac.Equal([]byte(signature), []byte(computed))
}
```

**Reject if**:
- Signature doesn't match
- Timestamp is older than 10 minutes (prevent replay attacks)

### Stream.Online Event Payload

**Request** (from Twitch):
```json
{
  "subscription": {
    "id": "sub123",
    "type": "stream.online",
    "version": "1",
    "condition": {
      "broadcaster_user_id": "987654321"
    }
  },
  "event": {
    "id": "event123",
    "broadcaster_user_id": "987654321",
    "broadcaster_user_login": "example_streamer",
    "broadcaster_user_name": "ExampleStreamer",
    "type": "live",
    "started_at": "2024-01-15T10:30:00Z"
  }
}
```

**Use Case**: Trigger notification fanout

**Important**: Event payload does NOT include stream title, game, or viewer count. Must fetch from Streams API.

### Fetching Stream Data

**Endpoint**: `GET /helix/streams?user_id=987654321`
**Auth**: App access token
**Response**:
```json
{
  "data": [
    {
      "id": "stream123",
      "user_id": "987654321",
      "user_login": "example_streamer",
      "user_name": "ExampleStreamer",
      "game_id": "509658",
      "game_name": "Just Chatting",
      "title": "Chill vibes and good times",
      "viewer_count": 1234,
      "thumbnail_url": "https://static-cdn.jtvnw.net/previews-ttv/...-{width}x{height}.jpg",
      "started_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

**Note**: Replace `{width}x{height}` in thumbnail URL (e.g., `1920x1080`)

### Deleting EventSub Subscriptions

**Endpoint**: `DELETE /helix/eventsub/subscriptions?id=sub123`
**Auth**: App access token
**Use Case**: Streamer disconnected, cleanup stale subscriptions

### Listing All Subscriptions

**Endpoint**: `GET /helix/eventsub/subscriptions`
**Auth**: App access token
**Use Case**: Admin cleanup job to find orphaned subscriptions

### Rate Limits

**Twitch API**: ~800 requests per minute (per client)
**EventSub**: Max 10,000 subscriptions per client (sufficient)

### Error Handling

**Common Errors**:
- `401 Unauthorized`: Invalid app access token
- `403 Forbidden`: Missing scopes (shouldn't happen for EventSub)
- `409 Conflict`: Subscription already exists
- `429 Too Many Requests`: Rate limited

**Mitigation**:
- Refresh app access token every 60 days
- Check for existing subscriptions before creating
- Handle 409 errors gracefully (use existing subscription)

### App Access Token

**Unlike user OAuth**, EventSub requires an app access token (client credentials flow).

**Obtaining Token**:
```bash
POST https://id.twitch.tv/oauth2/token
Body:
  client_id=YOUR_CLIENT_ID
  client_secret=YOUR_CLIENT_SECRET
  grant_type=client_credentials
```

**Response**:
```json
{
  "access_token": "app_token_123",
  "expires_in": 5184000,
  "token_type": "bearer"
}
```

**Storage**: Environment variable or database (refresh before expiration)

---

## Integration Best Practices

### Caching

**Discord Guilds/Channels**: Cache for 5 minutes to reduce API calls
**Twitch Stream Data**: No cache (always fetch fresh for notifications)

### Idempotency

**Notification Log**: Track (guild_id, event_id) to prevent duplicate notifications
**Database Constraint**: `UNIQUE(guild_id, event_id)` on notification_log table

### Error Recovery

**Discord Message Fails**:
- Log error (channel deleted, bot removed, etc.)
- Do NOT retry (webhook already returned 200 to Twitch)
- Mark notification as failed in database

**EventSub Subscription Fails**:
- Store error message in database
- Show retry button in UI
- Allow manual retry via API endpoint

### Monitoring

**Metrics to Track**:
- EventSub subscription status (enabled vs failed)
- Discord API rate limit hits
- Webhook signature verification failures
- Notification delivery success rate

**Alerts**:
- EventSub subscription status changes to failed
- Discord API returns 403 (permission error)
- High webhook signature verification failure rate (potential attack)

---

## Testing Integrations

### Discord Testing
- Create test Discord server
- Install bot to test server
- Test sending messages with embeds
- Test permission errors (remove bot permissions)

### Twitch Testing
- Use Twitch CLI for local EventSub testing
- `twitch event trigger stream.online --forward-address=http://localhost:3000/webhooks/twitch`
- Test challenge response
- Test signature verification

### Webhook Testing Tools
- ngrok for exposing local server to internet
- Postman for manually crafting webhook requests
- curl for command-line testing

---

## Security Considerations

### Webhook Signature Verification
- **ALWAYS** verify Twitch webhook signatures
- Reject requests with invalid signatures
- Check timestamp to prevent replay attacks

### Bot Token Security
- Store in environment variables (never commit to git)
- Use AWS Secrets Manager or Parameter Store
- Rotate tokens if compromised

### Rate Limit Abuse Prevention
- Implement per-user rate limits (future enhancement)
- Monitor for unusual webhook patterns
- Block IPs with repeated signature failures

### Discord Permissions
- Use least privilege (only required permissions)
- Regularly audit bot permissions
- Remove bot from inactive guilds (future cleanup job)
