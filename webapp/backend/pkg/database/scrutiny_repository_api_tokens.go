package database

import (
	"context"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"gorm.io/gorm"
)

// apiTokenTouchInterval limits last_used_at writes so a busy collector does not write on every request.
const apiTokenTouchInterval = time.Minute

// CreateAPIToken stores a new token. The caller supplies only the hash, never the plaintext.
func (sr *scrutinyRepository) CreateAPIToken(ctx context.Context, token *models.ApiToken) error {
	return sr.gormClient.WithContext(ctx).Create(token).Error
}

// ListAPITokens returns every token, revoked ones included, newest first.
func (sr *scrutinyRepository) ListAPITokens(ctx context.Context) ([]models.ApiToken, error) {
	var tokens []models.ApiToken
	if err := sr.gormClient.WithContext(ctx).Order("id DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// RevokeAPIToken marks a token revoked. The row stays for audit.
func (sr *scrutinyRepository) RevokeAPIToken(ctx context.Context, id uint) error {
	result := sr.gormClient.WithContext(ctx).
		Model(&models.ApiToken{}).
		Where("id = ?", id).
		Update("revoked", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetActiveAPITokenByHash looks up a token by its hash, skipping revoked and expired tokens.
func (sr *scrutinyRepository) GetActiveAPITokenByHash(ctx context.Context, tokenHash string) (*models.ApiToken, error) {
	var token models.ApiToken
	err := sr.gormClient.WithContext(ctx).
		Where("token_hash = ? AND revoked = ? AND (expires_at IS NULL OR expires_at > ?)", tokenHash, false, time.Now()).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// TouchAPIToken sets last_used_at, skipping the write when it was updated within the last minute.
func (sr *scrutinyRepository) TouchAPIToken(ctx context.Context, id uint) error {
	now := time.Now()
	return sr.gormClient.WithContext(ctx).
		Model(&models.ApiToken{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", id, now.Add(-apiTokenTouchInterval)).
		UpdateColumn("last_used_at", now).Error
}
