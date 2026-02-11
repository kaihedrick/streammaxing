package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/services/twitch"
)

// CleanupHandler handles database and subscription cleanup
type CleanupHandler struct {
	eventsubService *twitch.EventSubService
}

// NewCleanupHandler creates a new cleanup handler.
// The EventSub service is injected from the centralized config.
func NewCleanupHandler(eventsubService *twitch.EventSubService) *CleanupHandler {
	return &CleanupHandler{
		eventsubService: eventsubService,
	}
}

// RunCleanup runs all cleanup tasks (triggered manually or via cron)
func (h *CleanupHandler) RunCleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	results := make(map[string]interface{})

	// 1. Clean up orphaned streamers
	orphanedCount, err := h.cleanupOrphanedStreamers(ctx)
	if err != nil {
		log.Printf("[CLEANUP_ERROR] Orphaned streamers: %v", err)
		results["orphaned_streamers"] = map[string]interface{}{"error": err.Error()}
	} else {
		results["orphaned_streamers"] = map[string]interface{}{"deleted": orphanedCount}
	}

	// 2. Clean up old notification logs
	logCount, err := db.CleanupOldNotificationLogs(ctx)
	if err != nil {
		log.Printf("[CLEANUP_ERROR] Notification logs: %v", err)
		results["notification_logs"] = map[string]interface{}{"error": err.Error()}
	} else {
		results["notification_logs"] = map[string]interface{}{"deleted": logCount}
	}

	// 3. Sync EventSub subscription health
	syncCount, err := h.syncSubscriptionHealth(ctx)
	if err != nil {
		log.Printf("[CLEANUP_ERROR] Subscription sync: %v", err)
		results["subscription_sync"] = map[string]interface{}{"error": err.Error()}
	} else {
		results["subscription_sync"] = map[string]interface{}{"checked": syncCount}
	}

	log.Printf("[CLEANUP] Completed: orphans=%d, logs=%d, subs=%d", orphanedCount, logCount, syncCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// cleanupOrphanedStreamers removes streamers not linked to any guilds
func (h *CleanupHandler) cleanupOrphanedStreamers(ctx context.Context) (int, error) {
	orphanedIDs, err := db.GetOrphanedStreamers(ctx)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, streamerID := range orphanedIDs {
		// Delete EventSub subscription if exists
		sub, err := db.GetEventSubSubscription(ctx, streamerID)
		if err == nil && sub != nil {
			if delErr := h.eventsubService.DeleteSubscription(sub.SubscriptionID); delErr != nil {
				log.Printf("[CLEANUP_WARN] Failed to delete EventSub sub %s: %v", sub.SubscriptionID, delErr)
			} else {
				db.DeleteEventSubSubscription(ctx, sub.SubscriptionID)
			}
		}

		// Delete streamer (CASCADE handles related records)
		if err := db.DeleteStreamer(ctx, streamerID); err != nil {
			log.Printf("[CLEANUP_WARN] Failed to delete streamer %s: %v", streamerID, err)
			continue
		}
		deleted++
		log.Printf("[CLEANUP] Deleted orphaned streamer: %s", streamerID)
	}

	return deleted, nil
}

// syncSubscriptionHealth checks EventSub subscriptions against Twitch API
func (h *CleanupHandler) syncSubscriptionHealth(ctx context.Context) (int, error) {
	subs, err := h.eventsubService.ListSubscriptions()
	if err != nil {
		return 0, err
	}

	checked := 0
	for _, sub := range subs {
		broadcasterID, ok := sub.Condition["broadcaster_user_id"].(string)
		if !ok {
			continue
		}

		// Update status in database
		streamer, err := db.GetStreamerByBroadcasterID(ctx, broadcasterID)
		if err != nil {
			// Streamer not in our DB - this subscription is orphaned at Twitch
			log.Printf("[CLEANUP] Orphaned Twitch sub %s for broadcaster %s", sub.ID, broadcasterID)
			h.eventsubService.DeleteSubscription(sub.ID)
			continue
		}

		db.CreateEventSubSubscription(ctx, streamer.ID, sub.ID, sub.Status)
		checked++
	}

	return checked, nil
}

// HandleBotRemoved handles cleanup when the bot is removed from a guild
func (h *CleanupHandler) HandleBotRemoved(ctx context.Context, guildID string) error {
	log.Printf("[CLEANUP] Bot removed from guild: %s", guildID)

	// Delete guild (CASCADE handles guild_config, guild_streamers, user_preferences, notification_log)
	if err := db.DeleteGuild(ctx, guildID); err != nil {
		return err
	}

	log.Printf("[CLEANUP] Completed guild cleanup: %s", guildID)
	return nil
}
