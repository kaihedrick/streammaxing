package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/yourusername/streammaxing/internal/db"
	"github.com/yourusername/streammaxing/internal/middleware"
	"github.com/yourusername/streammaxing/internal/services/authorization"
	"github.com/yourusername/streammaxing/internal/services/logging"
	"github.com/yourusername/streammaxing/internal/validation"
)

// InviteHandler handles invite link operations
type InviteHandler struct {
	guildAuth      *authorization.GuildAuthService
	securityLogger *logging.SecurityLogger
	validator      *validation.Validator
}

// NewInviteHandler creates a new invite handler
func NewInviteHandler(
	guildAuth *authorization.GuildAuthService,
	securityLogger *logging.SecurityLogger,
) *InviteHandler {
	return &InviteHandler{
		guildAuth:      guildAuth,
		securityLogger: securityLogger,
		validator:      validation.NewValidator(),
	}
}

// CreateInvite creates a new invite link for a guild (admin only)
func (h *InviteHandler) CreateInvite(w http.ResponseWriter, r *http.Request, guildID string) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate guild ID
	if err := h.validator.ValidateGuildID(guildID); err != nil {
		http.Error(w, "Invalid guild ID", http.StatusBadRequest)
		return
	}

	// Verify admin with real-time check
	isAdmin, err := h.guildAuth.CheckGuildAdmin(r.Context(), userID, guildID)
	if err != nil || !isAdmin {
		h.securityLogger.LogPermissionDenied(r.Context(), userID, guildID, "create_invite")
		http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var body struct {
		ExpiresInHours int `json:"expires_in_hours"` // 0 = never
		MaxUses        int `json:"max_uses"`         // 0 = unlimited
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&body)
	}

	var expiresAt *time.Time
	if body.ExpiresInHours > 0 {
		t := time.Now().Add(time.Duration(body.ExpiresInHours) * time.Hour)
		expiresAt = &t
	}

	link, err := db.CreateInviteLink(r.Context(), guildID, userID, expiresAt, body.MaxUses)
	if err != nil {
		log.Printf("[INVITE_ERROR] Failed to create invite: %v", err)
		http.Error(w, "Failed to create invite", http.StatusInternalServerError)
		return
	}

	log.Printf("[INVITE] Created invite %s for guild %s by %s", link.Code, guildID, userID)
	db.InsertAuditLog(r.Context(), userID, "create_invite", "invite", link.Code, map[string]interface{}{"guild_id": guildID}, r.RemoteAddr, true)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(link)
}

// ListInvites returns all invite links for a guild (admin only)
func (h *InviteHandler) ListInvites(w http.ResponseWriter, r *http.Request, guildID string) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate guild ID
	if err := h.validator.ValidateGuildID(guildID); err != nil {
		http.Error(w, "Invalid guild ID", http.StatusBadRequest)
		return
	}

	// Verify admin with real-time check
	isAdmin, err := h.guildAuth.CheckGuildAdmin(r.Context(), userID, guildID)
	if err != nil || !isAdmin {
		h.securityLogger.LogPermissionDenied(r.Context(), userID, guildID, "list_invites")
		http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
		return
	}

	links, err := db.GetGuildInviteLinks(r.Context(), guildID)
	if err != nil {
		log.Printf("[INVITE_ERROR] Failed to list invites: %v", err)
		http.Error(w, "Failed to list invites", http.StatusInternalServerError)
		return
	}

	if links == nil {
		links = []db.InviteLink{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

// DeleteInvite deletes an invite link (admin only)
func (h *InviteHandler) DeleteInvite(w http.ResponseWriter, r *http.Request, guildID, inviteID string) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate guild ID
	if err := h.validator.ValidateGuildID(guildID); err != nil {
		http.Error(w, "Invalid guild ID", http.StatusBadRequest)
		return
	}

	// Verify admin with real-time check
	isAdmin, err := h.guildAuth.CheckGuildAdmin(r.Context(), userID, guildID)
	if err != nil || !isAdmin {
		h.securityLogger.LogPermissionDenied(r.Context(), userID, guildID, "delete_invite")
		http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
		return
	}

	if err := db.DeleteInviteLink(r.Context(), inviteID); err != nil {
		log.Printf("[INVITE_ERROR] Failed to delete invite: %v", err)
		http.Error(w, "Failed to delete invite", http.StatusInternalServerError)
		return
	}

	log.Printf("[INVITE] Deleted invite %s from guild %s by %s", inviteID, guildID, userID)
	db.InsertAuditLog(r.Context(), userID, "delete_invite", "invite", inviteID, map[string]interface{}{"guild_id": guildID}, r.RemoteAddr, true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Invite deleted"})
}

// GetInviteInfo returns public info about an invite (no auth required)
func (h *InviteHandler) GetInviteInfo(w http.ResponseWriter, r *http.Request, code string) {
	// Validate invite code format
	if err := h.validator.ValidateInviteCode(code); err != nil {
		http.Error(w, "Invalid invite code", http.StatusBadRequest)
		return
	}

	link, err := db.GetInviteLink(r.Context(), code)
	if err != nil {
		http.Error(w, "Invite not found", http.StatusNotFound)
		return
	}

	// Check if expired
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		http.Error(w, "Invite has expired", http.StatusGone)
		return
	}

	// Check if exhausted
	if link.MaxUses > 0 && link.UseCount >= link.MaxUses {
		http.Error(w, "Invite has reached maximum uses", http.StatusGone)
		return
	}

	// Fetch guild info
	guild, err := db.GetGuild(r.Context(), link.GuildID)
	if err != nil {
		http.Error(w, "Guild not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":       link.Code,
		"guild_id":   guild.GuildID,
		"guild_name": guild.Name,
		"guild_icon": guild.Icon,
		"valid":      true,
	})
}

// AcceptInvite accepts an invite and adds the user to the guild (auth required)
func (h *InviteHandler) AcceptInvite(w http.ResponseWriter, r *http.Request, code string) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate invite code format
	if err := h.validator.ValidateInviteCode(code); err != nil {
		http.Error(w, "Invalid invite code", http.StatusBadRequest)
		return
	}

	link, err := db.GetInviteLink(r.Context(), code)
	if err != nil {
		http.Error(w, "Invite not found", http.StatusNotFound)
		return
	}

	// Validate invite
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		http.Error(w, "Invite has expired", http.StatusGone)
		return
	}
	if link.MaxUses > 0 && link.UseCount >= link.MaxUses {
		http.Error(w, "Invite has reached maximum uses", http.StatusGone)
		return
	}

	// Add user to guild as non-admin member
	if err := db.UpsertUserGuild(r.Context(), userID, link.GuildID, false); err != nil {
		log.Printf("[INVITE_ERROR] Failed to add user %s to guild %s: %v", userID, link.GuildID, err)
		http.Error(w, "Failed to accept invite", http.StatusInternalServerError)
		return
	}

	// Increment use count
	if err := db.IncrementInviteUse(r.Context(), code); err != nil {
		log.Printf("[INVITE_WARN] Failed to increment invite use: %v", err)
	}

	// Fetch guild info for response
	guild, err := db.GetGuild(r.Context(), link.GuildID)
	if err != nil {
		http.Error(w, "Guild not found", http.StatusNotFound)
		return
	}

	log.Printf("[INVITE] User %s accepted invite %s to guild %s", userID, code, link.GuildID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Invite accepted",
		"guild_id": guild.GuildID,
		"name":     guild.Name,
	})
}
