package config

import (
	"fmt"
	"log"
	"os"

	"github.com/yourusername/streammaxing/internal/services/secrets"
)

// Config holds all application configuration.
// In production, secrets are loaded from AWS Secrets Manager.
// In development, everything comes from environment variables.
type Config struct {
	// Database
	DatabaseURL string

	// Discord
	DiscordClientID     string
	DiscordClientSecret string
	DiscordBotToken     string
	DiscordRedirectURI  string

	// Twitch
	TwitchClientID      string
	TwitchClientSecret  string
	TwitchWebhookSecret string

	// App (non-secret)
	APIBaseURL  string
	FrontendURL string
	JWTSecret   string
	Environment string
	LogLevel    string

	// AWS
	KMSKeyID string
}

// Load reads all configuration from the appropriate source.
// Production: secrets from AWS Secrets Manager, non-secrets from env vars.
// Development: everything from environment variables (loaded from .env).
func Load() (*Config, error) {
	cfg := &Config{
		// Non-secret config always comes from env vars
		APIBaseURL:  os.Getenv("API_BASE_URL"),
		FrontendURL: os.Getenv("FRONTEND_URL"),
		Environment: os.Getenv("ENVIRONMENT"),
		LogLevel:    os.Getenv("LOG_LEVEL"),
		KMSKeyID:    os.Getenv("KMS_KEY_ID"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	// Construct Discord redirect URI
	cfg.DiscordRedirectURI = os.Getenv("DISCORD_REDIRECT_URI")
	if cfg.DiscordRedirectURI == "" && cfg.APIBaseURL != "" {
		cfg.DiscordRedirectURI = cfg.APIBaseURL + "/api/auth/discord/callback"
	}

	// Try loading secrets from Secrets Manager in production
	if cfg.Environment == "production" {
		if err := cfg.loadFromSecretsManager(); err != nil {
			log.Printf("[CONFIG_WARN] Failed to load from Secrets Manager, falling back to env vars: %v", err)
			cfg.loadFromEnvVars()
		}
	} else {
		cfg.loadFromEnvVars()
	}

	// Validate JWT secret strength
	if err := cfg.validateJWTSecret(); err != nil {
		if cfg.IsProduction() {
			return nil, fmt.Errorf("JWT secret validation failed: %w", err)
		}
		log.Printf("[CONFIG_WARN] %v (non-production, continuing anyway)", err)
	}

	return cfg, nil
}

// validateJWTSecret checks that the JWT secret meets minimum security requirements.
// A 256-bit (32-byte) secret is required for HS256 signing.
func (c *Config) validateJWTSecret() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is empty — authentication will not work")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET is only %d bytes; minimum 32 bytes (256-bit) required for secure HS256 signing. Generate one with: openssl rand -base64 32", len(c.JWTSecret))
	}
	return nil
}

// loadFromSecretsManager loads secrets from AWS Secrets Manager.
func (c *Config) loadFromSecretsManager() error {
	mgr, err := secrets.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create secrets manager: %w", err)
	}

	// JWT Secret
	jwtSecret, err := mgr.GetJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to get JWT secret: %w", err)
	}
	c.JWTSecret = jwtSecret

	// Discord OAuth
	discordOAuth, err := mgr.GetDiscordOAuth()
	if err != nil {
		return fmt.Errorf("failed to get Discord OAuth: %w", err)
	}
	c.DiscordClientID = discordOAuth.ClientID
	c.DiscordClientSecret = discordOAuth.ClientSecret
	c.DiscordBotToken = discordOAuth.BotToken

	// Twitch OAuth
	twitchOAuth, err := mgr.GetTwitchOAuth()
	if err != nil {
		return fmt.Errorf("failed to get Twitch OAuth: %w", err)
	}
	c.TwitchClientID = twitchOAuth.ClientID
	c.TwitchClientSecret = twitchOAuth.ClientSecret
	c.TwitchWebhookSecret = twitchOAuth.WebhookSecret

	// Database URL still from env var (not in Secrets Manager yet)
	// In production, this comes from Lambda environment configuration
	if c.DatabaseURL == "" {
		c.DatabaseURL = os.Getenv("DATABASE_URL")
	}

	log.Println("[CONFIG] Loaded secrets from AWS Secrets Manager")
	return nil
}

// loadFromEnvVars loads all secrets from environment variables (development).
func (c *Config) loadFromEnvVars() {
	c.JWTSecret = os.Getenv("JWT_SECRET")
	c.DiscordClientID = os.Getenv("DISCORD_CLIENT_ID")
	c.DiscordClientSecret = os.Getenv("DISCORD_CLIENT_SECRET")
	c.DiscordBotToken = os.Getenv("DISCORD_BOT_TOKEN")
	c.TwitchClientID = os.Getenv("TWITCH_CLIENT_ID")
	c.TwitchClientSecret = os.Getenv("TWITCH_CLIENT_SECRET")
	c.TwitchWebhookSecret = os.Getenv("TWITCH_WEBHOOK_SECRET")

	log.Println("[CONFIG] Loaded secrets from environment variables")
}

// IsProduction returns true if running in production.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}
