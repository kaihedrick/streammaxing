package discord

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// OAuthService handles Discord OAuth 2.0 flows
type OAuthService struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// NewOAuthService creates a new Discord OAuth service with the given credentials.
func NewOAuthService(clientID, clientSecret, redirectURI string) *OAuthService {
	return &OAuthService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
	}
}

// GetAuthURL generates the Discord OAuth authorization URL
func (s *OAuthService) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":     {s.ClientID},
		"redirect_uri":  {s.RedirectURI},
		"response_type": {"code"},
		"scope":         {"identify guilds"},
		"state":         {state},
	}
	return "https://discord.com/api/oauth2/authorize?" + params.Encode()
}

// TokenResponse represents the Discord OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// ExchangeCode exchanges an authorization code for an access token
func (s *OAuthService) ExchangeCode(code string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {s.ClientID},
		"client_secret": {s.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.RedirectURI},
	}

	resp, err := http.PostForm("https://discord.com/api/oauth2/token", data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord oauth error (%d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// DiscordUser represents a Discord user from the API
type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// GetUser fetches the authenticated user's information
func (s *OAuthService) GetUser(accessToken string) (*DiscordUser, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch user (%d): %s", resp.StatusCode, body)
	}

	var user DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &user, nil
}

// DiscordGuild represents a Discord guild from the user's guild list
type DiscordGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Owner       bool   `json:"owner"`
	Permissions int64  `json:"permissions"`
}

// GetUserGuilds fetches the authenticated user's guilds
func (s *OAuthService) GetUserGuilds(accessToken string) ([]DiscordGuild, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch guilds: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord API error (%d): %s", resp.StatusCode, string(body))
	}

	var guilds []DiscordGuild
	if err := json.Unmarshal(body, &guilds); err != nil {
		return nil, fmt.Errorf("failed to decode guilds (response: %s): %w", string(body), err)
	}

	return guilds, nil
}

// GetBotInstallURL generates the bot installation URL for a guild
func (s *OAuthService) GetBotInstallURL(guildID string) string {
	return fmt.Sprintf(
		"https://discord.com/api/oauth2/authorize?client_id=%s&scope=bot&permissions=149504&guild_id=%s",
		s.ClientID, guildID,
	)
}
