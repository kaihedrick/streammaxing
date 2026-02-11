package twitch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// EventSubService manages Twitch EventSub subscriptions
type EventSubService struct {
	apiClient     *APIClient
	apiBaseURL    string
	webhookSecret string
}

// NewEventSubService creates a new EventSub service with the given configuration.
func NewEventSubService(apiClient *APIClient, apiBaseURL, webhookSecret string) *EventSubService {
	return &EventSubService{
		apiClient:     apiClient,
		apiBaseURL:    apiBaseURL,
		webhookSecret: webhookSecret,
	}
}

// Subscription represents a Twitch EventSub subscription
type Subscription struct {
	ID        string                 `json:"id"`
	Status    string                 `json:"status"`
	Type      string                 `json:"type"`
	Version   string                 `json:"version"`
	Condition map[string]interface{} `json:"condition"`
	Transport Transport              `json:"transport"`
	CreatedAt string                 `json:"created_at"`
}

// Transport represents the webhook transport for EventSub
type Transport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret,omitempty"`
}

// CreateSubscriptionRequest represents the request to create an EventSub subscription
type CreateSubscriptionRequest struct {
	Type      string                 `json:"type"`
	Version   string                 `json:"version"`
	Condition map[string]interface{} `json:"condition"`
	Transport Transport              `json:"transport"`
}

// CreateStreamOnlineSubscription creates a stream.online EventSub subscription
func (s *EventSubService) CreateStreamOnlineSubscription(broadcasterID string) (*Subscription, error) {
	token, err := s.apiClient.GetAppAccessToken()
	if err != nil {
		return nil, err
	}

	webhookURL := s.apiBaseURL + "/webhooks/twitch"

	reqBody := CreateSubscriptionRequest{
		Type:    "stream.online",
		Version: "1",
		Condition: map[string]interface{}{
			"broadcaster_user_id": broadcasterID,
		},
		Transport: Transport{
			Method:   "webhook",
			Callback: webhookURL,
			Secret:   s.webhookSecret,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.twitch.tv/helix/eventsub/subscriptions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", s.apiClient.ClientID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.apiClient.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("subscription already exists for broadcaster %s", broadcasterID)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create subscription (%d): %s", resp.StatusCode, respBody)
	}

	var result struct {
		Data []Subscription `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode subscription response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no subscription data returned")
	}

	return &result.Data[0], nil
}

// DeleteSubscription deletes an EventSub subscription
func (s *EventSubService) DeleteSubscription(subscriptionID string) error {
	token, err := s.apiClient.GetAppAccessToken()
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("https://api.twitch.tv/helix/eventsub/subscriptions?id=%s", subscriptionID)
	req, err := http.NewRequest("DELETE", reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", s.apiClient.ClientID)

	resp, err := s.apiClient.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete subscription (%d)", resp.StatusCode)
	}

	return nil
}

// ListSubscriptions lists all EventSub subscriptions
func (s *EventSubService) ListSubscriptions() ([]Subscription, error) {
	token, err := s.apiClient.GetAppAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/eventsub/subscriptions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", s.apiClient.ClientID)

	resp, err := s.apiClient.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list subscriptions (%d)", resp.StatusCode)
	}

	var result struct {
		Data []Subscription `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}
