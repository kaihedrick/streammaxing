package twitch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// APIClient handles Twitch API calls with automatic app access token management
type APIClient struct {
	ClientID       string
	ClientSecret   string
	appAccessToken string
	tokenExpiry    time.Time
	mu             sync.RWMutex
	httpClient     *http.Client
}

// NewAPIClient creates a new Twitch API client with the given credentials.
func NewAPIClient(clientID, clientSecret string) *APIClient {
	return &APIClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAppAccessToken returns a valid app access token, refreshing if expired
func (c *APIClient) GetAppAccessToken() (string, error) {
	c.mu.RLock()
	if c.appAccessToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.appAccessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.appAccessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.appAccessToken, nil
	}

	data := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"client_credentials"},
	}

	resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", data)
	if err != nil {
		return "", fmt.Errorf("failed to get app access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get app access token (%d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	c.appAccessToken = tokenResp.AccessToken
	// Refresh 5 minutes early
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-300) * time.Second)

	return c.appAccessToken, nil
}

// StreamData represents stream information from the Twitch API
type StreamData struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	GameID       string    `json:"game_id"`
	GameName     string    `json:"game_name"`
	Title        string    `json:"title"`
	ViewerCount  int       `json:"viewer_count"`
	ThumbnailURL string    `json:"thumbnail_url"`
	StartedAt    time.Time `json:"started_at"`
}

// GetStreamData fetches current stream data for a broadcaster
func (c *APIClient) GetStreamData(broadcasterID string) (*StreamData, error) {
	token, err := c.GetAppAccessToken()
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_id=%s", broadcasterID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", c.ClientID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stream data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch stream data (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		Data []StreamData `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode stream data: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("stream not found or offline")
	}

	return &result.Data[0], nil
}
