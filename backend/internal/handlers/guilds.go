package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/middleware"
	"github.com/yourusername/streammaxing/internal/services/discord"
)

// GuildHandler handles guild management routes
type GuildHandler struct {
	discordAPI *discord.APIClient
	oauth      *discord.OAuthService
}

// NewGuildHandler creates a new guild handler
func NewGuildHandler() *GuildHandler {
	return &GuildHandler{
		discordAPI: discord.NewAPIClient(),
		oauth:      discord.NewOAuthService(),
	}
}

// GetUserGuilds returns guilds the authenticated user is a member of, with admin flag
func (h *GuildHandler) GetUserGuilds(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	guilds, err := db.GetUserGuildsForUser(r.Context(), userID)
	if err != nil {
		log.Printf("[GUILD_ERROR] Failed to fetch guilds for user %s: %v", userID, err)
		http.Error(w, "Failed to fetch guilds", http.StatusInternalServerError)
		return
	}

	if guilds == nil {
		guilds = []db.GuildWithRole{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(guilds)
}

// GetGuildChannels returns text channels for a guild
func (h *GuildHandler) GetGuildChannels(w http.ResponseWriter, r *http.Request, guildID string) {
	channels, err := h.discordAPI.GetGuildChannels(guildID)
	if err != nil {
		log.Printf("[GUILD_ERROR] Failed to fetch channels for %s: %v", guildID, err)
		http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

// GetGuildRoles returns roles for a guild
func (h *GuildHandler) GetGuildRoles(w http.ResponseWriter, r *http.Request, guildID string) {
	roles, err := h.discordAPI.GetGuildRoles(guildID)
	if err != nil {
		log.Printf("[GUILD_ERROR] Failed to fetch roles for %s: %v", guildID, err)
		http.Error(w, "Failed to fetch roles", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roles)
}

// GetGuildStreamers returns streamers linked to a guild (with custom_content and added_by)
func (h *GuildHandler) GetGuildStreamers(w http.ResponseWriter, r *http.Request, guildID string) {
	streamers, err := db.GetGuildStreamersWithContent(r.Context(), guildID)
	if err != nil {
		log.Printf("[GUILD_ERROR] Failed to fetch streamers for %s: %v", guildID, err)
		http.Error(w, "Failed to fetch streamers", http.StatusInternalServerError)
		return
	}

	if streamers == nil {
		streamers = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(streamers)
}

// GetStreamerMessage returns the custom notification text for a streamer
func (h *GuildHandler) GetStreamerMessage(w http.ResponseWriter, r *http.Request, guildID, streamerID string) {
	content, err := db.GetStreamerCustomContent(r.Context(), guildID, streamerID)
	if err != nil {
		log.Printf("[GUILD_ERROR] Failed to fetch custom content: %v", err)
		http.Error(w, "Failed to fetch message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"custom_content": content})
}

// UpdateStreamerMessage updates the custom notification text for a streamer
func (h *GuildHandler) UpdateStreamerMessage(w http.ResponseWriter, r *http.Request, guildID, streamerID string) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check permissions: admin can edit any, member can only edit their own
	isAdmin, err := db.IsUserGuildAdmin(r.Context(), userID, guildID)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !isAdmin {
		// Check if this streamer was added by the current user
		addedBy, err := db.GetGuildStreamerAddedBy(r.Context(), guildID, streamerID)
		if err != nil || addedBy != userID {
			http.Error(w, "Forbidden: you can only edit your own message", http.StatusForbidden)
			return
		}
	}

	var body struct {
		CustomContent string `json:"custom_content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := db.UpdateStreamerCustomContent(r.Context(), guildID, streamerID, body.CustomContent); err != nil {
		log.Printf("[GUILD_ERROR] Failed to update custom content: %v", err)
		http.Error(w, "Failed to update message", http.StatusInternalServerError)
		return
	}

	log.Printf("[GUILD] Updated custom content: guild=%s streamer=%s by=%s", guildID, streamerID, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Message updated"})
}

// UnlinkStreamer removes a streamer from a guild
func (h *GuildHandler) UnlinkStreamer(w http.ResponseWriter, r *http.Request, guildID, streamerID string) {
	if err := db.UnlinkStreamerFromGuild(r.Context(), guildID, streamerID); err != nil {
		log.Printf("[GUILD_ERROR] Failed to unlink streamer %s from %s: %v", streamerID, guildID, err)
		http.Error(w, "Failed to unlink streamer", http.StatusInternalServerError)
		return
	}

	log.Printf("[GUILD] Unlinked streamer %s from guild %s", streamerID, guildID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Streamer unlinked"})
}

// GetGuildConfig returns the guild notification configuration
func (h *GuildHandler) GetGuildConfig(w http.ResponseWriter, r *http.Request, guildID string) {
	config, err := db.GetGuildConfig(r.Context(), guildID)
	if err != nil {
		log.Printf("[GUILD_ERROR] Failed to fetch config for %s: %v", guildID, err)
		http.Error(w, "Failed to fetch configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateGuildConfig updates the guild notification configuration
func (h *GuildHandler) UpdateGuildConfig(w http.ResponseWriter, r *http.Request, guildID string) {
	var config db.GuildConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	config.GuildID = guildID

	if err := db.UpdateGuildConfig(r.Context(), &config); err != nil {
		log.Printf("[GUILD_ERROR] Failed to update config for %s: %v", guildID, err)
		http.Error(w, "Failed to update configuration", http.StatusInternalServerError)
		return
	}

	log.Printf("[GUILD] Updated config for guild %s", guildID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Configuration updated"})
}

// GetBotInstallURL returns the bot installation URL for a guild
func (h *GuildHandler) GetBotInstallURL(w http.ResponseWriter, r *http.Request, guildID string) {
	installURL := h.oauth.GetBotInstallURL(guildID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": installURL})
}
