package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// APIClient handles Discord Bot API calls
type APIClient struct {
	BotToken   string
	httpClient *http.Client
}

// NewAPIClient creates a new Discord API client
func NewAPIClient() *APIClient {
	return &APIClient{
		BotToken:   os.Getenv("DISCORD_BOT_TOKEN"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// doRequest executes a Discord API request with rate limit handling
func (c *APIClient) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bot "+c.BotToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			wait, _ := strconv.Atoi(retryAfter)
			log.Printf("[DISCORD_API] Rate limited, retrying after %d seconds", wait)
			time.Sleep(time.Duration(wait) * time.Second)
			resp.Body.Close()
			return c.doRequest(req)
		}
	}

	return resp, nil
}

// Channel represents a Discord channel
type Channel struct {
	ID       string `json:"id"`
	Type     int    `json:"type"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// GetGuildChannels fetches text channels from a guild
func (c *APIClient) GetGuildChannels(guildID string) ([]Channel, error) {
	reqURL := fmt.Sprintf("https://discord.com/api/guilds/%s/channels", guildID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch channels (%d): %s", resp.StatusCode, body)
	}

	var allChannels []Channel
	if err := json.NewDecoder(resp.Body).Decode(&allChannels); err != nil {
		return nil, err
	}

	// Filter to text (0) and announcement (5) channels
	var textChannels []Channel
	for _, ch := range allChannels {
		if ch.Type == 0 || ch.Type == 5 {
			textChannels = append(textChannels, ch)
		}
	}

	return textChannels, nil
}

// Role represents a Discord role
type Role struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    int    `json:"color"`
	Position int    `json:"position"`
}

// GetGuildRoles fetches roles from a guild
func (c *APIClient) GetGuildRoles(guildID string) ([]Role, error) {
	reqURL := fmt.Sprintf("https://discord.com/api/guilds/%s/roles", guildID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch roles: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch roles (%d): %s", resp.StatusCode, body)
	}

	var roles []Role
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, err
	}

	return roles, nil
}

// CheckGuildMembership checks if a user is a member of a guild
func (c *APIClient) CheckGuildMembership(guildID, userID string) (bool, error) {
	reqURL := fmt.Sprintf("https://discord.com/api/guilds/%s/members/%s", guildID, userID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// DiscordMessage represents a message to send via the Discord API
type DiscordMessage struct {
	Content string          `json:"content,omitempty"`
	Embeds  []*DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	URL         string          `json:"url,omitempty"`
	Color       int             `json:"color,omitempty"`
	Thumbnail   *DiscordImage   `json:"thumbnail,omitempty"`
	Image       *DiscordImage   `json:"image,omitempty"`
	Fields      []DiscordField  `json:"fields,omitempty"`
	Footer      *DiscordFooter  `json:"footer,omitempty"`
	Timestamp   string          `json:"timestamp,omitempty"`
}

// DiscordImage represents an embed image
type DiscordImage struct {
	URL string `json:"url"`
}

// DiscordField represents an embed field
type DiscordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DiscordFooter represents an embed footer
type DiscordFooter struct {
	Text string `json:"text"`
}

// SendMessage sends a message to a Discord channel
func (c *APIClient) SendMessage(channelID string, message *DiscordMessage) error {
	reqURL := fmt.Sprintf("https://discord.com/api/channels/%s/messages", channelID)

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord send error (%d): %s", resp.StatusCode, respBody)
	}

	return nil
}
