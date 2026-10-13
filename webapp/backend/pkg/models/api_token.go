package models

import (
	"time"

	"gorm.io/gorm"
)

// Token scopes. admin satisfies every requirement; read and collector satisfy only their own.
const (
	APITokenScopeRead      = "read"
	APITokenScopeCollector = "collector"
	APITokenScopeAdmin     = "admin"
)

// IsValidAPITokenScope reports whether scope is one a token can be issued with.
func IsValidAPITokenScope(scope string) bool {
	switch scope {
	case APITokenScopeRead, APITokenScopeCollector, APITokenScopeAdmin:
		return true
	}
	return false
}

// ApiToken represents a named, scoped API token for programmatic access (mobile clients, read-only
// integrations, per-host collectors). The master token in config (web.auth.token) is separate and
// always has admin scope.
type ApiToken struct {
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// LastUsedAt is updated on each successful authentication for audit purposes
	LastUsedAt *time.Time `json:"last_used_at"`

	// ExpiresAt is the optional expiry date; nil means the token never expires
	ExpiresAt *time.Time `json:"expires_at"`

	// Name is a human-readable label for this token (e.g., "NAS Collector", "Monitoring Script")
	Name string `json:"name" gorm:"not null"`

	// TokenHash stores the SHA-256 hash of the plaintext token; never stored in plaintext
	TokenHash string `json:"-" gorm:"uniqueIndex;not null"`

	// Scope is one of the APITokenScope* values. The column has no default, so a token created
	// without a scope holds an empty, invalid one and cannot authenticate. Migration
	// m20261013000000 dropped the legacy 'full' default.
	Scope string `json:"scope"`

	ID uint `json:"id" gorm:"primaryKey"`

	// Revoked allows soft-revoking a token without deleting it
	Revoked bool `json:"revoked" gorm:"default:false"`
}

func (ApiToken) TableName() string {
	return "api_tokens"
}
