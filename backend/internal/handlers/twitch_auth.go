package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/middleware"
	"github.com/yourusername/streammaxing/internal/services/encryption"
	"github.com/yourusername/streammaxing/internal/services/logging"
	"github.com/yourusername/streammaxing/internal/services/twitch"
	"github.com/yourusername/streammaxing/internal/validation"
)

// TwitchAuthHandler handles Twitch OAuth and streamer linking
type TwitchAuthHandler struct {
	oauth          *twitch.OAuthService
	eventsub       *twitch.EventSubService
	encryptionSvc  *encryption.Service
	securityLogger *logging.SecurityLogger
	validator      *validation.Validator
}

// NewTwitchAuthHandler creates a new Twitch auth handler.
// Twitch services are injected from the centralized config.
func NewTwitchAuthHandler(
	twitchOAuth *twitch.OAuthService,
	eventsub *twitch.EventSubService,
	encryptionSvc *encryption.Service,
	securityLogger *logging.SecurityLogger,
) *TwitchAuthHandler {
	return &TwitchAuthHandler{
		oauth:          twitchOAuth,
		eventsub:       eventsub,
		encryptionSvc:  encryptionSvc,
		securityLogger: securityLogger,
		validator:      validation.NewValidator(),
	}
}

// InitiateStreamerLink starts the Twitch OAuth flow to link a streamer to a guild
func (h *TwitchAuthHandler) InitiateStreamerLink(w http.ResponseWriter, r *http.Request, guildID string) {
	// Validate guild ID format
	if err := h.validator.ValidateGuildID(guildID); err != nil {
		http.Error(w, "Invalid guild ID", http.StatusBadRequest)
		return
	}

	// Get the authenticated user's ID so we can embed it in the state.
	// This avoids relying on the session cookie surviving the Twitch redirect.
	userID := middleware.GetUserID(r)

	// Generate state with guild_id and user_id encoded
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	state := fmt.Sprintf("guild_id:%s:user_id:%s:%s", guildID, userID, hex.EncodeToString(randomBytes))

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
		h.securityLogger.LogAuthFailure(ctx, "", r.RemoteAddr, "invalid_twitch_oauth_state")
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Extract guild_id and user_id from state
	// Format: "guild_id:GUILD_ID:user_id:USER_ID:RANDOM"
	stateParts := strings.SplitN(stateCookie.Value, ":", 5)
	if len(stateParts) < 5 || stateParts[0] != "guild_id" || stateParts[2] != "user_id" {
		// Fallback: try old format "guild_id:GUILD_ID:RANDOM" for existing cookies
		oldParts := strings.SplitN(stateCookie.Value, ":", 3)
		if len(oldParts) < 3 || oldParts[0] != "guild_id" {
			log.Printf("[TWITCH_AUTH_ERROR] Invalid state format: %s", stateCookie.Value)
			http.Error(w, "Invalid state format", http.StatusBadRequest)
			return
		}
		// Old format - can't get user_id, proceed without it
		log.Printf("[TWITCH_AUTH_WARN] Old state format, user_id not available")
	}
	guildID := stateParts[1]
	stateUserID := ""
	if len(stateParts) >= 5 {
		stateUserID = stateParts[3]
	}

	// Validate guild ID
	if err := h.validator.ValidateGuildID(guildID); err != nil {
		http.Error(w, "Invalid guild ID in state", http.StatusBadRequest)
		return
	}

	// Check for OAuth error
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		log.Printf("[TWITCH_AUTH_ERROR] Twitch OAuth error: %s", errParam)
		http.Redirect(w, r, fmt.Sprintf("%s/dashboard/guilds/%s?error=twitch_auth_denied", handlerFrontendURL, guildID), http.StatusTemporaryRedirect)
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

	// SECURITY: Encrypt OAuth tokens before storing in database
	encryptedAccessToken := tokenResp.AccessToken
	encryptedRefreshToken := tokenResp.RefreshToken
	if h.encryptionSvc != nil {
		encryptedAccessToken, err = h.encryptionSvc.Encrypt(tokenResp.AccessToken)
		if err != nil {
			log.Printf("[TWITCH_AUTH_ERROR] Failed to encrypt access token: %v", err)
			h.securityLogger.LogTokenEncryptionFailure(ctx, user.ID, err)
			http.Error(w, "Internal security error", http.StatusInternalServerError)
			return
		}

		encryptedRefreshToken, err = h.encryptionSvc.Encrypt(tokenResp.RefreshToken)
		if err != nil {
			log.Printf("[TWITCH_AUTH_ERROR] Failed to encrypt refresh token: %v", err)
			h.securityLogger.LogTokenEncryptionFailure(ctx, user.ID, err)
			http.Error(w, "Internal security error", http.StatusInternalServerError)
			return
		}
	}

	// Store streamer in database with encrypted tokens
	streamer := &db.Streamer{
		TwitchBroadcasterID: user.ID,
		TwitchLogin:         user.Login,
		TwitchDisplayName:   user.DisplayName,
		TwitchAvatarURL:     user.ProfileImageURL,
		TwitchAccessToken:   encryptedAccessToken,
		TwitchRefreshToken:  encryptedRefreshToken,
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
	// Use user_id from the state parameter (embedded during initiation)
	// to avoid depending on the session cookie surviving the Twitch redirect.
	userID := stateUserID
	if userID == "" {
		// Fallback: try session cookie if available
		userID = middleware.GetUserID(r)
	}
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
	http.Redirect(w, r, fmt.Sprintf("%s/dashboard/guilds/%s", handlerFrontendURL, guildID), http.StatusTemporaryRedirect)
}
