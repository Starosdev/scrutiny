package m20260225000000

import (
	"time"

	"gorm.io/gorm"
)

// ApiToken is the migration-specific model for creating the api_tokens table.
// This is a snapshot of the model at migration time -- do not modify after release.
type ApiToken struct {
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	Name       string `gorm:"not null"`
	TokenHash  string `gorm:"uniqueIndex;not null"`
	Scope      string `gorm:"default:'full'"`
	ID         uint   `gorm:"primaryKey"`
	Revoked    bool   `gorm:"default:false"`
}

func (ApiToken) TableName() string {
	return "api_tokens"
}
