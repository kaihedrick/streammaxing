-- StreamMaxing v3 - Rollback Initial Schema
-- Migration: 001 (DOWN)
-- Description: Drop all tables (reverse of 001_initial_schema.sql)

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS notification_log CASCADE;
DROP TABLE IF EXISTS eventsub_subscriptions CASCADE;
DROP TABLE IF EXISTS user_preferences CASCADE;
DROP TABLE IF EXISTS guild_streamers CASCADE;
DROP TABLE IF EXISTS streamers CASCADE;
DROP TABLE IF EXISTS guild_config CASCADE;
DROP TABLE IF EXISTS guilds CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Migration rollback complete
