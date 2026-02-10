package db

import (
	"encoding/json"
	"time"
)

// User represents a Discord user
type User struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Avatar    string    `json:"avatar,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	LastLogin time.Time `json:"last_login"`
}

// Guild represents a Discord server
type Guild struct {
	GuildID   string    `json:"guild_id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon,omitempty"`
	OwnerID   string    `json:"owner_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// GuildConfig represents per-server notification settings
type GuildConfig struct {
	GuildID         string          `json:"guild_id"`
	ChannelID       string          `json:"channel_id"`
	MentionRoleID   string          `json:"mention_role_id,omitempty"`
	MessageTemplate json.RawMessage `json:"message_template"`
	Enabled         bool            `json:"enabled"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Streamer represents a Twitch streamer
type Streamer struct {
	ID                   string    `json:"id"`
	TwitchBroadcasterID  string    `json:"twitch_broadcaster_id"`
	TwitchLogin          string    `json:"twitch_login"`
	TwitchDisplayName    string    `json:"twitch_display_name,omitempty"`
	TwitchAvatarURL      string    `json:"twitch_avatar_url,omitempty"`
	TwitchAccessToken    string    `json:"-"` // Never serialize to JSON
	TwitchRefreshToken   string    `json:"-"` // Never serialize to JSON
	CreatedAt            time.Time `json:"created_at"`
	LastUpdated          time.Time `json:"last_updated"`
}

// GuildStreamer represents the link between a guild and a streamer
type GuildStreamer struct {
	GuildID       string    `json:"guild_id"`
	StreamerID    string    `json:"streamer_id"`
	Enabled       bool      `json:"enabled"`
	AddedBy       string    `json:"added_by,omitempty"`
	AddedAt       time.Time `json:"added_at"`
	CustomContent string    `json:"custom_content,omitempty"`
}

// GuildWithRole represents a guild with the user's admin status
type GuildWithRole struct {
	Guild
	IsAdmin bool `json:"is_admin"`
}

// InviteLink represents an admin-generated invite code
type InviteLink struct {
	ID        string     `json:"id"`
	GuildID   string     `json:"guild_id"`
	Code      string     `json:"code"`
	CreatedBy string     `json:"created_by"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	MaxUses   int        `json:"max_uses"`
	UseCount  int        `json:"use_count"`
	CreatedAt time.Time  `json:"created_at"`
}

// UserPreference represents per-user notification settings
type UserPreference struct {
	UserID               string    `json:"user_id"`
	GuildID              string    `json:"guild_id"`
	StreamerID           string    `json:"streamer_id"`
	NotificationsEnabled bool      `json:"notifications_enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// EventSubSubscription represents a Twitch EventSub subscription
type EventSubSubscription struct {
	StreamerID     string    `json:"streamer_id"`
	SubscriptionID string    `json:"subscription_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	LastVerified   time.Time `json:"last_verified"`
}

// NotificationLog represents a sent notification (for idempotency)
type NotificationLog struct {
	ID         string    `json:"id"`
	GuildID    string    `json:"guild_id"`
	StreamerID string    `json:"streamer_id"`
	EventID    string    `json:"event_id"`
	SentAt     time.Time `json:"sent_at"`
}

// MessageTemplate represents the JSONB structure for notification templates
type MessageTemplate struct {
	Content string       `json:"content,omitempty"`
	Embed   *EmbedObject `json:"embed,omitempty"`
}

// EmbedObject represents a Discord embed
type EmbedObject struct {
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	URL         string        `json:"url,omitempty"`
	Color       int           `json:"color,omitempty"`
	Thumbnail   *EmbedImage   `json:"thumbnail,omitempty"`
	Image       *EmbedImage   `json:"image,omitempty"`
	Fields      []EmbedField  `json:"fields,omitempty"`
	Footer      *EmbedFooter  `json:"footer,omitempty"`
	Timestamp   bool          `json:"timestamp,omitempty"` // If true, use stream start time
}

// EmbedImage represents an embed image or thumbnail
type EmbedImage struct {
	URL string `json:"url"`
}

// EmbedField represents an embed field
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// EmbedFooter represents an embed footer
type EmbedFooter struct {
	Text string `json:"text"`
}

// DefaultMessageTemplate returns the default notification template
func DefaultMessageTemplate() MessageTemplate {
	return MessageTemplate{
		Content: "{streamer_display_name} is now live!",
		Embed: &EmbedObject{
			Title:       "{streamer_display_name} is streaming {game_name}",
			Description: "{stream_title}",
			URL:         "https://twitch.tv/{streamer_login}",
			Color:       6570404, // Twitch purple
			Thumbnail:   &EmbedImage{URL: "{streamer_avatar_url}"},
			Image:       &EmbedImage{URL: "{stream_thumbnail_url}"},
			Fields: []EmbedField{
				{Name: "Viewers", Value: "{viewer_count}", Inline: true},
				{Name: "Game", Value: "{game_name}", Inline: true},
			},
			Footer:    &EmbedFooter{Text: "Twitch Notification"},
			Timestamp: true,
		},
	}
}
