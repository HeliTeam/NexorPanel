package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/nexor/panel/database"
	"github.com/nexor/panel/database/model"
)

// APIKeyService manages hashed API keys for bots and automation.
type APIKeyService struct{}

const apiKeyPrefix = "nxr_"

// hashAPIKey returns hex-encoded SHA-256 of the plaintext key.
func hashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// CreateAPIKey persists a new key and returns the plaintext once.
func (s *APIKeyService) CreateAPIKey(permissions []string) (plain string, err error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plain = apiKeyPrefix + hex.EncodeToString(raw)
	permsJSON, err := json.Marshal(permissions)
	if err != nil {
		return "", err
	}
	row := &model.APIKey{
		KeyHash:     hashAPIKey(plain),
		Permissions: string(permsJSON),
	}
	db := database.GetDB()
	if err := db.Create(row).Error; err != nil {
		return "", err
	}
	return plain, nil
}

// VerifyAPIKey finds an API key row by plaintext and updates last_used_at.
func (s *APIKeyService) VerifyAPIKey(plain string) (*model.APIKey, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" || !strings.HasPrefix(plain, apiKeyPrefix) {
		return nil, errors.New("invalid api key")
	}
	h := hashAPIKey(plain)
	db := database.GetDB()
	var row model.APIKey
	if err := db.Where("key_hash = ?", h).First(&row).Error; err != nil {
		return nil, errors.New("invalid api key")
	}
	_ = db.Model(&row).Update("last_used_at", time.Now().Unix()).Error
	return &row, nil
}

// ScopeAllows reports whether permission JSON allows the required scope (or "*").
func ScopeAllows(permissionsJSON string, required string) bool {
	if permissionsJSON == "" || required == "" {
		return false
	}
	var perms []string
	if err := json.Unmarshal([]byte(permissionsJSON), &perms); err != nil {
		return false
	}
	for _, p := range perms {
		if p == "*" || p == required {
			return true
		}
	}
	return false
}
