package database

import (
	"context"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/overrides"
)

// GetAttributeOverrides retrieves all attribute overrides from the database
func (sr *scrutinyRepository) GetAttributeOverrides(ctx context.Context) ([]models.AttributeOverride, error) {
	var dbOverrides []models.AttributeOverride
	if err := sr.gormClient.WithContext(ctx).Find(&dbOverrides).Error; err != nil {
		return nil, err
	}
	return dbOverrides, nil
}

// GetAttributeOverrideByID retrieves a single attribute override by its ID
func (sr *scrutinyRepository) GetAttributeOverrideByID(ctx context.Context, id uint) (*models.AttributeOverride, error) {
	var override models.AttributeOverride
	if err := sr.gormClient.WithContext(ctx).First(&override, id).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

// GetMergedOverrides retrieves overrides from both config file and database,
// merging them with database overrides taking precedence over config overrides.
func (sr *scrutinyRepository) GetMergedOverrides(ctx context.Context) []overrides.AttributeOverride {
	// Get config-based overrides
	configOverrides := overrides.ParseOverrides(sr.appConfig)

	// Get database overrides
	dbOverrides, err := sr.GetAttributeOverrides(ctx)
	if err != nil {
		// If DB fails, just use config overrides
		return configOverrides
	}

	// Convert DB overrides to overrides.AttributeOverride type
	convertedDBOverrides := models.ConvertToOverrides(dbOverrides)

	// Merge with DB overrides taking precedence
	return overrides.MergeOverrides(configOverrides, convertedDBOverrides)
}

// SaveAttributeOverride creates or updates an attribute override
// If the override has an ID, it will update; otherwise it will create
// Uses pointer so that GORM can populate the ID field after creation
func (sr *scrutinyRepository) SaveAttributeOverride(ctx context.Context, override *models.AttributeOverride) error {
	// Ensure source is set to "ui" for database-saved overrides
	override.Source = "ui"

	if override.ID == 0 {
		// Create new override
		return sr.gormClient.WithContext(ctx).Create(override).Error
	}
	// Update existing override
	return sr.gormClient.WithContext(ctx).Save(override).Error
}

// DeleteAttributeOverride removes an attribute override by ID
func (sr *scrutinyRepository) DeleteAttributeOverride(ctx context.Context, id uint) error {
	return sr.gormClient.WithContext(ctx).Delete(&models.AttributeOverride{}, id).Error
}
