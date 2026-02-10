package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/middleware"
	"github.com/yourusername/streammaxing/internal/services/discord"
)

// AuthHandler handles authentication routes
type AuthHandler struct {
	oauth *discord.OAuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		oauth: discord.NewOAuthService(),
	}
}

// generateState creates a random state string for CSRF protection
func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateJWT creates a JWT token for a user session
func generateJWT(userID, username string) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", jwt.ErrSignatureInvalid
	}

	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// isProduction returns true if running in production environment
func isProduction() bool {
	return os.Getenv("ENVIRONMENT") == "production"
}

// getCookieDomain returns the appropriate cookie domain for the current environment.
// In production: empty string (cookie scoped to the exact host that set it).
// In development: "localhost" (shared across ports 8080 and 5173).
func getCookieDomain() string {
	if isProduction() {
		return "" // Omit Domain attr → scoped to the CloudFront/custom domain automatically
	}
	return "localhost"
}

// DiscordLogin initiates the Discord OAuth flow
func (h *AuthHandler) DiscordLogin(w http.ResponseWriter, r *http.Request) {
	state := generateState()

	// Store state in cookie for verification
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   isProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	authURL := h.oauth.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// DiscordCallback handles the Discord OAuth callback
func (h *AuthHandler) DiscordCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Debug: log all cookies and the raw Cookie header
	log.Printf("[AUTH_DEBUG] Callback hit. Cookie header: %q", r.Header.Get("Cookie"))
	log.Printf("[AUTH_DEBUG] All cookies: %v", r.Cookies())
	log.Printf("[AUTH_DEBUG] Request headers: %v", r.Header)

	// Verify state parameter (CSRF protection)
	stateCookie, err := r.Cookie("oauth_state")
	queryState := r.URL.Query().Get("state")
	if err != nil {
		log.Printf("[AUTH_ERROR] Missing oauth_state cookie (did you start from /api/auth/discord/login?). URL state=%s, cookie error=%v", queryState, err)
		http.Error(w, "Invalid state parameter - please start login from the beginning", http.StatusBadRequest)
		return
	}
	if stateCookie.Value != queryState {
		log.Printf("[AUTH_ERROR] State mismatch: cookie=%s, url=%s", stateCookie.Value, queryState)
		http.Error(w, "Invalid state parameter - state mismatch", http.StatusBadRequest)
		return
	}

	// Check for OAuth error
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		log.Printf("[AUTH_ERROR] Discord OAuth error: %s", errParam)
		frontendURL := os.Getenv("FRONTEND_URL")
		http.Redirect(w, r, frontendURL+"/?error=auth_denied", http.StatusTemporaryRedirect)
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
		log.Printf("[AUTH_ERROR] Failed to exchange code: %v", err)
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}
	log.Printf("[AUTH_DEBUG] Token exchange successful, scopes: %s", tokenResp.Scope)

	// Fetch user info
	user, err := h.oauth.GetUser(tokenResp.AccessToken)
	if err != nil {
		log.Printf("[AUTH_ERROR] Failed to fetch user: %v", err)
		http.Error(w, "Failed to fetch user information", http.StatusInternalServerError)
		return
	}
	log.Printf("[AUTH_DEBUG] User fetched: %s (%s)", user.Username, user.ID)

	// Fetch user guilds
	log.Printf("[AUTH_DEBUG] Attempting to fetch guilds with access token...")
	guilds, err := h.oauth.GetUserGuilds(tokenResp.AccessToken)
	if err != nil {
		log.Printf("[AUTH_ERROR] Failed to fetch guilds: %v", err)
		http.Error(w, "Failed to fetch guilds", http.StatusInternalServerError)
		return
	}
	log.Printf("[AUTH_DEBUG] Successfully fetched %d guilds", len(guilds))

	// Store user in database
	if err := db.CreateOrUpdateUser(ctx, &db.User{
		UserID:   user.ID,
		Username: user.Username,
		Avatar:   user.Avatar,
	}); err != nil {
		log.Printf("[AUTH_ERROR] Failed to store user: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Get all guild IDs in our database (where the bot is installed)
	knownGuildIDs, err := db.GetAllGuildIDs(ctx)
	if err != nil {
		log.Printf("[AUTH_WARN] Failed to fetch known guild IDs: %v", err)
		knownGuildIDs = make(map[string]bool)
	}

	// Store guilds where user has admin permission and update memberships
	for _, guild := range guilds {
		isAdmin := guild.Owner || discord.HasAdminPermission(guild.Permissions)

		// Admins can add new guilds to the database
		if isAdmin {
			ownerID := ""
			if guild.Owner {
				ownerID = user.ID
			}
			if err := db.CreateOrUpdateGuild(ctx, &db.Guild{
				GuildID: guild.ID,
				Name:    guild.Name,
				Icon:    guild.Icon,
				OwnerID: ownerID,
			}); err != nil {
				log.Printf("[AUTH_WARN] Failed to store guild %s: %v", guild.ID, err)
				continue
			}
			// Upsert user-guild membership for admin
			if err := db.UpsertUserGuild(ctx, user.ID, guild.ID, true); err != nil {
				log.Printf("[AUTH_WARN] Failed to store membership for guild %s: %v", guild.ID, err)
			}
		} else if knownGuildIDs[guild.ID] {
			// Non-admin user is in a guild that's already in our DB
			// Store their membership (non-admin)
			if err := db.UpsertUserGuild(ctx, user.ID, guild.ID, false); err != nil {
				log.Printf("[AUTH_WARN] Failed to store membership for guild %s: %v", guild.ID, err)
			}
		}
	}

	// Generate JWT session token
	jwtToken, err := generateJWT(user.ID, user.Username)
	if err != nil {
		log.Printf("[AUTH_ERROR] Failed to generate JWT: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	// Dev: Domain "localhost" shares cookie across ports (8080 → 5173)
	// Prod: Empty Domain → scoped to the host that set it (CloudFront domain)
	sessionCookie := &http.Cookie{
		Name:     "session",
		Value:    jwtToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,
		Secure:   isProduction(),
		SameSite: http.SameSiteLaxMode,
	}
	if domain := getCookieDomain(); domain != "" {
		sessionCookie.Domain = domain
	}
	http.SetCookie(w, sessionCookie)

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	log.Printf("[AUTH] User %s (%s) logged in", user.Username, user.ID)

	// Redirect to frontend dashboard
	frontendURL := os.Getenv("FRONTEND_URL")
	http.Redirect(w, r, frontendURL+"/dashboard", http.StatusTemporaryRedirect)
}

// Logout clears the session cookie
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out"})
}

// GetMe returns the current authenticated user info
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
