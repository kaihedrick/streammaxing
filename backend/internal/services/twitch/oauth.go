package twitch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// OAuthService handles Twitch OAuth 2.0 flows
type OAuthService struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// NewOAuthService creates a new Twitch OAuth service with the given credentials.
func NewOAuthService(clientID, clientSecret, apiBaseURL string) *OAuthService {
	return &OAuthService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  apiBaseURL + "/api/auth/twitch/callback",
	}
}

// GetAuthURL generates the Twitch OAuth authorization URL
func (s *OAuthService) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":     {s.ClientID},
		"redirect_uri":  {s.RedirectURI},
		"response_type": {"code"},
		"scope":         {"user:read:email"},
		"state":         {state},
	}
	return "https://id.twitch.tv/oauth2/authorize?" + params.Encode()
}

// TokenResponse represents the Twitch OAuth token response
type TokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	TokenType    string   `json:"token_type"`
	Scope        []string `json:"scope"`
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

	resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("twitch oauth error (%d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// TwitchUser represents a Twitch user from the API
type TwitchUser struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	ProfileImageURL string `json:"profile_image_url"`
}

// GetUser fetches the authenticated Twitch user's information
func (s *OAuthService) GetUser(accessToken string) (*TwitchUser, error) {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", s.ClientID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch user (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		Data []TwitchUser `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no user data returned")
	}

	return &result.Data[0], nil
}

// RefreshToken refreshes an expired Twitch access token
func (s *OAuthService) RefreshToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {s.ClientID},
		"client_secret": {s.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", data)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to refresh token (%d): %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}
