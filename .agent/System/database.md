# Database Schema

## Overview

StreamMaxing uses Neon Postgres (serverless) with a relational schema designed for multi-server support, user preferences, and efficient querying.

---

## Entity Relationship Diagram

```
users (Discord users)
  └─< user_preferences (per-user notification settings)
      └─> guilds (Discord servers)
      └─> streamers (Twitch streamers)

guilds (Discord servers)
  ├─< guild_config (notification settings per server)
  ├─< guild_streamers (which streamers per server)
  │   └─> streamers
  └─< notification_log (notification history)

streamers (Twitch streamers)
  ├─< guild_streamers
  ├─< eventsub_subscriptions (Twitch EventSub)
  ├─< user_preferences
  └─< notification_log
```

---

## Tables

### users

Stores Discord users (admins and regular users).

```sql
CREATE TABLE users (
    user_id TEXT PRIMARY KEY,              -- Discord user ID
    username TEXT NOT NULL,                -- Discord username
    avatar TEXT,                           -- Discord avatar hash
    created_at TIMESTAMPTZ DEFAULT now(),  -- Account creation
    last_login TIMESTAMPTZ DEFAULT now()   -- Last login timestamp
);
```

**Indexes**: None (primary key is sufficient)

**Notes**:
- `user_id` is Discord user ID (string, not UUID)
- `avatar` is hash, full URL constructed as: `https://cdn.discordapp.com/avatars/{user_id}/{avatar}.png`
- Updated on each login

---

### guilds

Stores Discord servers (guilds) where the bot is installed.

```sql
CREATE TABLE guilds (
    guild_id TEXT PRIMARY KEY,             -- Discord guild ID
    name TEXT NOT NULL,                    -- Guild name
    icon TEXT,                             -- Guild icon hash
    owner_id TEXT,                         -- Discord user ID of owner
    created_at TIMESTAMPTZ DEFAULT now()   -- Bot installation timestamp
);
```

**Indexes**: None (primary key is sufficient)

**Notes**:
- `guild_id` is Discord guild ID
- `owner_id` can be NULL (optional)
- Guild data fetched from Discord API on first bot install

---

### guild_config

Per-server notification configuration (one row per guild).

```sql
CREATE TABLE guild_config (
    guild_id TEXT PRIMARY KEY REFERENCES guilds(guild_id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL,              -- Discord channel ID for notifications
    mention_role_id TEXT,                  -- Discord role ID to mention (optional)
    message_template JSONB NOT NULL,       -- Advanced message template (embed, fields, etc.)
    enabled BOOLEAN DEFAULT true,          -- Master toggle for guild notifications
    updated_at TIMESTAMPTZ DEFAULT now()   -- Last config update
);
```

**Indexes**: None (primary key is sufficient)

**Default Template**:
```json
{
  "content": "{streamer_display_name} is now live!",
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

**Template Variables**:
- `{streamer_login}` - Twitch username
- `{streamer_display_name}` - Display name
- `{streamer_avatar_url}` - Profile picture URL
- `{stream_title}` - Stream title
- `{game_name}` - Game being played
- `{viewer_count}` - Current viewer count
- `{stream_thumbnail_url}` - Stream preview image URL
- `{started_at}` - ISO timestamp
- `{mention_role}` - Rendered role mention (e.g., `@Streamers`)

**Notes**:
- CASCADE delete when guild is deleted
- `mention_role_id` is optional (no mention if NULL)
- `enabled` allows disabling all notifications for a guild

---

### streamers

Stores Twitch streamers tracked by the system.

```sql
CREATE TABLE streamers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    twitch_broadcaster_id TEXT UNIQUE NOT NULL,  -- Twitch user ID
    twitch_login TEXT NOT NULL,                  -- Twitch username (lowercase)
    twitch_display_name TEXT,                    -- Display name
    twitch_avatar_url TEXT,                      -- Profile picture URL
    twitch_access_token TEXT,                    -- OAuth access token (encrypted)
    twitch_refresh_token TEXT,                   -- OAuth refresh token (encrypted)
    created_at TIMESTAMPTZ DEFAULT now(),        -- Streamer linked timestamp
    last_updated TIMESTAMPTZ DEFAULT now()       -- Last token refresh
);
```

**Indexes**:
- `UNIQUE (twitch_broadcaster_id)` - Prevent duplicate streamers

**Notes**:
- `id` is UUID (internal identifier)
- `twitch_broadcaster_id` is Twitch user ID (string)
- Access/refresh tokens stored for future use (e.g., fetching stream data)
- **Future**: Encrypt tokens using AWS KMS

---

### guild_streamers

Junction table linking guilds to streamers (many-to-many).

```sql
CREATE TABLE guild_streamers (
    guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    enabled BOOLEAN DEFAULT true,           -- Toggle notifications for this streamer in this guild
    added_by TEXT REFERENCES users(user_id), -- User who linked the streamer
    added_at TIMESTAMPTZ DEFAULT now(),     -- Link timestamp
    PRIMARY KEY (guild_id, streamer_id)
);

CREATE INDEX idx_guild_streamers_guild ON guild_streamers(guild_id);
CREATE INDEX idx_guild_streamers_streamer ON guild_streamers(streamer_id);
```

**Indexes**:
- `idx_guild_streamers_guild` - Fast lookup of streamers for a guild
- `idx_guild_streamers_streamer` - Fast lookup of guilds tracking a streamer

**Notes**:
- CASCADE delete when guild or streamer is deleted
- `enabled` allows disabling a specific streamer in a guild without unlinking
- Same streamer can be linked to multiple guilds

---

### user_preferences

Per-user notification preferences (mute specific streamers).

```sql
CREATE TABLE user_preferences (
    user_id TEXT REFERENCES users(user_id) ON DELETE CASCADE,
    guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    notifications_enabled BOOLEAN DEFAULT true,  -- Toggle for this user+guild+streamer
    created_at TIMESTAMPTZ DEFAULT now(),        -- Preference creation
    updated_at TIMESTAMPTZ DEFAULT now(),        -- Last update
    PRIMARY KEY (user_id, guild_id, streamer_id)
);

CREATE INDEX idx_user_preferences_user ON user_preferences(user_id);
CREATE INDEX idx_user_preferences_guild ON user_preferences(guild_id);
```

**Indexes**:
- `idx_user_preferences_user` - Fast lookup of all preferences for a user
- `idx_user_preferences_guild` - Fast lookup of preferences in a guild

**Notes**:
- CASCADE delete when user, guild, or streamer is deleted
- Default is `true` (notifications enabled)
- Row exists only if user has explicitly set a preference

**Query Pattern**:
```sql
-- Get users who disabled notifications for streamer X in guild Y
SELECT user_id
FROM user_preferences
WHERE guild_id = $1
  AND streamer_id = $2
  AND notifications_enabled = false;
```

---

### eventsub_subscriptions

Tracks Twitch EventSub subscriptions.

```sql
CREATE TABLE eventsub_subscriptions (
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    subscription_id TEXT UNIQUE NOT NULL,   -- Twitch subscription ID
    status TEXT NOT NULL,                   -- enabled, failed, etc.
    created_at TIMESTAMPTZ DEFAULT now(),   -- Subscription creation
    last_verified TIMESTAMPTZ DEFAULT now() -- Last status check
);
```

**Indexes**:
- `UNIQUE (subscription_id)` - Prevent duplicate subscriptions

**Notes**:
- CASCADE delete when streamer is deleted
- `status` values: `pending`, `enabled`, `failed`, `webhook_callback_verification_failed`
- Updated by cleanup job to match Twitch API state

---

### notification_log

Tracks sent notifications for idempotency and auditing.

```sql
CREATE TABLE notification_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,                 -- Twitch event ID
    sent_at TIMESTAMPTZ DEFAULT now(),      -- Notification sent timestamp
    UNIQUE(guild_id, event_id)              -- Prevent duplicate notifications
);

CREATE INDEX idx_notification_log_event ON notification_log(event_id);
```

**Indexes**:
- `UNIQUE (guild_id, event_id)` - Idempotency constraint
- `idx_notification_log_event` - Fast lookup by event ID

**Notes**:
- CASCADE delete when guild or streamer is deleted
- `event_id` is from Twitch EventSub payload (`event.id`)
- Prevents sending duplicate notifications if Twitch sends duplicate webhooks

**Idempotency Check**:
```sql
INSERT INTO notification_log (guild_id, streamer_id, event_id)
VALUES ($1, $2, $3)
ON CONFLICT (guild_id, event_id) DO NOTHING
RETURNING id;

-- If no rows returned, notification already sent
```

---

## Common Queries

### Get guilds tracking a streamer

```sql
SELECT g.guild_id, g.name, gc.channel_id, gc.message_template
FROM guilds g
JOIN guild_streamers gs ON g.guild_id = gs.guild_id
JOIN guild_config gc ON g.guild_id = gc.guild_id
WHERE gs.streamer_id = $1
  AND gs.enabled = true
  AND gc.enabled = true;
```

### Get users who opted out of notifications

```sql
SELECT user_id
FROM user_preferences
WHERE guild_id = $1
  AND streamer_id = $2
  AND notifications_enabled = false;
```

### Get user's guilds with notification preferences

```sql
SELECT g.guild_id, g.name, s.twitch_display_name,
       COALESCE(up.notifications_enabled, true) AS enabled
FROM guilds g
JOIN guild_streamers gs ON g.guild_id = gs.guild_id
JOIN streamers s ON gs.streamer_id = s.id
LEFT JOIN user_preferences up ON up.guild_id = g.guild_id
                              AND up.streamer_id = s.id
                              AND up.user_id = $1
WHERE g.guild_id IN (SELECT guild_id FROM user_guilds WHERE user_id = $1)
ORDER BY g.name, s.twitch_display_name;
```

*Note*: `user_guilds` is not a real table; guilds are fetched from Discord API.

### Cleanup orphaned streamers

```sql
DELETE FROM streamers
WHERE id NOT IN (SELECT DISTINCT streamer_id FROM guild_streamers);
```

### Check for duplicate notification

```sql
SELECT 1 FROM notification_log
WHERE guild_id = $1 AND event_id = $2
LIMIT 1;
```

---

## Migrations

### Migration Strategy

**File Naming**: `XXX_description.sql` (e.g., `001_initial_schema.sql`)

**Execution**: Manual via `psql` or automated via migration tool (e.g., `golang-migrate`)

**Rollback**: Create `XXX_description.down.sql` for each migration (optional)

### Migration 001: Initial Schema

**File**: `backend/migrations/001_initial_schema.sql`

Contains:
- CREATE TABLE statements for all tables
- CREATE INDEX statements
- Default data (none for initial version)

**Execution**:
```bash
psql $DATABASE_URL -f migrations/001_initial_schema.sql
```

---

## Database Configuration

### Connection String

**Format**:
```
postgresql://username:password@host:port/database?sslmode=require
```

**Example** (Neon):
```
postgresql://user:pass@ep-cool-name-12345.us-east-2.aws.neon.tech/neondb?sslmode=require
```

### Connection Pooling

**Neon Serverless Driver**: Uses HTTP-based connection (no traditional pooling)

**Alternative** (pgx with connection pool):
```go
config, _ := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
config.MaxConns = 10
config.MinConns = 2
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = time.Minute * 30

pool, _ := pgxpool.NewWithConfig(context.Background(), config)
```

**Lambda Best Practice**: Use Neon's serverless driver or configure short idle timeout

---

## Data Retention

### Current Policy
- No automatic deletion (data retained indefinitely)
- `notification_log` grows unbounded (acceptable for now)

### Future Enhancements
- Delete `notification_log` entries older than 30 days (cron job)
- Archive inactive guilds (no notifications in 90 days)
- GDPR compliance: User data deletion API endpoint

---

## Backup & Recovery

### Neon Backups
- Automatic daily backups (retention: 7 days on free tier)
- Point-in-time recovery (last 7 days)
- Manual snapshots via Neon dashboard

### Disaster Recovery
1. Export schema: `pg_dump --schema-only $DATABASE_URL > schema.sql`
2. Export data: `pg_dump --data-only $DATABASE_URL > data.sql`
3. Restore: `psql $NEW_DATABASE_URL < schema.sql && psql $NEW_DATABASE_URL < data.sql`

---

## Performance Optimization

### Indexes
- All foreign keys indexed for fast joins
- Unique constraints double as indexes
- Composite index on `(guild_id, event_id)` for idempotency check

### Query Optimization
- Use `LIMIT` to prevent full table scans
- Avoid `SELECT *` (select only needed columns)
- Use `EXPLAIN ANALYZE` to identify slow queries

### Monitoring
- Track slow queries (> 100ms)
- Monitor connection pool usage
- Alert on connection pool exhaustion

---

## Security

### Encryption
- Data at rest: Neon encrypts storage by default
- Data in transit: SSL/TLS required (`sslmode=require`)
- Future: Encrypt Twitch tokens using AWS KMS

### Access Control
- Database credentials in environment variables
- No public database access (Neon IP allowlist optional)
- Application-level access control (no direct DB access for users)

### SQL Injection Prevention
- Parameterized queries only (using pgx)
- Never concatenate user input into SQL

**Good**:
```go
db.Query("SELECT * FROM users WHERE user_id = $1", userID)
```

**Bad**:
```go
db.Query(fmt.Sprintf("SELECT * FROM users WHERE user_id = '%s'", userID))
```
