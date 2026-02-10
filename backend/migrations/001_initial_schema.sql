-- StreamMaxing v3 - Initial Database Schema
-- Migration: 001
-- Description: Create all tables for multi-server Discord notification system

-- Users table (Discord users)
CREATE TABLE users (
    user_id TEXT PRIMARY KEY,              -- Discord user ID
    username TEXT NOT NULL,                -- Discord username
    avatar TEXT,                           -- Discord avatar hash
    created_at TIMESTAMPTZ DEFAULT now(),  -- Account creation
    last_login TIMESTAMPTZ DEFAULT now()   -- Last login timestamp
);

-- Guilds table (Discord servers)
CREATE TABLE guilds (
    guild_id TEXT PRIMARY KEY,             -- Discord guild ID
    name TEXT NOT NULL,                    -- Guild name
    icon TEXT,                             -- Guild icon hash
    owner_id TEXT,                         -- Discord user ID of owner
    created_at TIMESTAMPTZ DEFAULT now()   -- Bot installation timestamp
);

-- Guild configuration table (per-server notification settings)
CREATE TABLE guild_config (
    guild_id TEXT PRIMARY KEY REFERENCES guilds(guild_id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL,              -- Discord channel ID for notifications
    mention_role_id TEXT,                  -- Discord role ID to mention (optional)
    message_template JSONB NOT NULL,       -- Advanced message template (embed, fields, etc.)
    enabled BOOLEAN DEFAULT true,          -- Master toggle for guild notifications
    updated_at TIMESTAMPTZ DEFAULT now()   -- Last config update
);

-- Streamers table (Twitch streamers)
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

-- Guild-Streamers junction table (which streamers per guild)
CREATE TABLE guild_streamers (
    guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    enabled BOOLEAN DEFAULT true,           -- Toggle notifications for this streamer in this guild
    added_by TEXT REFERENCES users(user_id), -- User who linked the streamer
    added_at TIMESTAMPTZ DEFAULT now(),     -- Link timestamp
    PRIMARY KEY (guild_id, streamer_id)
);

-- User preferences table (per-user notification settings)
CREATE TABLE user_preferences (
    user_id TEXT REFERENCES users(user_id) ON DELETE CASCADE,
    guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    notifications_enabled BOOLEAN DEFAULT true,  -- Toggle for this user+guild+streamer
    created_at TIMESTAMPTZ DEFAULT now(),        -- Preference creation
    updated_at TIMESTAMPTZ DEFAULT now(),        -- Last update
    PRIMARY KEY (user_id, guild_id, streamer_id)
);

-- EventSub subscriptions table (Twitch EventSub)
CREATE TABLE eventsub_subscriptions (
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    subscription_id TEXT UNIQUE NOT NULL,   -- Twitch subscription ID
    status TEXT NOT NULL,                   -- enabled, failed, etc.
    created_at TIMESTAMPTZ DEFAULT now(),   -- Subscription creation
    last_verified TIMESTAMPTZ DEFAULT now() -- Last status check
);

-- Notification log table (idempotency and auditing)
CREATE TABLE notification_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id TEXT REFERENCES guilds(guild_id) ON DELETE CASCADE,
    streamer_id UUID REFERENCES streamers(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,                 -- Twitch event ID
    sent_at TIMESTAMPTZ DEFAULT now(),      -- Notification sent timestamp
    UNIQUE(guild_id, event_id)              -- Prevent duplicate notifications
);

-- Create indexes for performance
CREATE INDEX idx_guild_streamers_guild ON guild_streamers(guild_id);
CREATE INDEX idx_guild_streamers_streamer ON guild_streamers(streamer_id);
CREATE INDEX idx_user_preferences_user ON user_preferences(user_id);
CREATE INDEX idx_user_preferences_guild ON user_preferences(guild_id);
CREATE INDEX idx_notification_log_event ON notification_log(event_id);

-- Insert default message template for new guilds (optional, can be done in application code)
-- Example default template:
-- {
--   "content": "{streamer_display_name} is now live!",
--   "embed": {
--     "title": "{streamer_display_name} is streaming {game_name}",
--     "description": "{stream_title}",
--     "url": "https://twitch.tv/{streamer_login}",
--     "color": 6570404,
--     "thumbnail": { "url": "{streamer_avatar_url}" },
--     "image": { "url": "{stream_thumbnail_url}" },
--     "fields": [
--       { "name": "Viewers", "value": "{viewer_count}", "inline": true },
--       { "name": "Game", "value": "{game_name}", "inline": true }
--     ],
--     "footer": { "text": "Twitch Notification" },
--     "timestamp": true
--   }
-- }

-- Migration complete
