package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/middleware"
	"github.com/yourusername/streammaxing/internal/services/twitch"
)

// TwitchAuthHandler handles Twitch OAuth and streamer linking
type TwitchAuthHandler struct {
	oauth    *twitch.OAuthService
	eventsub *twitch.EventSubService
}

// NewTwitchAuthHandler creates a new Twitch auth handler
func NewTwitchAuthHandler(apiClient *twitch.APIClient) *TwitchAuthHandler {
	return &TwitchAuthHandler{
		oauth:    twitch.NewOAuthService(),
		eventsub: twitch.NewEventSubService(apiClient),
	}
}

// InitiateStreamerLink starts the Twitch OAuth flow to link a streamer to a guild
func (h *TwitchAuthHandler) InitiateStreamerLink(w http.ResponseWriter, r *http.Request, guildID string) {
	// Generate state with guild_id encoded
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	state := fmt.Sprintf("guild_id:%s:%s", guildID, hex.EncodeToString(randomBytes))

	// Store state in cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "twitch_oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   isProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	authURL := h.oauth.GetAuthURL(state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": authURL})
}

// TwitchCallback handles the Twitch OAuth callback after streamer authorization
func (h *TwitchAuthHandler) TwitchCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify state
	stateCookie, err := r.Cookie("twitch_oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		log.Printf("[TWITCH_AUTH_ERROR] Invalid OAuth state")
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Extract guild_id from state (format: "guild_id:GUILD_ID:RANDOM")
	stateParts := strings.SplitN(stateCookie.Value, ":", 3)
	if len(stateParts) < 3 || stateParts[0] != "guild_id" {
		http.Error(w, "Invalid state format", http.StatusBadRequest)
		return
	}
	guildID := stateParts[1]

	// Check for OAuth error
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		log.Printf("[TWITCH_AUTH_ERROR] Twitch OAuth error: %s", errParam)
		frontendURL := os.Getenv("FRONTEND_URL")
		http.Redirect(w, r, fmt.Sprintf("%s/dashboard/guilds/%s?error=twitch_auth_denied", frontendURL, guildID), http.StatusTemporaryRedirect)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	tokenResp, err := h.oauth.ExchangeCode(code)
	if err != nil {
		log.Printf("[TWITCH_AUTH_ERROR] Failed to exchange code: %v", err)
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Fetch streamer info
	user, err := h.oauth.GetUser(tokenResp.AccessToken)
	if err != nil {
		log.Printf("[TWITCH_AUTH_ERROR] Failed to fetch Twitch user: %v", err)
		http.Error(w, "Failed to fetch Twitch user", http.StatusInternalServerError)
		return
	}

	// Store streamer in database
	streamer := &db.Streamer{
		TwitchBroadcasterID: user.ID,
		TwitchLogin:         user.Login,
		TwitchDisplayName:   user.DisplayName,
		TwitchAvatarURL:     user.ProfileImageURL,
		TwitchAccessToken:   tokenResp.AccessToken,
		TwitchRefreshToken:  tokenResp.RefreshToken,
	}

	if err := db.CreateStreamer(ctx, streamer); err != nil {
		log.Printf("[TWITCH_AUTH_ERROR] Failed to store streamer: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Create EventSub subscription
	subscription, err := h.eventsub.CreateStreamOnlineSubscription(user.ID)
	if err != nil {
		// Log error but don't fail - can retry later
		log.Printf("[TWITCH_AUTH_WARN] Failed to create EventSub subscription for %s: %v", user.Login, err)
	} else {
		// Store subscription in database
		if err := db.CreateEventSubSubscription(ctx, streamer.ID, subscription.ID, subscription.Status); err != nil {
			log.Printf("[TWITCH_AUTH_WARN] Failed to store subscription: %v", err)
		}
	}

	// Link streamer to guild
	userID := middleware.GetUserID(r)
	if err := db.LinkStreamerToGuild(ctx, guildID, streamer.ID, userID); err != nil {
		log.Printf("[TWITCH_AUTH_ERROR] Failed to link streamer to guild: %v", err)
		http.Error(w, "Failed to link streamer", http.StatusInternalServerError)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "twitch_oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	log.Printf("[TWITCH_AUTH] Linked streamer %s (%s) to guild %s", user.DisplayName, user.ID, guildID)

	// Redirect to frontend dashboard
	frontendURL := os.Getenv("FRONTEND_URL")
	http.Redirect(w, r, fmt.Sprintf("%s/dashboard/guilds/%s", frontendURL, guildID), http.StatusTemporaryRedirect)
}
