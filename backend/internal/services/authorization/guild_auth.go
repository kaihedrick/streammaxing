package authorization

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/streammaxing/internal/db"
)

// GuildAuthService validates guild permissions with a short-lived cache.
// Permissions are re-validated against the database on every request,
// with a 5-minute cache TTL to reduce database load.
type GuildAuthService struct {
	cache *guildPermissionCache
}

type guildPermissionCache struct {
	mu   sync.RWMutex
	data map[string]map[string]cachedPermission // userID -> guildID -> permission
	ttl  time.Duration
}

type cachedPermission struct {
	isMember bool
	isAdmin  bool
	cachedAt time.Time
}

// NewGuildAuthService creates a new guild authorization service.
func NewGuildAuthService() *GuildAuthService {
	return &GuildAuthService{
		cache: &guildPermissionCache{
			data: make(map[string]map[string]cachedPermission),
			ttl:  5 * time.Minute,
		},
	}
}

// CheckGuildAdmin verifies the user is an admin of the specified guild.
// Uses a 5-minute TTL cache to reduce DB queries.
func (s *GuildAuthService) CheckGuildAdmin(ctx context.Context, userID, guildID string) (bool, error) {
	// Check cache first
	if perm, ok := s.cache.get(userID, guildID); ok {
		if time.Since(perm.cachedAt) < s.cache.ttl {
			return perm.isAdmin, nil
		}
	}

	// Query database for fresh data
	isAdmin, err := db.IsUserGuildAdmin(ctx, userID, guildID)
	if err != nil {
		return false, fmt.Errorf("failed to check guild admin: %w", err)
	}

	// Cache the result
	s.cache.set(userID, guildID, cachedPermission{
		isMember: true,
		isAdmin:  isAdmin,
		cachedAt: time.Now(),
	})

	return isAdmin, nil
}

// CheckGuildMember verifies the user is a member of the specified guild.
// Uses a 5-minute TTL cache to reduce DB queries.
func (s *GuildAuthService) CheckGuildMember(ctx context.Context, userID, guildID string) (bool, error) {
	// Check cache first
	if perm, ok := s.cache.get(userID, guildID); ok {
		if time.Since(perm.cachedAt) < s.cache.ttl {
			return perm.isMember, nil
		}
	}

	// Query database for fresh data
	isMember, err := db.IsUserGuildMember(ctx, userID, guildID)
	if err != nil {
		return false, fmt.Errorf("failed to check guild member: %w", err)
	}

	// Also check admin status since we're querying
	isAdmin := false
	if isMember {
		isAdmin, _ = db.IsUserGuildAdmin(ctx, userID, guildID)
	}

	// Cache the result
	s.cache.set(userID, guildID, cachedPermission{
		isMember: isMember,
		isAdmin:  isAdmin,
		cachedAt: time.Now(),
	})

	return isMember, nil
}

// InvalidateUser clears all cached permissions for a user (call on logout).
func (s *GuildAuthService) InvalidateUser(userID string) {
	s.cache.invalidate(userID)
}

// --- cache methods ---

func (c *guildPermissionCache) get(userID, guildID string) (cachedPermission, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if userGuilds, ok := c.data[userID]; ok {
		if perm, ok := userGuilds[guildID]; ok {
			return perm, true
		}
	}
	return cachedPermission{}, false
}

func (c *guildPermissionCache) set(userID, guildID string, perm cachedPermission) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.data[userID]; !ok {
		c.data[userID] = make(map[string]cachedPermission)
	}
	c.data[userID][guildID] = perm
}

func (c *guildPermissionCache) invalidate(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, userID)
}
