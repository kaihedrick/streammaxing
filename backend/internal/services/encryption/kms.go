package encryption

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// Service provides encryption/decryption for sensitive data using AWS KMS.
// In development mode (no KMS key ID), it falls back to base64 encoding
// which is NOT secure but allows local testing without AWS infrastructure.
type Service struct {
	client *kms.Client
	keyID  string
	isDev  bool
}

var (
	instance *Service
	once     sync.Once
	initErr  error
)

// NewService creates or returns the singleton encryption service.
// Pass the KMS key ID from the centralized config (empty string enables dev mode).
func NewService(kmsKeyID string) (*Service, error) {
	once.Do(func() {
		if kmsKeyID == "" {
			// Development mode - no real encryption
			instance = &Service{isDev: true}
			return
		}

		cfg, err := awsconfig.LoadDefaultConfig(context.Background())
		if err != nil {
			initErr = fmt.Errorf("failed to load AWS config: %w", err)
			return
		}

		instance = &Service{
			client: kms.NewFromConfig(cfg),
			keyID:  kmsKeyID,
			isDev:  false,
		}
	})

	if initErr != nil {
		return nil, initErr
	}
	return instance, nil
}

// Encrypt encrypts plaintext using AWS KMS and returns a base64-encoded ciphertext.
func (s *Service) Encrypt(plaintext string) (string, error) {
	if s.isDev {
		// Development fallback: base64 encode with a prefix to identify encrypted values
		return "dev:" + base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
	}

	result, err := s.client.Encrypt(context.Background(), &kms.EncryptInput{
		KeyId:     &s.keyID,
		Plaintext: []byte(plaintext),
	})
	if err != nil {
		return "", fmt.Errorf("KMS encryption failed: %w", err)
	}

	return "kms:" + base64.StdEncoding.EncodeToString(result.CiphertextBlob), nil
}

// Decrypt decrypts a base64-encoded ciphertext using AWS KMS.
func (s *Service) Decrypt(ciphertext string) (string, error) {
	// Handle dev-mode encoded values
	if len(ciphertext) > 4 && ciphertext[:4] == "dev:" {
		decoded, err := base64.StdEncoding.DecodeString(ciphertext[4:])
		if err != nil {
			return "", fmt.Errorf("failed to decode dev ciphertext: %w", err)
		}
		return string(decoded), nil
	}

	// Handle KMS-encrypted values
	if len(ciphertext) > 4 && ciphertext[:4] == "kms:" {
		ciphertext = ciphertext[4:]
	}

	// If it doesn't have a prefix, it might be a legacy plaintext token
	// Try to decode as base64 first; if it fails, return as-is (plaintext legacy)
	ciphertextBlob, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		// Likely a plaintext legacy token - return as-is
		return ciphertext, nil
	}

	if s.isDev {
		// In dev mode with KMS-prefixed data, just decode the base64
		return string(ciphertextBlob), nil
	}

	result, err := s.client.Decrypt(context.Background(), &kms.DecryptInput{
		CiphertextBlob: ciphertextBlob,
	})
	if err != nil {
		return "", fmt.Errorf("KMS decryption failed: %w", err)
	}

	return string(result.Plaintext), nil
}

// IsEncrypted checks if a token value appears to be encrypted (has a prefix).
func IsEncrypted(value string) bool {
	if len(value) < 4 {
		return false
	}
	return value[:4] == "kms:" || value[:4] == "dev:"
}
