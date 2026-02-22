package models

import (
	"time"

	"gorm.io/gorm"
)

// ApiToken represents a stored API token for authenticating programmatic access.
// In Phase 1, the primary auth mechanism is the config-based master token;
// this model provides the infrastructure for future multi-token management
// (e.g., per-collector tokens, revocable tokens with expiry).
type ApiToken struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// Name is a human-readable label for this token (e.g., "NAS Collector", "Monitoring Script")
	Name string `json:"name" gorm:"not null"`

	// TokenHash stores the SHA-256 hash of the plaintext token; never stored in plaintext
	TokenHash string `json:"-" gorm:"uniqueIndex;not null"`

	// LastUsedAt is updated on each successful authentication for audit purposes
	LastUsedAt *time.Time `json:"last_used_at"`

	// ExpiresAt is the optional expiry date; nil means the token never expires
	ExpiresAt *time.Time `json:"expires_at"`

	// Revoked allows soft-revoking a token without deleting it
	Revoked bool `json:"revoked" gorm:"default:false"`

	// Scope controls what the token can access (future use: "full", "collector", "read-only")
	Scope string `json:"scope" gorm:"default:'full'"`
}

func (ApiToken) TableName() string {
	return "api_tokens"
}
