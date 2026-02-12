package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Manager provides centralized secrets management via AWS Secrets Manager.
// Falls back to environment variables in development when ENVIRONMENT != "production".
type Manager struct {
	client *secretsmanager.Client
	cache  map[string]cachedSecret
	mu     sync.RWMutex
	isDev  bool
}

type cachedSecret struct {
	value     string
	fetchedAt time.Time
}

// JWTSecret holds the JWT signing secret.
type JWTSecret struct {
	Secret string `json:"jwt_secret"`
}

// DiscordOAuth holds Discord OAuth credentials.
type DiscordOAuth struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	BotToken     string `json:"bot_token"`
}

// TwitchOAuth holds Twitch OAuth credentials.
type TwitchOAuth struct {
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	WebhookSecret string `json:"webhook_secret"`
}

var (
	instance *Manager
	once     sync.Once
	initErr  error
)

// NewManager creates or returns the singleton secrets manager.
func NewManager() (*Manager, error) {
	once.Do(func() {
		env := os.Getenv("ENVIRONMENT")
		if env != "production" {
			// Development mode: use environment variables
			instance = &Manager{
				cache: make(map[string]cachedSecret),
				isDev: true,
			}
			log.Println("[SECRETS] Using environment variables (development mode)")
			return
		}

		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			initErr = fmt.Errorf("failed to load AWS config for Secrets Manager: %w", err)
			return
		}

		instance = &Manager{
			client: secretsmanager.NewFromConfig(cfg),
			cache:  make(map[string]cachedSecret),
			isDev:  false,
		}
		log.Println("[SECRETS] Using AWS Secrets Manager (production mode)")
	})

	if initErr != nil {
		return nil, initErr
	}
	return instance, nil
}

// GetJWTSecret returns the JWT signing secret.
// Priority: JWT_SECRET env var (always available via SAM template) → Secrets Manager.
func (m *Manager) GetJWTSecret() (string, error) {
	// The JWT secret is injected as an env var by the SAM template in all
	// environments.  Prefer the env var so we don't require Secrets Manager
	// IAM permissions just for this secret.
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret, nil
	}

	if m.isDev {
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}

	// Fallback: try AWS Secrets Manager (requires IAM permissions + the secret to exist)
	var s JWTSecret
	raw, err := m.getSecret("streammaxing/jwt-secret")
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return "", fmt.Errorf("failed to parse JWT secret: %w", err)
	}
	return s.Secret, nil
}

// GetDiscordOAuth returns Discord OAuth credentials.
func (m *Manager) GetDiscordOAuth() (*DiscordOAuth, error) {
	if m.isDev {
		return &DiscordOAuth{
			ClientID:     os.Getenv("DISCORD_CLIENT_ID"),
			ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
			BotToken:     os.Getenv("DISCORD_BOT_TOKEN"),
		}, nil
	}

	var s DiscordOAuth
	raw, err := m.getSecret("streammaxing/discord-oauth")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("failed to parse Discord OAuth secret: %w", err)
	}
	return &s, nil
}

// GetTwitchOAuth returns Twitch OAuth credentials.
func (m *Manager) GetTwitchOAuth() (*TwitchOAuth, error) {
	if m.isDev {
		return &TwitchOAuth{
			ClientID:      os.Getenv("TWITCH_CLIENT_ID"),
			ClientSecret:  os.Getenv("TWITCH_CLIENT_SECRET"),
			WebhookSecret: os.Getenv("TWITCH_WEBHOOK_SECRET"),
		}, nil
	}

	var s TwitchOAuth
	raw, err := m.getSecret("streammaxing/twitch-oauth")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("failed to parse Twitch OAuth secret: %w", err)
	}
	return &s, nil
}

// getSecret fetches a secret from AWS Secrets Manager with a 5-minute cache.
func (m *Manager) getSecret(secretName string) (string, error) {
	// Check cache (TTL: 5 minutes)
	m.mu.RLock()
	if cached, ok := m.cache[secretName]; ok {
		if time.Since(cached.fetchedAt) < 5*time.Minute {
			m.mu.RUnlock()
			return cached.value, nil
		}
	}
	m.mu.RUnlock()

	// Fetch from Secrets Manager
	result, err := m.client.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch secret %s: %w", secretName, err)
	}

	value := *result.SecretString

	// Update cache
	m.mu.Lock()
	m.cache[secretName] = cachedSecret{
		value:     value,
		fetchedAt: time.Now(),
	}
	m.mu.Unlock()

	return value, nil
}

// InvalidateCache clears all cached secrets (call after secret rotation).
func (m *Manager) InvalidateCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]cachedSecret)
}
