package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// User queries

// CreateOrUpdateUser inserts or updates a user
func CreateOrUpdateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (user_id, username, avatar, last_login)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id)
		DO UPDATE SET username = $2, avatar = $3, last_login = now()
	`
	_, err := Pool.Exec(ctx, query, user.UserID, user.Username, user.Avatar)
	return err
}

// GetUser retrieves a user by ID
func GetUser(ctx context.Context, userID string) (*User, error) {
	query := `SELECT user_id, username, avatar, created_at, last_login FROM users WHERE user_id = $1`
	var user User
	err := Pool.QueryRow(ctx, query, userID).Scan(
		&user.UserID, &user.Username, &user.Avatar, &user.CreatedAt, &user.LastLogin,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Guild queries

// CreateOrUpdateGuild inserts or updates a guild
func CreateOrUpdateGuild(ctx context.Context, guild *Guild) error {
	query := `
		INSERT INTO guilds (guild_id, name, icon, owner_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (guild_id)
		DO UPDATE SET name = $2, icon = $3, owner_id = $4
	`
	_, err := Pool.Exec(ctx, query, guild.GuildID, guild.Name, guild.Icon, guild.OwnerID)
	return err
}

// GetGuild retrieves a guild by ID
func GetGuild(ctx context.Context, guildID string) (*Guild, error) {
	query := `SELECT guild_id, name, icon, owner_id, created_at FROM guilds WHERE guild_id = $1`
	var guild Guild
	err := Pool.QueryRow(ctx, query, guildID).Scan(
		&guild.GuildID, &guild.Name, &guild.Icon, &guild.OwnerID, &guild.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &guild, nil
}

// DeleteGuild deletes a guild (CASCADE deletes related data)
func DeleteGuild(ctx context.Context, guildID string) error {
	query := `DELETE FROM guilds WHERE guild_id = $1`
	_, err := Pool.Exec(ctx, query, guildID)
	return err
}

// GuildConfig queries

// CreateGuildConfig creates default guild configuration
func CreateGuildConfig(ctx context.Context, guildID, channelID string) error {
	defaultTemplate := DefaultMessageTemplate()
	templateJSON, err := json.Marshal(defaultTemplate)
	if err != nil {
		return fmt.Errorf("failed to marshal default template: %w", err)
	}

	query := `
		INSERT INTO guild_config (guild_id, channel_id, message_template)
		VALUES ($1, $2, $3)
		ON CONFLICT (guild_id) DO NOTHING
	`
	_, err = Pool.Exec(ctx, query, guildID, channelID, templateJSON)
	return err
}

// GetGuildConfig retrieves guild configuration, creating a default if none exists
func GetGuildConfig(ctx context.Context, guildID string) (*GuildConfig, error) {
	query := `
		SELECT guild_id, channel_id, mention_role_id, message_template, enabled, updated_at
		FROM guild_config
		WHERE guild_id = $1
	`
	var config GuildConfig
	var mentionRoleID *string
	err := Pool.QueryRow(ctx, query, guildID).Scan(
		&config.GuildID, &config.ChannelID, &mentionRoleID,
		&config.MessageTemplate, &config.Enabled, &config.UpdatedAt,
	)
	if err != nil {
		// If no config exists yet, create a default one
		if err.Error() == "no rows in result set" {
			if createErr := CreateGuildConfig(ctx, guildID, ""); createErr != nil {
				return nil, fmt.Errorf("failed to create default config: %w", createErr)
			}
			// Re-fetch after creation
			err = Pool.QueryRow(ctx, query, guildID).Scan(
				&config.GuildID, &config.ChannelID, &mentionRoleID,
				&config.MessageTemplate, &config.Enabled, &config.UpdatedAt,
			)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	if mentionRoleID != nil {
		config.MentionRoleID = *mentionRoleID
	}
	return &config, nil
}

// UpdateGuildConfig updates guild configuration
func UpdateGuildConfig(ctx context.Context, config *GuildConfig) error {
	query := `
		UPDATE guild_config
		SET channel_id = $2, mention_role_id = $3, message_template = $4, enabled = $5, updated_at = now()
		WHERE guild_id = $1
	`
	mentionRoleID := nullableString(config.MentionRoleID)
	_, err := Pool.Exec(ctx, query, config.GuildID, config.ChannelID, mentionRoleID, config.MessageTemplate, config.Enabled)
	return err
}

// Streamer queries

// CreateStreamer inserts a new streamer
func CreateStreamer(ctx context.Context, streamer *Streamer) error {
	query := `
		INSERT INTO streamers (twitch_broadcaster_id, twitch_login, twitch_display_name, twitch_avatar_url, twitch_access_token, twitch_refresh_token)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (twitch_broadcaster_id)
		DO UPDATE SET twitch_login = $2, twitch_display_name = $3, twitch_avatar_url = $4, twitch_access_token = $5, twitch_refresh_token = $6, last_updated = now()
		RETURNING id
	`
	return Pool.QueryRow(ctx, query,
		streamer.TwitchBroadcasterID, streamer.TwitchLogin, streamer.TwitchDisplayName,
		streamer.TwitchAvatarURL, streamer.TwitchAccessToken, streamer.TwitchRefreshToken,
	).Scan(&streamer.ID)
}

// GetStreamerByID retrieves a streamer by internal ID
func GetStreamerByID(ctx context.Context, id string) (*Streamer, error) {
	query := `
		SELECT id, twitch_broadcaster_id, twitch_login, twitch_display_name, twitch_avatar_url, created_at, last_updated
		FROM streamers
		WHERE id = $1
	`
	var streamer Streamer
	err := Pool.QueryRow(ctx, query, id).Scan(
		&streamer.ID, &streamer.TwitchBroadcasterID, &streamer.TwitchLogin,
		&streamer.TwitchDisplayName, &streamer.TwitchAvatarURL,
		&streamer.CreatedAt, &streamer.LastUpdated,
	)
	if err != nil {
		return nil, err
	}
	return &streamer, nil
}

// GetStreamerByBroadcasterID retrieves a streamer by Twitch broadcaster ID
func GetStreamerByBroadcasterID(ctx context.Context, broadcasterID string) (*Streamer, error) {
	query := `
		SELECT id, twitch_broadcaster_id, twitch_login, twitch_display_name, twitch_avatar_url, created_at, last_updated
		FROM streamers
		WHERE twitch_broadcaster_id = $1
	`
	var streamer Streamer
	err := Pool.QueryRow(ctx, query, broadcasterID).Scan(
		&streamer.ID, &streamer.TwitchBroadcasterID, &streamer.TwitchLogin,
		&streamer.TwitchDisplayName, &streamer.TwitchAvatarURL,
		&streamer.CreatedAt, &streamer.LastUpdated,
	)
	if err != nil {
		return nil, err
	}
	return &streamer, nil
}

// GetGuildStreamers retrieves all streamers for a guild
func GetGuildStreamers(ctx context.Context, guildID string) ([]Streamer, error) {
	query := `
		SELECT s.id, s.twitch_broadcaster_id, s.twitch_login, s.twitch_display_name, s.twitch_avatar_url, s.created_at, s.last_updated
		FROM streamers s
		JOIN guild_streamers gs ON s.id = gs.streamer_id
		WHERE gs.guild_id = $1
		ORDER BY s.twitch_display_name
	`
	rows, err := Pool.Query(ctx, query, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streamers []Streamer
	for rows.Next() {
		var s Streamer
		err := rows.Scan(&s.ID, &s.TwitchBroadcasterID, &s.TwitchLogin, &s.TwitchDisplayName, &s.TwitchAvatarURL, &s.CreatedAt, &s.LastUpdated)
		if err != nil {
			return nil, err
		}
		streamers = append(streamers, s)
	}
	return streamers, rows.Err()
}

// LinkStreamerToGuild links a streamer to a guild.
// Returns (true, nil) if a new link was created, (false, nil) if the link already existed.
func LinkStreamerToGuild(ctx context.Context, guildID, streamerID, addedBy string) (bool, error) {
	query := `
		INSERT INTO guild_streamers (guild_id, streamer_id, added_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (guild_id, streamer_id) DO NOTHING
	`
	result, err := Pool.Exec(ctx, query, guildID, streamerID, addedBy)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

// UnlinkStreamerFromGuild removes a streamer from a guild
func UnlinkStreamerFromGuild(ctx context.Context, guildID, streamerID string) error {
	query := `DELETE FROM guild_streamers WHERE guild_id = $1 AND streamer_id = $2`
	_, err := Pool.Exec(ctx, query, guildID, streamerID)
	return err
}

// GetGuildsTrackingStreamer retrieves all guilds tracking a specific streamer
func GetGuildsTrackingStreamer(ctx context.Context, streamerID string) ([]string, error) {
	query := `
		SELECT guild_id FROM guild_streamers
		WHERE streamer_id = $1 AND enabled = true
	`
	rows, err := Pool.Query(ctx, query, streamerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guildIDs []string
	for rows.Next() {
		var guildID string
		if err := rows.Scan(&guildID); err != nil {
			return nil, err
		}
		guildIDs = append(guildIDs, guildID)
	}
	return guildIDs, rows.Err()
}

// NotificationLog queries

// CheckNotificationSent checks if a notification has already been sent
func CheckNotificationSent(ctx context.Context, guildID, eventID string) (bool, error) {
	query := `SELECT 1 FROM notification_log WHERE guild_id = $1 AND event_id = $2 LIMIT 1`
	var exists int
	err := Pool.QueryRow(ctx, query, guildID, eventID).Scan(&exists)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// LogNotification logs a sent notification (idempotency)
func LogNotification(ctx context.Context, guildID, streamerID, eventID string) error {
	query := `
		INSERT INTO notification_log (guild_id, streamer_id, event_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (guild_id, event_id) DO NOTHING
	`
	_, err := Pool.Exec(ctx, query, guildID, streamerID, eventID)
	return err
}

// TryClaimNotification atomically claims the right to send a notification.
// Returns true if we inserted a new row (we should send), false if another
// Lambda instance already claimed it (skip sending).
// This eliminates the TOCTOU race between CheckNotificationSent and LogNotification
// that caused duplicate Discord messages.
func TryClaimNotification(ctx context.Context, guildID, streamerID, eventID string) (bool, error) {
	query := `
		INSERT INTO notification_log (guild_id, streamer_id, event_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (guild_id, event_id) DO NOTHING
		RETURNING id
	`
	var id string
	err := Pool.QueryRow(ctx, query, guildID, streamerID, eventID).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Conflict: another instance already claimed this notification
			return false, nil
		}
		return false, err
	}
	// We inserted: we own this notification
	return true, nil
}

// EventSub subscription queries

// CreateEventSubSubscription creates or updates an EventSub subscription record
func CreateEventSubSubscription(ctx context.Context, streamerID, subscriptionID, status string) error {
	query := `
		INSERT INTO eventsub_subscriptions (streamer_id, subscription_id, status, last_verified)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (subscription_id)
		DO UPDATE SET status = $3, last_verified = now()
	`
	_, err := Pool.Exec(ctx, query, streamerID, subscriptionID, status)
	return err
}

// GetEventSubSubscription retrieves a subscription by streamer ID
func GetEventSubSubscription(ctx context.Context, streamerID string) (*EventSubSubscription, error) {
	query := `
		SELECT streamer_id, subscription_id, status, created_at, last_verified
		FROM eventsub_subscriptions
		WHERE streamer_id = $1
	`
	var sub EventSubSubscription
	err := Pool.QueryRow(ctx, query, streamerID).Scan(
		&sub.StreamerID, &sub.SubscriptionID, &sub.Status, &sub.CreatedAt, &sub.LastVerified,
	)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// DeleteEventSubSubscription deletes a subscription record
func DeleteEventSubSubscription(ctx context.Context, subscriptionID string) error {
	query := `DELETE FROM eventsub_subscriptions WHERE subscription_id = $1`
	_, err := Pool.Exec(ctx, query, subscriptionID)
	return err
}

// User preference queries

// GetUserPreferences retrieves all notification preferences for a user
func GetUserPreferences(ctx context.Context, userID string) ([]UserPreference, error) {
	query := `
		SELECT up.user_id, up.guild_id, up.streamer_id, up.notifications_enabled, up.created_at, up.updated_at
		FROM user_preferences up
		WHERE up.user_id = $1
		ORDER BY up.guild_id, up.streamer_id
	`
	rows, err := Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []UserPreference
	for rows.Next() {
		var p UserPreference
		if err := rows.Scan(&p.UserID, &p.GuildID, &p.StreamerID, &p.NotificationsEnabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// SetUserPreference creates or updates a user notification preference
func SetUserPreference(ctx context.Context, userID, guildID, streamerID string, enabled bool) error {
	query := `
		INSERT INTO user_preferences (user_id, guild_id, streamer_id, notifications_enabled)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, guild_id, streamer_id)
		DO UPDATE SET notifications_enabled = $4, updated_at = now()
	`
	_, err := Pool.Exec(ctx, query, userID, guildID, streamerID, enabled)
	return err
}

// GetOptedOutUsers retrieves users who have opted out of notifications for a streamer in a guild
func GetOptedOutUsers(ctx context.Context, guildID, streamerID string) ([]string, error) {
	query := `
		SELECT user_id FROM user_preferences
		WHERE guild_id = $1 AND streamer_id = $2 AND notifications_enabled = false
	`
	rows, err := Pool.Query(ctx, query, guildID, streamerID)
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
	return userIDs, rows.Err()
}

// DeleteStreamer deletes a streamer (CASCADE deletes related data)
func DeleteStreamer(ctx context.Context, streamerID string) error {
	query := `DELETE FROM streamers WHERE id = $1`
	_, err := Pool.Exec(ctx, query, streamerID)
	return err
}

// CleanupOldNotificationLogs deletes notification logs older than 30 days
func CleanupOldNotificationLogs(ctx context.Context) (int64, error) {
	query := `DELETE FROM notification_log WHERE sent_at < now() - interval '30 days'`
	result, err := Pool.Exec(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// GetOrphanedStreamers returns streamer IDs not linked to any guilds
func GetOrphanedStreamers(ctx context.Context) ([]string, error) {
	query := `
		SELECT id::text FROM streamers
		WHERE id NOT IN (SELECT DISTINCT streamer_id FROM guild_streamers)
	`
	rows, err := Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// =====================
// User-Guild membership queries
// =====================

// UpsertUserGuild inserts or updates a user-guild membership
func UpsertUserGuild(ctx context.Context, userID, guildID string, isAdmin bool) error {
	query := `
		INSERT INTO user_guilds (user_id, guild_id, is_admin, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id, guild_id)
		DO UPDATE SET is_admin = $3, updated_at = now()
	`
	_, err := Pool.Exec(ctx, query, userID, guildID, isAdmin)
	return err
}

// GetUserGuildsForUser returns all guilds a user is a member of, with admin flag
func GetUserGuildsForUser(ctx context.Context, userID string) ([]GuildWithRole, error) {
	query := `
		SELECT g.guild_id, g.name, g.icon, g.owner_id, g.created_at, ug.is_admin
		FROM guilds g
		JOIN user_guilds ug ON g.guild_id = ug.guild_id
		WHERE ug.user_id = $1
		ORDER BY g.name
	`
	rows, err := Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guilds []GuildWithRole
	for rows.Next() {
		var gwr GuildWithRole
		if err := rows.Scan(&gwr.GuildID, &gwr.Name, &gwr.Icon, &gwr.OwnerID, &gwr.CreatedAt, &gwr.IsAdmin); err != nil {
			return nil, err
		}
		guilds = append(guilds, gwr)
	}
	return guilds, rows.Err()
}

// IsUserGuildAdmin checks if a user is an admin of a guild
func IsUserGuildAdmin(ctx context.Context, userID, guildID string) (bool, error) {
	query := `SELECT is_admin FROM user_guilds WHERE user_id = $1 AND guild_id = $2`
	var isAdmin bool
	err := Pool.QueryRow(ctx, query, userID, guildID).Scan(&isAdmin)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return isAdmin, nil
}

// IsUserGuildMember checks if a user is a member of a guild
func IsUserGuildMember(ctx context.Context, userID, guildID string) (bool, error) {
	query := `SELECT 1 FROM user_guilds WHERE user_id = $1 AND guild_id = $2`
	var exists int
	err := Pool.QueryRow(ctx, query, userID, guildID).Scan(&exists)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetAllGuildIDs returns all guild IDs in our database (for cross-referencing)
func GetAllGuildIDs(ctx context.Context) (map[string]bool, error) {
	query := `SELECT guild_id FROM guilds`
	rows, err := Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// =====================
// Invite link queries
// =====================

// generateInviteCode creates a random 8-character invite code
func generateInviteCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateInviteLink creates a new invite link for a guild
func CreateInviteLink(ctx context.Context, guildID, createdBy string, expiresAt *time.Time, maxUses int) (*InviteLink, error) {
	code := generateInviteCode()
	query := `
		INSERT INTO invite_links (guild_id, code, created_by, expires_at, max_uses)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, guild_id, code, created_by, expires_at, max_uses, use_count, created_at
	`
	var link InviteLink
	err := Pool.QueryRow(ctx, query, guildID, code, createdBy, expiresAt, maxUses).Scan(
		&link.ID, &link.GuildID, &link.Code, &link.CreatedBy,
		&link.ExpiresAt, &link.MaxUses, &link.UseCount, &link.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// GetInviteLink retrieves an invite link by code (validates not expired/exhausted)
func GetInviteLink(ctx context.Context, code string) (*InviteLink, error) {
	query := `
		SELECT id, guild_id, code, created_by, expires_at, max_uses, use_count, created_at
		FROM invite_links
		WHERE code = $1
	`
	var link InviteLink
	err := Pool.QueryRow(ctx, query, code).Scan(
		&link.ID, &link.GuildID, &link.Code, &link.CreatedBy,
		&link.ExpiresAt, &link.MaxUses, &link.UseCount, &link.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// IncrementInviteUse increments the use count of an invite link
func IncrementInviteUse(ctx context.Context, code string) error {
	query := `UPDATE invite_links SET use_count = use_count + 1 WHERE code = $1`
	_, err := Pool.Exec(ctx, query, code)
	return err
}

// GetGuildInviteLinks returns all invite links for a guild
func GetGuildInviteLinks(ctx context.Context, guildID string) ([]InviteLink, error) {
	query := `
		SELECT id, guild_id, code, created_by, expires_at, max_uses, use_count, created_at
		FROM invite_links
		WHERE guild_id = $1
		ORDER BY created_at DESC
	`
	rows, err := Pool.Query(ctx, query, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []InviteLink
	for rows.Next() {
		var link InviteLink
		if err := rows.Scan(
			&link.ID, &link.GuildID, &link.Code, &link.CreatedBy,
			&link.ExpiresAt, &link.MaxUses, &link.UseCount, &link.CreatedAt,
		); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

// DeleteInviteLink deletes an invite link
func DeleteInviteLink(ctx context.Context, id string) error {
	query := `DELETE FROM invite_links WHERE id = $1`
	_, err := Pool.Exec(ctx, query, id)
	return err
}

// =====================
// Custom content queries
// =====================

// GetStreamerCustomContent retrieves the custom notification text for a streamer in a guild
func GetStreamerCustomContent(ctx context.Context, guildID, streamerID string) (string, error) {
	query := `SELECT COALESCE(custom_content, '') FROM guild_streamers WHERE guild_id = $1 AND streamer_id = $2`
	var content string
	err := Pool.QueryRow(ctx, query, guildID, streamerID).Scan(&content)
	if err != nil {
		return "", err
	}
	return content, nil
}

// UpdateStreamerCustomContent updates the custom notification text
func UpdateStreamerCustomContent(ctx context.Context, guildID, streamerID, content string) error {
	query := `UPDATE guild_streamers SET custom_content = $3 WHERE guild_id = $1 AND streamer_id = $2`
	c := nullableString(content)
	_, err := Pool.Exec(ctx, query, guildID, streamerID, c)
	return err
}

// GetGuildStreamerAddedBy retrieves who added a streamer to a guild
func GetGuildStreamerAddedBy(ctx context.Context, guildID, streamerID string) (string, error) {
	query := `SELECT COALESCE(added_by, '') FROM guild_streamers WHERE guild_id = $1 AND streamer_id = $2`
	var addedBy string
	err := Pool.QueryRow(ctx, query, guildID, streamerID).Scan(&addedBy)
	if err != nil {
		return "", err
	}
	return addedBy, nil
}

// GetGuildStreamersWithContent retrieves all streamers for a guild including custom content and added_by
func GetGuildStreamersWithContent(ctx context.Context, guildID string) ([]map[string]interface{}, error) {
	query := `
		SELECT s.id, s.twitch_broadcaster_id, s.twitch_login, s.twitch_display_name, s.twitch_avatar_url,
		       s.created_at, s.last_updated, COALESCE(gs.custom_content, '') as custom_content, COALESCE(gs.added_by, '') as added_by
		FROM streamers s
		JOIN guild_streamers gs ON s.id = gs.streamer_id
		WHERE gs.guild_id = $1
		ORDER BY s.twitch_display_name
	`
	rows, err := Pool.Query(ctx, query, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var s Streamer
		var customContent, addedBy string
		err := rows.Scan(&s.ID, &s.TwitchBroadcasterID, &s.TwitchLogin, &s.TwitchDisplayName,
			&s.TwitchAvatarURL, &s.CreatedAt, &s.LastUpdated, &customContent, &addedBy)
		if err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":                    s.ID,
			"twitch_broadcaster_id": s.TwitchBroadcasterID,
			"twitch_login":          s.TwitchLogin,
			"twitch_display_name":   s.TwitchDisplayName,
			"twitch_avatar_url":     s.TwitchAvatarURL,
			"custom_content":        customContent,
			"added_by":              addedBy,
		})
	}
	return results, rows.Err()
}

// Helper functions

// nullableString converts empty string to nil for SQL
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
