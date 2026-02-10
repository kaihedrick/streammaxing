

-- StreamMaxing v3 - Migration 002
-- Description: Add user_guilds, invite_links tables and custom_content to guild_streamers

-- User-guild memberships (tracks which users are members of which guilds)
CREATE TABLE IF NOT EXISTS user_guilds (
    user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    guild_id TEXT NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
    is_admin BOOLEAN DEFAULT false,
    updated_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, guild_id)
);

CREATE INDEX IF NOT EXISTS idx_user_guilds_user ON user_guilds(user_id);
CREATE INDEX IF NOT EXISTS idx_user_guilds_guild ON user_guilds(guild_id);

-- Invite links (admin-generated codes for streamers to join)
CREATE TABLE IF NOT EXISTS invite_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id TEXT NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
    code TEXT UNIQUE NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(user_id),
    expires_at TIMESTAMPTZ,            -- NULL = never expires
    max_uses INT DEFAULT 0,            -- 0 = unlimited
    use_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_invite_links_code ON invite_links(code);
CREATE INDEX IF NOT EXISTS idx_invite_links_guild ON invite_links(guild_id);

-- Per-streamer custom notification text on guild_streamers
ALTER TABLE guild_streamers
    ADD COLUMN IF NOT EXISTS custom_content TEXT;

-- Migration complete
