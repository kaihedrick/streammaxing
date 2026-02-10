# Task 004: Notification Fanout

## Status
Complete

## Overview
Implement the notification delivery system that processes Twitch stream.online events and sends Discord notifications to all configured guilds, respecting user preferences and using custom message templates.

---

## Goals
1. Process stream.online webhook events from Twitch
2. Fetch stream metadata (title, game, viewers, thumbnail)
3. Query all guilds tracking the streamer
4. Fetch guild configuration (channel, role, template)
5. Query user preferences to exclude opted-out users
6. Render message templates with dynamic data
7. Send Discord notifications with embeds
8. Implement idempotency to prevent duplicate notifications
9. Handle errors gracefully (missing channels, permission errors)
10. Log notification delivery for auditing

---

## Prerequisites
- Task 001 (Project Bootstrap) completed
- Task 002 (Discord Auth) completed
- Task 003 (Twitch EventSub) completed
- Database schema includes: notification_log, user_preferences, guild_config
- Discord bot installed in at least one guild
- Twitch EventSub subscription active

---

## Architecture Flow

```
Twitch EventSub → Webhook Handler → Notification Service
                                    ↓
                    ┌───────────────┴──────────────┐
                    │                              │
            Fetch Stream Data            Query Guilds Tracking Streamer
                    │                              │
                    └───────────────┬──────────────┘
                                    ↓
                    For Each Guild:
                    1. Check idempotency (notification_log)
                    2. Fetch guild_config (channel, template, role)
                    3. Query user_preferences (opted-out users)
                    4. Render template with stream data
                    5. Send Discord message
                    6. Log notification
```

---

## Backend Implementation

### File Structure
```
backend/
├── internal/
│   ├── services/
│   │   └── notifications/
│   │       ├── fanout.go         # Notification fanout logic
│   │       ├── template.go       # Template rendering
│   │       └── discord.go        # Discord message sending
│   └── db/
│       ├── notification_log.go   # Idempotency tracking
│       └── user_preferences.go   # User preference queries
```

### 1. Notification Fanout Service

**File**: `backend/internal/services/notifications/fanout.go`

```go
package notifications

import (
    "context"
    "fmt"
    "log"

    "your-module/internal/db"
    "your-module/internal/services/discord"
    "your-module/internal/services/twitch"
)

type FanoutService struct {
    twitchAPI     *twitch.APIClient
    discordAPI    *discord.APIClient
    streamerDB    *db.StreamerDB
    guildDB       *db.GuildStreamerDB
    configDB      *db.GuildConfigDB
    preferencesDB *db.UserPreferencesDB
    notifLogDB    *db.NotificationLogDB
    templateSvc   *TemplateService
}

func NewFanoutService(
    twitchAPI *twitch.APIClient,
    discordAPI *discord.APIClient,
    streamerDB *db.StreamerDB,
    guildDB *db.GuildStreamerDB,
    configDB *db.GuildConfigDB,
    preferencesDB *db.UserPreferencesDB,
    notifLogDB *db.NotificationLogDB,
) *FanoutService {
    return &FanoutService{
        twitchAPI:     twitchAPI,
        discordAPI:    discordAPI,
        streamerDB:    streamerDB,
        guildDB:       guildDB,
        configDB:      configDB,
        preferencesDB: preferencesDB,
        notifLogDB:    notifLogDB,
        templateSvc:   NewTemplateService(),
    }
}

type StreamOnlineEvent struct {
    ID                  string `json:"id"`
    BroadcasterUserID   string `json:"broadcaster_user_id"`
    BroadcasterUserLogin string `json:"broadcaster_user_login"`
    BroadcasterUserName string `json:"broadcaster_user_name"`
    Type                string `json:"type"`
    StartedAt           string `json:"started_at"`
}

func (s *FanoutService) HandleStreamOnline(ctx context.Context, eventID string, event StreamOnlineEvent) error {
    // Fetch full stream data (title, game, viewers, thumbnail)
    streamData, err := s.twitchAPI.GetStreamData(event.BroadcasterUserID)
    if err != nil {
        log.Printf("Failed to fetch stream data for %s: %v", event.BroadcasterUserID, err)
        return err
    }

    // Get streamer from database
    streamer, err := s.streamerDB.GetStreamerByBroadcasterID(event.BroadcasterUserID)
    if err != nil {
        log.Printf("Streamer not found: %s", event.BroadcasterUserID)
        return err
    }

    // Query all guilds tracking this streamer
    guilds, err := s.guildDB.GetGuildsTrackingStreamer(streamer.ID)
    if err != nil {
        log.Printf("Failed to fetch guilds for streamer %s: %v", streamer.ID, err)
        return err
    }

    log.Printf("Fanout: %s went live, notifying %d guilds", event.BroadcasterUserName, len(guilds))

    // Fan out to each guild
    for _, guild := range guilds {
        if err := s.sendNotificationToGuild(ctx, guild.GuildID, streamer, streamData, eventID); err != nil {
            log.Printf("Failed to send notification to guild %s: %v", guild.GuildID, err)
            // Continue to next guild (don't fail entire fanout)
        }
    }

    return nil
}

func (s *FanoutService) sendNotificationToGuild(
    ctx context.Context,
    guildID string,
    streamer *db.Streamer,
    streamData *twitch.StreamData,
    eventID string,
) error {
    // Check idempotency (prevent duplicate notifications)
    isDuplicate, err := s.notifLogDB.CheckDuplicate(guildID, eventID)
    if err != nil {
        return fmt.Errorf("idempotency check failed: %w", err)
    }
    if isDuplicate {
        log.Printf("Duplicate notification for guild %s, event %s", guildID, eventID)
        return nil
    }

    // Fetch guild configuration
    config, err := s.configDB.GetGuildConfig(guildID)
    if err != nil {
        return fmt.Errorf("failed to fetch guild config: %w", err)
    }

    if !config.Enabled {
        log.Printf("Notifications disabled for guild %s", guildID)
        return nil
    }

    // Query opted-out users
    optedOutUsers, err := s.preferencesDB.GetOptedOutUsers(guildID, streamer.ID)
    if err != nil {
        log.Printf("Failed to fetch user preferences: %v", err)
        // Continue anyway (better to send to everyone than skip)
    }

    // Render message template
    message, err := s.templateSvc.RenderTemplate(config.MessageTemplate, streamer, streamData, config.MentionRoleID)
    if err != nil {
        return fmt.Errorf("template rendering failed: %w", err)
    }

    // Send Discord message
    if err := s.discordAPI.SendMessage(config.ChannelID, message, optedOutUsers); err != nil {
        return fmt.Errorf("discord send failed: %w", err)
    }

    // Log notification
    if err := s.notifLogDB.LogNotification(guildID, streamer.ID, eventID); err != nil {
        log.Printf("Failed to log notification: %v", err)
        // Non-fatal error
    }

    log.Printf("Successfully sent notification to guild %s", guildID)
    return nil
}
```

### 2. Template Rendering Service

**File**: `backend/internal/services/notifications/template.go`

```go
package notifications

import (
    "encoding/json"
    "fmt"
    "strings"

    "your-module/internal/db"
    "your-module/internal/services/twitch"
)

type TemplateService struct{}

func NewTemplateService() *TemplateService {
    return &TemplateService{}
}

type MessageTemplate struct {
    Content string              `json:"content"`
    Embed   *EmbedTemplate      `json:"embed,omitempty"`
}

type EmbedTemplate struct {
    Title       string         `json:"title"`
    Description string         `json:"description"`
    URL         string         `json:"url"`
    Color       int            `json:"color"`
    Thumbnail   *ImageTemplate `json:"thumbnail,omitempty"`
    Image       *ImageTemplate `json:"image,omitempty"`
    Fields      []FieldTemplate `json:"fields,omitempty"`
    Footer      *FooterTemplate `json:"footer,omitempty"`
    Timestamp   bool           `json:"timestamp,omitempty"`
}

type ImageTemplate struct {
    URL string `json:"url"`
}

type FieldTemplate struct {
    Name   string `json:"name"`
    Value  string `json:"value"`
    Inline bool   `json:"inline"`
}

type FooterTemplate struct {
    Text string `json:"text"`
}

func (s *TemplateService) RenderTemplate(
    templateJSON json.RawMessage,
    streamer *db.Streamer,
    streamData *twitch.StreamData,
    mentionRoleID *string,
) (*DiscordMessage, error) {
    var template MessageTemplate
    if err := json.Unmarshal(templateJSON, &template); err != nil {
        return nil, err
    }

    // Build variable map
    vars := map[string]string{
        "{streamer_login}":         streamer.Login,
        "{streamer_display_name}":  streamer.DisplayName,
        "{streamer_avatar_url}":    streamer.AvatarURL,
        "{stream_title}":           streamData.Title,
        "{game_name}":              streamData.GameName,
        "{viewer_count}":           fmt.Sprintf("%d", streamData.ViewerCount),
        "{stream_thumbnail_url}":   strings.ReplaceAll(streamData.ThumbnailURL, "{width}x{height}", "1920x1080"),
        "{started_at}":             streamData.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
    }

    // Add mention role if configured
    if mentionRoleID != nil && *mentionRoleID != "" {
        vars["{mention_role}"] = fmt.Sprintf("<@&%s>", *mentionRoleID)
    } else {
        vars["{mention_role}"] = ""
    }

    // Render content
    content := replaceVariables(template.Content, vars)

    // Render embed
    var embed *DiscordEmbed
    if template.Embed != nil {
        embed = &DiscordEmbed{
            Title:       replaceVariables(template.Embed.Title, vars),
            Description: replaceVariables(template.Embed.Description, vars),
            URL:         replaceVariables(template.Embed.URL, vars),
            Color:       template.Embed.Color,
        }

        if template.Embed.Thumbnail != nil {
            embed.Thumbnail = &DiscordImage{
                URL: replaceVariables(template.Embed.Thumbnail.URL, vars),
            }
        }

        if template.Embed.Image != nil {
            embed.Image = &DiscordImage{
                URL: replaceVariables(template.Embed.Image.URL, vars),
            }
        }

        for _, field := range template.Embed.Fields {
            embed.Fields = append(embed.Fields, DiscordField{
                Name:   replaceVariables(field.Name, vars),
                Value:  replaceVariables(field.Value, vars),
                Inline: field.Inline,
            })
        }

        if template.Embed.Footer != nil {
            embed.Footer = &DiscordFooter{
                Text: replaceVariables(template.Embed.Footer.Text, vars),
            }
        }

        if template.Embed.Timestamp {
            embed.Timestamp = streamData.StartedAt.Format("2006-01-02T15:04:05Z07:00")
        }
    }

    return &DiscordMessage{
        Content: content,
        Embeds:  []*DiscordEmbed{embed},
    }, nil
}

func replaceVariables(text string, vars map[string]string) string {
    for key, value := range vars {
        text = strings.ReplaceAll(text, key, value)
    }
    return text
}

type DiscordMessage struct {
    Content string          `json:"content"`
    Embeds  []*DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
    Title       string         `json:"title,omitempty"`
    Description string         `json:"description,omitempty"`
    URL         string         `json:"url,omitempty"`
    Color       int            `json:"color,omitempty"`
    Thumbnail   *DiscordImage  `json:"thumbnail,omitempty"`
    Image       *DiscordImage  `json:"image,omitempty"`
    Fields      []DiscordField `json:"fields,omitempty"`
    Footer      *DiscordFooter `json:"footer,omitempty"`
    Timestamp   string         `json:"timestamp,omitempty"`
}

type DiscordImage struct {
    URL string `json:"url"`
}

type DiscordField struct {
    Name   string `json:"name"`
    Value  string `json:"value"`
    Inline bool   `json:"inline"`
}

type DiscordFooter struct {
    Text string `json:"text"`
}
```

### 3. Discord Message Sending

**File**: `backend/internal/services/notifications/discord.go`

```go
package notifications

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

type DiscordSender struct {
    botToken string
}

func NewDiscordSender() *DiscordSender {
    return &DiscordSender{
        botToken: os.Getenv("DISCORD_BOT_TOKEN"),
    }
}

func (d *DiscordSender) SendMessage(channelID string, message *DiscordMessage, optedOutUsers []string) error {
    // TODO: For now, send to channel (in future, send DMs or suppress mentions for opted-out users)

    url := fmt.Sprintf("https://discord.com/api/channels/%s/messages", channelID)

    body, _ := json.Marshal(message)
    req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bot "+d.botToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        return fmt.Errorf("discord API error: %d", resp.StatusCode)
    }

    return nil
}
```

### 4. Database Layer - Notification Log

**File**: `backend/internal/db/notification_log.go`

```go
package db

import (
    "context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type NotificationLogDB struct {
    pool *pgxpool.Pool
}

func NewNotificationLogDB(pool *pgxpool.Pool) *NotificationLogDB {
    return &NotificationLogDB{pool: pool}
}

func (db *NotificationLogDB) CheckDuplicate(guildID, eventID string) (bool, error) {
    query := `SELECT 1 FROM notification_log WHERE guild_id = $1 AND event_id = $2 LIMIT 1`

    var exists int
    err := db.pool.QueryRow(context.Background(), query, guildID, eventID).Scan(&exists)

    if err == pgx.ErrNoRows {
        return false, nil
    }
    if err != nil {
        return false, err
    }

    return true, nil
}

func (db *NotificationLogDB) LogNotification(guildID string, streamerID uuid.UUID, eventID string) error {
    query := `
        INSERT INTO notification_log (guild_id, streamer_id, event_id)
        VALUES ($1, $2, $3)
        ON CONFLICT (guild_id, event_id) DO NOTHING
    `
    _, err := db.pool.Exec(context.Background(), query, guildID, streamerID, eventID)
    return err
}
```

### 5. Database Layer - User Preferences

**File**: `backend/internal/db/user_preferences.go`

```go
package db

import (
    "context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type UserPreferencesDB struct {
    pool *pgxpool.Pool
}

func NewUserPreferencesDB(pool *pgxpool.Pool) *UserPreferencesDB {
    return &UserPreferencesDB{pool: pool}
}

func (db *UserPreferencesDB) GetOptedOutUsers(guildID string, streamerID uuid.UUID) ([]string, error) {
    query := `
        SELECT user_id
        FROM user_preferences
        WHERE guild_id = $1 AND streamer_id = $2 AND notifications_enabled = false
    `

    rows, err := db.pool.Query(context.Background(), query, guildID, streamerID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var userIDs []string
    for rows.Next() {
        var userID string
        if err := rows.Scan(&userID); err != nil {
            return nil, err
        }
        userIDs = append(userIDs, userID)
    }

    return userIDs, nil
}

func (db *UserPreferencesDB) SetUserPreference(userID, guildID string, streamerID uuid.UUID, enabled bool) error {
    query := `
        INSERT INTO user_preferences (user_id, guild_id, streamer_id, notifications_enabled)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (user_id, guild_id, streamer_id)
        DO UPDATE SET notifications_enabled = $4, updated_at = now()
    `
    _, err := db.pool.Exec(context.Background(), query, userID, guildID, streamerID, enabled)
    return err
}

func (db *UserPreferencesDB) GetUserPreferences(userID string) ([]UserPreference, error) {
    query := `
        SELECT guild_id, streamer_id, notifications_enabled
        FROM user_preferences
        WHERE user_id = $1
    `

    rows, err := db.pool.Query(context.Background(), query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var prefs []UserPreference
    for rows.Next() {
        var pref UserPreference
        if err := rows.Scan(&pref.GuildID, &pref.StreamerID, &pref.NotificationsEnabled); err != nil {
            return nil, err
        }
        prefs = append(prefs, pref)
    }

    return prefs, nil
}

type UserPreference struct {
    GuildID              string
    StreamerID           uuid.UUID
    NotificationsEnabled bool
}
```

### 6. Database Layer - Guild Config

**File**: `backend/internal/db/guild_config.go`

```go
package db

import (
    "context"
    "encoding/json"

    "github.com/jackc/pgx/v5/pgxpool"
)

type GuildConfigDB struct {
    pool *pgxpool.Pool
}

func NewGuildConfigDB(pool *pgxpool.Pool) *GuildConfigDB {
    return &GuildConfigDB{pool: pool}
}

func (db *GuildConfigDB) GetGuildConfig(guildID string) (*GuildConfig, error) {
    query := `
        SELECT guild_id, channel_id, mention_role_id, message_template, enabled
        FROM guild_config
        WHERE guild_id = $1
    `

    var config GuildConfig
    var mentionRoleID *string
    err := db.pool.QueryRow(context.Background(), query, guildID).Scan(
        &config.GuildID,
        &config.ChannelID,
        &mentionRoleID,
        &config.MessageTemplate,
        &config.Enabled,
    )

    config.MentionRoleID = mentionRoleID
    return &config, err
}

func (db *GuildConfigDB) UpsertGuildConfig(config *GuildConfig) error {
    query := `
        INSERT INTO guild_config (guild_id, channel_id, mention_role_id, message_template, enabled)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (guild_id)
        DO UPDATE SET
            channel_id = $2,
            mention_role_id = $3,
            message_template = $4,
            enabled = $5,
            updated_at = now()
    `
    _, err := db.pool.Exec(
        context.Background(),
        query,
        config.GuildID,
        config.ChannelID,
        config.MentionRoleID,
        config.MessageTemplate,
        config.Enabled,
    )
    return err
}

type GuildConfig struct {
    GuildID         string
    ChannelID       string
    MentionRoleID   *string
    MessageTemplate json.RawMessage
    Enabled         bool
}
```

### 7. Update Webhook Handler

**File**: `backend/internal/handlers/webhooks.go` (update)

```go
func (h *WebhookHandler) HandleTwitchWebhook(w http.ResponseWriter, r *http.Request) {
    // ... (existing signature verification and challenge code) ...

    // Handle stream.online notification
    if payload.Subscription.Type == "stream.online" {
        event := StreamOnlineEvent{
            ID:                  payload.Event["id"].(string),
            BroadcasterUserID:   payload.Event["broadcaster_user_id"].(string),
            BroadcasterUserLogin: payload.Event["broadcaster_user_login"].(string),
            BroadcasterUserName: payload.Event["broadcaster_user_name"].(string),
            Type:                payload.Event["type"].(string),
            StartedAt:           payload.Event["started_at"].(string),
        }

        // Process asynchronously (don't block webhook response)
        go func() {
            ctx := context.Background()
            if err := h.fanoutService.HandleStreamOnline(ctx, messageID, event); err != nil {
                log.Printf("Fanout failed: %v", err)
            }
        }()
    }

    w.WriteHeader(http.StatusOK)
}
```

---

## API Endpoints

### User Preferences Routes

**GET /api/users/me/preferences**
- Description: Get user's notification preferences
- Auth: Required (JWT)
- Response: Array of preference objects

**PUT /api/users/me/preferences/:guild_id/:streamer_id**
- Description: Update notification preference for specific streamer in guild
- Auth: Required (JWT + guild membership)
- Body: `{ "enabled": true/false }`
- Response: `{ "message": "Preference updated" }`

### Guild Config Routes (for Task 005)

**GET /api/guilds/:guild_id/config**
- Description: Get guild notification configuration
- Auth: Required (JWT + guild admin permission)
- Response: Guild config object

**PUT /api/guilds/:guild_id/config**
- Description: Update guild notification configuration
- Auth: Required (JWT + guild admin permission)
- Body: `{ "channel_id": "...", "mention_role_id": "...", "message_template": {...}, "enabled": true }`
- Response: `{ "message": "Config updated" }`

---

## Default Message Template

When a guild installs the bot, create default config:

```json
{
  "content": "{mention_role} {streamer_display_name} is now live!",
  "embed": {
    "title": "{streamer_display_name} is streaming {game_name}",
    "description": "{stream_title}",
    "url": "https://twitch.tv/{streamer_login}",
    "color": 6570404,
    "thumbnail": {
      "url": "{streamer_avatar_url}"
    },
    "image": {
      "url": "{stream_thumbnail_url}"
    },
    "fields": [
      {
        "name": "Viewers",
        "value": "{viewer_count}",
        "inline": true
      },
      {
        "name": "Game",
        "value": "{game_name}",
        "inline": true
      }
    ],
    "footer": {
      "text": "Twitch Notification"
    },
    "timestamp": true
  }
}
```

---

## Testing Checklist

### Manual Testing
- [ ] Stream goes live → Webhook received
- [ ] Stream data fetched successfully
- [ ] All guilds tracking streamer receive notification
- [ ] Message template renders correctly
- [ ] Role mention works (if configured)
- [ ] Opted-out users excluded (future: verify no mention)
- [ ] Idempotency prevents duplicate notifications
- [ ] Notification logged in database

### Twitch CLI Testing
```bash
# Trigger stream.online event
twitch event trigger stream.online \
  --broadcaster-user-id=123456789 \
  --forward-address=http://localhost:3000/webhooks/twitch
```

### Error Handling
- [ ] Missing channel → Log error, continue to next guild
- [ ] Discord API error → Log error, continue to next guild
- [ ] Missing guild config → Log error, skip guild
- [ ] Template rendering error → Log error, skip guild
- [ ] Database error → Log error, return 500

### Load Testing
- [ ] Test with 10+ guilds tracking same streamer
- [ ] Test concurrent webhook events
- [ ] Test database connection pool under load

---

## Performance Optimization

### Async Processing
- Process fanout asynchronously (goroutine) to return 200 OK to Twitch quickly
- Consider using worker pool for large fanouts (100+ guilds)

### Caching
- Cache guild configs for 5 minutes
- Cache user preferences for 1 minute
- Use Redis for distributed cache (future enhancement)

### Batch Operations
- Batch Discord API calls if rate limited
- Use prepared statements for database queries

---

## Error Scenarios

### Discord Errors

**403 Forbidden (Missing Permissions)**:
- Bot removed from guild or permissions revoked
- Action: Log error, mark guild as needing reconfiguration

**404 Not Found (Channel Deleted)**:
- Notification channel deleted
- Action: Log error, send alert to guild admin

**429 Too Many Requests (Rate Limited)**:
- Discord rate limit hit
- Action: Retry with exponential backoff

### Twitch Errors

**Stream Data Not Found**:
- Stream went offline between event and query
- Action: Log warning, skip notification

**API Rate Limit**:
- Too many concurrent requests
- Action: Retry with backoff

---

## Monitoring

### Metrics to Track
- Notifications sent per hour
- Fanout latency (webhook received → all notifications sent)
- Discord API error rate
- Template rendering errors
- Database query latency

### CloudWatch Logs
```go
log.Printf("[FANOUT] Streamer: %s, Guilds: %d, Duration: %dms", streamerID, len(guilds), duration)
log.Printf("[NOTIF_SENT] Guild: %s, Channel: %s, Event: %s", guildID, channelID, eventID)
log.Printf("[NOTIF_ERROR] Guild: %s, Error: %v", guildID, err)
```

---

## Next Steps

After completing this task:
1. Task 005: Build admin dashboard UI for template editing
2. Task 006: Implement edge case handling (bot removed, channel deleted, etc.)
3. Add CloudWatch alarms for notification failures

---

## Notes

- Always respond 200 OK to Twitch webhook (even if fanout fails)
- Process fanout asynchronously to avoid webhook timeout
- User preference exclusion may require Discord advanced permissions (future enhancement)
- Consider SQS for buffering high-volume notifications (future enhancement)
- Template variables can be extended with custom fields
