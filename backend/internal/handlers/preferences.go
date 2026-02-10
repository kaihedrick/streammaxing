package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/middleware"
)

// PreferencesHandler handles user notification preferences
type PreferencesHandler struct{}

// NewPreferencesHandler creates a new preferences handler
func NewPreferencesHandler() *PreferencesHandler {
	return &PreferencesHandler{}
}

// GetUserPreferences returns the current user's notification preferences
func (h *PreferencesHandler) GetUserPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	prefs, err := db.GetUserPreferences(r.Context(), userID)
	if err != nil {
		log.Printf("[PREF_ERROR] Failed to fetch preferences for user %s: %v", userID, err)
		http.Error(w, "Failed to fetch preferences", http.StatusInternalServerError)
		return
	}

	if prefs == nil {
		prefs = []db.UserPreference{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

// UpdateUserPreference updates a notification preference for a specific streamer in a guild
func (h *PreferencesHandler) UpdateUserPreference(w http.ResponseWriter, r *http.Request, guildID, streamerID string) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := db.SetUserPreference(r.Context(), userID, guildID, streamerID, body.Enabled); err != nil {
		log.Printf("[PREF_ERROR] Failed to update preference: %v", err)
		http.Error(w, "Failed to update preference", http.StatusInternalServerError)
		return
	}

	log.Printf("[PREF] Updated preference: user=%s guild=%s streamer=%s enabled=%v", userID, guildID, streamerID, body.Enabled)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Preference updated"})
}
