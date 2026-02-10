package notifications

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/streammaxing/internal/db"
	discordSvc "github.com/yourusername/streammaxing/internal/services/discord"
	twitchSvc "github.com/yourusername/streammaxing/internal/services/twitch"
)

// FanoutService handles notification delivery to all configured guilds
type FanoutService struct {
	TwitchAPI   *twitchSvc.APIClient
	DiscordAPI  *discordSvc.APIClient
	TemplateSvc *TemplateService
}

// NewFanoutService creates a new notification fanout service
func NewFanoutService(twitchAPI *twitchSvc.APIClient, discordAPI *discordSvc.APIClient) *FanoutService {
	return &FanoutService{
		TwitchAPI:   twitchAPI,
		DiscordAPI:  discordAPI,
		TemplateSvc: NewTemplateService(),
	}
}

// StreamOnlineEvent represents the event data from a stream.online EventSub notification
type StreamOnlineEvent struct {
	ID                   string `json:"id"`
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	Type                 string `json:"type"`
	StartedAt            string `json:"started_at"`
}

// HandleStreamOnline processes a stream.online event and fans out notifications
func (s *FanoutService) HandleStreamOnline(ctx context.Context, eventID string, event StreamOnlineEvent) error {
	start := time.Now()

	// Fetch full stream data (title, game, viewers, thumbnail)
	streamData, err := s.TwitchAPI.GetStreamData(event.BroadcasterUserID)
	if err != nil {
		log.Printf("[FANOUT_ERROR] Failed to fetch stream data for %s: %v", event.BroadcasterUserID, err)
		return err
	}

	// Get streamer from database
	streamer, err := db.GetStreamerByBroadcasterID(ctx, event.BroadcasterUserID)
	if err != nil {
		log.Printf("[FANOUT_ERROR] Streamer not found: %s: %v", event.BroadcasterUserID, err)
		return err
	}

	// Query all guilds tracking this streamer
	guildIDs, err := db.GetGuildsTrackingStreamer(ctx, streamer.ID)
	if err != nil {
		log.Printf("[FANOUT_ERROR] Failed to fetch guilds for streamer %s: %v", streamer.ID, err)
		return err
	}

	log.Printf("[FANOUT] %s went live, notifying %d guilds", event.BroadcasterUserName, len(guildIDs))

	// Fan out to each guild
	successCount := 0
	for _, guildID := range guildIDs {
		if err := s.sendNotificationToGuild(ctx, guildID, streamer, streamData, eventID); err != nil {
			log.Printf("[NOTIF_ERROR] Guild %s: %v", guildID, err)
			// Continue to next guild (don't fail entire fanout)
		} else {
			successCount++
		}
	}

	duration := time.Since(start)
	log.Printf("[FANOUT] Completed: %s, Guilds: %d/%d, Duration: %v", event.BroadcasterUserName, successCount, len(guildIDs), duration)

	return nil
}

// sendNotificationToGuild sends a notification to a single guild
func (s *FanoutService) sendNotificationToGuild(
	ctx context.Context,
	guildID string,
	streamer *db.Streamer,
	streamData *twitchSvc.StreamData,
	eventID string,
) error {
	// Check idempotency (prevent duplicate notifications)
	isDuplicate, err := db.CheckNotificationSent(ctx, guildID, eventID)
	if err != nil {
		return fmt.Errorf("idempotency check failed: %w", err)
	}
	if isDuplicate {
		log.Printf("[NOTIF_SKIP] Duplicate: guild=%s event=%s", guildID, eventID)
		return nil
	}

	// Fetch guild configuration
	config, err := db.GetGuildConfig(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to fetch guild config: %w", err)
	}

	if !config.Enabled {
		log.Printf("[NOTIF_SKIP] Disabled: guild=%s", guildID)
		return nil
	}

	// Check for per-streamer custom content
	customContent, err := db.GetStreamerCustomContent(ctx, guildID, streamer.ID)
	if err != nil {
		log.Printf("[NOTIF_WARN] Failed to fetch custom content for guild=%s streamer=%s: %v", guildID, streamer.ID, err)
		customContent = "" // Fall back to template default
	}

	// Render message template (with optional custom content override)
	message, err := s.TemplateSvc.RenderTemplate(config.MessageTemplate, streamer, streamData, config.MentionRoleID)
	if err != nil {
		return fmt.Errorf("template rendering failed: %w", err)
	}

	// Override text content if streamer has custom content set
	if customContent != "" {
		message.Content = s.TemplateSvc.RenderCustomContent(customContent, streamer, streamData, config.MentionRoleID)
	}

	// Send Discord message
	if err := s.DiscordAPI.SendMessage(config.ChannelID, message); err != nil {
		return fmt.Errorf("discord send failed: %w", err)
	}

	// Log notification (idempotency)
	if err := db.LogNotification(ctx, guildID, streamer.ID, eventID); err != nil {
		log.Printf("[NOTIF_WARN] Failed to log notification: %v", err)
		// Non-fatal error
	}

	log.Printf("[NOTIF_SENT] Guild=%s Channel=%s Event=%s", guildID, config.ChannelID, eventID)
	return nil
}
