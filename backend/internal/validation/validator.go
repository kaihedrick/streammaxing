package validation

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Discord IDs are snowflakes: 17-20 digit numeric strings
	snowflakeRegex = regexp.MustCompile(`^\d{17,20}$`)
)

// Validator provides input validation for API endpoints.
type Validator struct{}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateGuildID checks that a Discord guild ID is a valid snowflake.
func (v *Validator) ValidateGuildID(guildID string) error {
	if !snowflakeRegex.MatchString(guildID) {
		return fmt.Errorf("invalid guild ID format")
	}
	return nil
}

// ValidateUserID checks that a Discord user ID is a valid snowflake.
func (v *Validator) ValidateUserID(userID string) error {
	if !snowflakeRegex.MatchString(userID) {
		return fmt.Errorf("invalid user ID format")
	}
	return nil
}

// ValidateChannelID checks that a Discord channel ID is a valid snowflake.
func (v *Validator) ValidateChannelID(channelID string) error {
	if channelID == "" {
		return nil // Channel ID can be empty (not yet configured)
	}
	if !snowflakeRegex.MatchString(channelID) {
		return fmt.Errorf("invalid channel ID format")
	}
	return nil
}

// ValidateTemplateContent checks message template content for injection attempts.
func (v *Validator) ValidateTemplateContent(content string) error {
	if len(content) > 4000 {
		return fmt.Errorf("template content too long (max 4000 characters)")
	}

	// Check for script injection attempts
	dangerous := []string{"<script", "javascript:", "onerror=", "onclick=", "onload=", "onmouseover="}
	contentLower := strings.ToLower(content)
	for _, pattern := range dangerous {
		if strings.Contains(contentLower, pattern) {
			return fmt.Errorf("template contains potentially dangerous content")
		}
	}

	return nil
}

// ValidateCustomContent validates custom notification text.
func (v *Validator) ValidateCustomContent(content string) error {
	if len(content) > 2000 {
		return fmt.Errorf("custom content too long (max 2000 characters)")
	}

	dangerous := []string{"<script", "javascript:", "onerror=", "onclick="}
	contentLower := strings.ToLower(content)
	for _, pattern := range dangerous {
		if strings.Contains(contentLower, pattern) {
			return fmt.Errorf("content contains potentially dangerous characters")
		}
	}

	return nil
}

// SanitizeInput removes potentially dangerous characters from input.
func (v *Validator) SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")
	// Trim whitespace
	input = strings.TrimSpace(input)
	return input
}

// ValidateJSONSize checks if a request payload is within acceptable size limits.
func (v *Validator) ValidateJSONSize(data []byte) error {
	if len(data) > 1024*1024 { // 1MB max
		return fmt.Errorf("request payload too large (max 1MB)")
	}
	return nil
}

// ValidateInviteCode checks that an invite code is a valid hex string.
func (v *Validator) ValidateInviteCode(code string) error {
	if len(code) < 6 || len(code) > 32 {
		return fmt.Errorf("invalid invite code length")
	}
	matched, _ := regexp.MatchString(`^[a-f0-9]+$`, code)
	if !matched {
		return fmt.Errorf("invalid invite code format")
	}
	return nil
}
