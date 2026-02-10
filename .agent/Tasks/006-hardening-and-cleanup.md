# Task 006: Hardening and Cleanup

## Status
Complete

## Overview
Implement comprehensive edge case handling, cleanup logic, error recovery, and production hardening for all components of the system.

---

## Goals
1. Handle bot removal from guilds
2. Handle channel/role deletion
3. Handle streamer disconnection
4. Clean up orphaned database records
5. Implement EventSub subscription health checks
6. Handle Discord API rate limiting
7. Handle Twitch token expiration
8. Implement retry logic with exponential backoff
9. Add comprehensive logging and monitoring
10. Implement cost guards and resource limits

---

## Prerequisites
- All previous tasks (001-005) completed
- System running end-to-end
- CloudWatch access configured
- Database access for cleanup jobs

---

## Edge Cases to Handle

### 1. Bot Removed from Guild

**Trigger**: Admin removes bot from Discord server

**Impact**:
- Guild still in database
- EventSub subscriptions still active
- Notifications fail with 403 Forbidden

**Solution**:

**File**: `backend/internal/handlers/cleanup.go`

```go
package handlers

import (
    "context"
    "log"

    "your-module/internal/db"
    "your-module/internal/services/discord"
    "your-module/internal/services/twitch"
)

type CleanupHandler struct {
    discordAPI *discord.APIClient
    twitchAPI  *twitch.EventSubService
    guildDB    *db.GuildDB
    streamerDB *db.StreamerDB
    subDB      *db.SubscriptionDB
}

func (h *CleanupHandler) HandleBotRemoved(guildID string) error {
    log.Printf("Bot removed from guild: %s", guildID)

    // Get all streamers linked to this guild
    streamers, err := h.streamerDB.GetGuildStreamers(guildID)
    if err != nil {
        return err
    }

    // Check if any other guilds are tracking these streamers
    for _, streamer := range streamers {
        guilds, _ := h.streamerDB.GetGuildsTrackingStreamer(streamer.ID)
        if len(guilds) == 1 && guilds[0].GuildID == guildID {
            // This guild was the only one tracking this streamer
            // Delete EventSub subscription
            sub, err := h.subDB.GetSubscriptionByStreamerID(streamer.ID)
            if err == nil {
                h.twitchAPI.DeleteSubscription(sub.SubscriptionID)
                h.subDB.DeleteSubscription(sub.SubscriptionID)
            }

            // Delete streamer (CASCADE will delete guild_streamers)
            h.streamerDB.DeleteStreamer(streamer.ID)
        }
    }

    // Delete guild (CASCADE will delete guild_config, guild_streamers, user_preferences, notification_log)
    if err := h.guildDB.DeleteGuild(guildID); err != nil {
        return err
    }

    log.Printf("Cleanup completed for guild: %s", guildID)
    return nil
}
```

**API Endpoint**:
```go
// POST /api/webhooks/discord (optional, if using Discord webhooks)
func (h *WebhookHandler) HandleDiscordWebhook(w http.ResponseWriter, r *http.Request) {
    // Parse Discord webhook payload
    var payload struct {
        Type    string `json:"t"`
        GuildID string `json:"guild_id"`
    }
    json.NewDecoder(r.Body).Decode(&payload)

    if payload.Type == "GUILD_DELETE" {
        h.cleanup.HandleBotRemoved(payload.GuildID)
    }

    w.WriteHeader(http.StatusOK)
}
```

**Alternative (Lazy Cleanup)**:
Check bot access on each notification attempt. If 403 Forbidden, mark guild for cleanup.

```go
func (s *FanoutService) sendNotificationToGuild(...) error {
    // ... existing code ...

    if err := s.discordAPI.SendMessage(config.ChannelID, message, optedOutUsers); err != nil {
        if strings.Contains(err.Error(), "403") {
            // Bot removed or missing permissions
            log.Printf("Bot removed from guild %s, marking for cleanup", guildID)
            s.guildDB.MarkGuildForCleanup(guildID)
        }
        return err
    }

    // ... rest of code ...
}
```

---

### 2. Notification Channel Deleted

**Trigger**: Admin deletes the configured notification channel

**Impact**:
- Notifications fail with 404 Not Found
- Guild config still references deleted channel

**Solution**:

```go
func (s *FanoutService) sendNotificationToGuild(...) error {
    // ... existing code ...

    if err := s.discordAPI.SendMessage(config.ChannelID, message, optedOutUsers); err != nil {
        if strings.Contains(err.Error(), "404") {
            // Channel deleted
            log.Printf("Channel %s not found in guild %s", config.ChannelID, guildID)

            // Option 1: Disable notifications for guild
            config.Enabled = false
            s.configDB.UpsertGuildConfig(config)

            // Option 2: Send alert to guild owner (requires additional logic)
            // s.alertGuildOwner(guildID, "Notification channel deleted")
        }
        return err
    }

    // ... rest of code ...
}
```

**Frontend**: Show warning if channel is invalid when loading config editor.

---

### 3. Mention Role Deleted

**Trigger**: Admin deletes the configured mention role

**Impact**:
- Role mention in template becomes invalid
- Discord may reject message or ignore mention

**Solution**:

```go
func (s *TemplateService) RenderTemplate(...) (*DiscordMessage, error) {
    // ... existing code ...

    // Check if mention role still exists (requires Discord API call)
    if mentionRoleID != nil && *mentionRoleID != "" {
        exists, _ := s.discordAPI.CheckRoleExists(guildID, *mentionRoleID)
        if !exists {
            // Role deleted, remove mention
            vars["{mention_role}"] = ""
            log.Printf("Mention role %s not found in guild %s", *mentionRoleID, guildID)
        } else {
            vars["{mention_role}"] = fmt.Sprintf("<@&%s>", *mentionRoleID)
        }
    } else {
        vars["{mention_role}"] = ""
    }

    // ... rest of code ...
}
```

---

### 4. Streamer Disconnects/Deletes Account

**Trigger**: Streamer unlinks their Twitch account or deletes account

**Impact**:
- EventSub subscriptions become invalid
- Notifications fail

**Solution**:

**Periodic Health Check** (cron job):

```go
func (h *CleanupHandler) CheckSubscriptionHealth() error {
    log.Println("Running EventSub subscription health check")

    // List all subscriptions from Twitch API
    allSubs, err := h.twitchAPI.ListSubscriptions()
    if err != nil {
        return err
    }

    // Build map of active subscription IDs
    activeSubs := make(map[string]bool)
    for _, sub := range allSubs {
        activeSubs[sub.ID] = true

        // Update status in database
        streamer, err := h.streamerDB.GetStreamerByBroadcasterID(sub.Condition["broadcaster_user_id"].(string))
        if err == nil {
            h.subDB.UpsertSubscription(streamer.ID, sub.ID, sub.Status)
        }
    }

    // Find subscriptions in database not in Twitch API
    dbSubs, _ := h.subDB.GetAllSubscriptions()
    for _, dbSub := range dbSubs {
        if !activeSubs[dbSub.SubscriptionID] {
            log.Printf("Subscription %s not found in Twitch, deleting", dbSub.SubscriptionID)
            h.subDB.DeleteSubscription(dbSub.SubscriptionID)

            // Check if streamer has no other subscriptions
            remaining, _ := h.subDB.GetSubscriptionByStreamerID(dbSub.StreamerID)
            if remaining == nil {
                // Mark streamer as inactive or delete if not linked to any guilds
                guilds, _ := h.streamerDB.GetGuildsTrackingStreamer(dbSub.StreamerID)
                if len(guilds) == 0 {
                    h.streamerDB.DeleteStreamer(dbSub.StreamerID)
                }
            }
        }
    }

    return nil
}
```

**Schedule**: Run every 6 hours via Lambda cron or external scheduler.

---

### 5. Orphaned Streamers

**Trigger**: All guilds unlink a streamer

**Impact**:
- Streamer record remains in database
- EventSub subscription remains active
- Unnecessary costs

**Solution**:

```go
func (h *CleanupHandler) CleanupOrphanedStreamers() error {
    log.Println("Cleaning up orphaned streamers")

    query := `
        SELECT id FROM streamers
        WHERE id NOT IN (SELECT DISTINCT streamer_id FROM guild_streamers)
    `

    rows, err := h.streamerDB.Query(query)
    if err != nil {
        return err
    }
    defer rows.Close()

    for rows.Next() {
        var streamerID uuid.UUID
        rows.Scan(&streamerID)

        // Delete EventSub subscription
        sub, err := h.subDB.GetSubscriptionByStreamerID(streamerID)
        if err == nil {
            h.twitchAPI.DeleteSubscription(sub.SubscriptionID)
            h.subDB.DeleteSubscription(sub.SubscriptionID)
        }

        // Delete streamer
        h.streamerDB.DeleteStreamer(streamerID)
        log.Printf("Deleted orphaned streamer: %s", streamerID)
    }

    return nil
}
```

**Schedule**: Run daily.

---

### 6. User Leaves Guild

**Trigger**: User leaves Discord server

**Impact**:
- User preferences remain in database
- Unnecessary data

**Solution**:

**Lazy Cleanup**: Delete preferences when user tries to access guild they're no longer in.

```go
func (h *PreferencesHandler) GetUserPreferences(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(string)

    // Fetch user's guilds from Discord
    guilds, err := h.discordAPI.GetUserGuilds(userID)
    if err != nil {
        http.Error(w, "Failed to fetch guilds", http.StatusInternalServerError)
        return
    }

    guildIDs := make(map[string]bool)
    for _, guild := range guilds {
        guildIDs[guild.ID] = true
    }

    // Get preferences from database
    prefs, err := h.preferencesDB.GetUserPreferences(userID)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    // Filter out preferences for guilds user is no longer in
    var validPrefs []UserPreference
    for _, pref := range prefs {
        if guildIDs[pref.GuildID] {
            validPrefs = append(validPrefs, pref)
        } else {
            // Delete invalid preference
            h.preferencesDB.DeleteUserPreference(userID, pref.GuildID, pref.StreamerID)
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(validPrefs)
}
```

---

### 7. Discord API Rate Limiting

**Trigger**: Too many concurrent requests to Discord API

**Impact**:
- 429 Too Many Requests errors
- Notification delivery fails

**Solution**:

**Rate Limiter with Retry**:

```go
package discord

import (
    "net/http"
    "strconv"
    "time"
)

type RateLimitedClient struct {
    client      *http.Client
    rateLimiter chan struct{}
}

func NewRateLimitedClient(requestsPerSecond int) *RateLimitedClient {
    limiter := make(chan struct{}, requestsPerSecond)
    go func() {
        ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
        for range ticker.C {
            select {
            case limiter <- struct{}{}:
            default:
            }
        }
    }()

    return &RateLimitedClient{
        client:      &http.Client{Timeout: 10 * time.Second},
        rateLimiter: limiter,
    }
}

func (c *RateLimitedClient) Do(req *http.Request) (*http.Response, error) {
    <-c.rateLimiter // Wait for rate limit slot

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }

    // Handle 429 with retry
    if resp.StatusCode == 429 {
        retryAfter := resp.Header.Get("Retry-After")
        if retryAfter != "" {
            wait, _ := strconv.Atoi(retryAfter)
            log.Printf("Rate limited, retrying after %d seconds", wait)
            time.Sleep(time.Duration(wait) * time.Second)
            return c.Do(req) // Retry
        }
    }

    return resp, nil
}
```

---

### 8. Twitch Token Expiration

**Trigger**: Stored Twitch access tokens expire (after ~4 hours)

**Impact**:
- API calls fail with 401 Unauthorized
- Cannot fetch stream data

**Solution**:

**Automatic Token Refresh**:

```go
func (s *StreamerDB) RefreshTokenIfNeeded(streamerID uuid.UUID) error {
    streamer, err := s.GetStreamer(streamerID)
    if err != nil {
        return err
    }

    // Check if token is expired (tokens expire after 4 hours, refresh proactively)
    if time.Since(streamer.LastUpdated) > 3*time.Hour {
        log.Printf("Refreshing token for streamer %s", streamerID)

        oauth := twitch.NewOAuthService()
        tokenResp, err := oauth.RefreshToken(streamer.RefreshToken)
        if err != nil {
            return fmt.Errorf("failed to refresh token: %w", err)
        }

        // Update tokens in database
        query := `
            UPDATE streamers
            SET twitch_access_token = $1, twitch_refresh_token = $2, last_updated = now()
            WHERE id = $3
        `
        _, err = s.pool.Exec(context.Background(), query, tokenResp.AccessToken, tokenResp.RefreshToken, streamerID)
        return err
    }

    return nil
}
```

**Call before using token**:

```go
func (s *FanoutService) HandleStreamOnline(...) error {
    // Refresh token if needed
    s.streamerDB.RefreshTokenIfNeeded(streamer.ID)

    // Fetch stream data (uses refreshed token)
    streamData, err := s.twitchAPI.GetStreamData(event.BroadcasterUserID)
    // ...
}
```

---

### 9. Notification Log Cleanup

**Trigger**: notification_log table grows too large

**Impact**:
- Increased storage costs
- Slower queries

**Solution**:

**Delete old entries** (keep last 30 days):

```go
func (h *CleanupHandler) CleanupNotificationLog() error {
    log.Println("Cleaning up old notification logs")

    query := `
        DELETE FROM notification_log
        WHERE sent_at < now() - interval '30 days'
    `

    result, err := h.db.Exec(context.Background(), query)
    if err != nil {
        return err
    }

    rowsDeleted := result.RowsAffected()
    log.Printf("Deleted %d old notification log entries", rowsDeleted)
    return nil
}
```

**Schedule**: Run weekly.

---

### 10. Cost Guards

**Trigger**: Unexpected spike in usage

**Impact**:
- AWS bill exceeds budget

**Solution**:

**CloudWatch Billing Alarms**:

```bash
aws cloudwatch put-metric-alarm \
  --alarm-name "StreamMaxing-BillingAlert-10USD" \
  --alarm-description "Alert when bill exceeds $10" \
  --metric-name EstimatedCharges \
  --namespace AWS/Billing \
  --statistic Maximum \
  --period 21600 \
  --evaluation-periods 1 \
  --threshold 10 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=Currency,Value=USD
```

**Lambda Concurrency Limit**:

```bash
aws lambda put-function-concurrency \
  --function-name streammaxing \
  --reserved-concurrent-executions 50
```

**Database Row Limits** (application-level):

```go
const (
    MaxGuildsPerUser     = 100
    MaxStreamersPerGuild = 50
    MaxTotalStreamers    = 1000
)

func (h *GuildHandler) AddStreamer(w http.ResponseWriter, r *http.Request) {
    // Check limits before adding
    totalStreamers, _ := h.streamerDB.CountStreamers()
    if totalStreamers >= MaxTotalStreamers {
        http.Error(w, "Streamer limit reached", http.StatusForbidden)
        return
    }

    // Continue with normal logic...
}
```

---

## Monitoring and Logging

### Structured Logging

```go
type LogEntry struct {
    Timestamp string                 `json:"timestamp"`
    Level     string                 `json:"level"`
    Message   string                 `json:"message"`
    Context   map[string]interface{} `json:"context,omitempty"`
}

func LogInfo(message string, context map[string]interface{}) {
    entry := LogEntry{
        Timestamp: time.Now().Format(time.RFC3339),
        Level:     "INFO",
        Message:   message,
        Context:   context,
    }
    json.NewEncoder(os.Stdout).Encode(entry)
}

func LogError(message string, err error, context map[string]interface{}) {
    if context == nil {
        context = make(map[string]interface{})
    }
    context["error"] = err.Error()

    entry := LogEntry{
        Timestamp: time.Now().Format(time.RFC3339),
        Level:     "ERROR",
        Message:   message,
        Context:   context,
    }
    json.NewEncoder(os.Stdout).Encode(entry)
}
```

**Usage**:

```go
LogInfo("Notification sent", map[string]interface{}{
    "guild_id":    guildID,
    "streamer_id": streamerID,
    "event_id":    eventID,
})

LogError("Failed to send notification", err, map[string]interface{}{
    "guild_id": guildID,
})
```

---

### CloudWatch Metrics

**Custom Metrics**:

```go
func PublishMetric(metricName string, value float64, unit string) {
    svc := cloudwatch.New(session.Must(session.NewSession()))
    _, err := svc.PutMetricData(&cloudwatch.PutMetricDataInput{
        Namespace: aws.String("StreamMaxing"),
        MetricData: []*cloudwatch.MetricDatum{
            {
                MetricName: aws.String(metricName),
                Value:      aws.Float64(value),
                Unit:       aws.String(unit),
                Timestamp:  aws.Time(time.Now()),
            },
        },
    })
    if err != nil {
        log.Printf("Failed to publish metric: %v", err)
    }
}

// Example usage
PublishMetric("NotificationsSent", 1, "Count")
PublishMetric("FanoutLatency", 123.45, "Milliseconds")
```

---

## Cleanup Job Scheduler

**Lambda Function** (triggered by EventBridge cron):

```go
func HandleCleanupEvent(ctx context.Context, event events.CloudWatchEvent) error {
    handler := NewCleanupHandler(/* dependencies */)

    // Run all cleanup tasks
    handler.CleanupOrphanedStreamers()
    handler.CheckSubscriptionHealth()
    handler.CleanupNotificationLog()

    return nil
}
```

**EventBridge Rule**:

```bash
aws events put-rule \
  --name "StreamMaxing-DailyCleanup" \
  --schedule-expression "cron(0 2 * * ? *)" \
  --state ENABLED

aws events put-targets \
  --rule StreamMaxing-DailyCleanup \
  --targets "Id"="1","Arn"="arn:aws:lambda:us-east-1:123456789:function:streammaxing-cleanup"
```

---

## Testing Checklist

### Edge Case Testing
- [ ] Remove bot from guild → Database cleanup triggered
- [ ] Delete notification channel → Notifications disabled or channel updated
- [ ] Delete mention role → Mention removed from template
- [ ] Unlink streamer from all guilds → EventSub subscription deleted
- [ ] User leaves guild → Preferences deleted
- [ ] Twitch token expires → Automatic refresh works
- [ ] Discord rate limit hit → Retry logic works
- [ ] Duplicate webhook event → Idempotency prevents duplicate notification

### Cleanup Job Testing
- [ ] Orphaned streamers deleted
- [ ] Stale subscriptions removed
- [ ] Old notification logs deleted
- [ ] Cost guards prevent runaway usage

### Load Testing
- [ ] 100+ concurrent webhooks handled
- [ ] Database connection pool doesn't exhaust
- [ ] Rate limiter prevents Discord API errors

---

## Production Readiness Checklist

- [ ] All edge cases handled
- [ ] Retry logic with exponential backoff implemented
- [ ] Rate limiting configured
- [ ] CloudWatch alarms set up
- [ ] Database backups configured
- [ ] Secrets stored in AWS Secrets Manager
- [ ] SSL/TLS certificates configured
- [ ] CORS configured correctly
- [ ] Cost guards in place
- [ ] Monitoring dashboard created
- [ ] Incident response plan documented
- [ ] Backup Lambda deployment tested

---

## Next Steps

After completing this task:
1. Load test in production-like environment
2. Monitor for 1 week to identify additional edge cases
3. Document all error codes and recovery procedures
4. Create runbook for common issues

---

## Notes

- Always log errors with context for debugging
- Implement graceful degradation (system continues with reduced functionality)
- Use database transactions for cleanup operations
- Test cleanup logic on copy of production data
- Monitor CloudWatch costs (logging can be expensive)
- Consider using AWS X-Ray for distributed tracing
