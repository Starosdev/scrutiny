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

// GetAllOverridesForDisplay returns all active overrides for display in the settings UI.
// DB overrides (source: "ui") are returned as-is. Config file overrides (source: "config")
// are synthesized into models.AttributeOverride with ID=0, so the UI can show them as
// read-only entries. DB overrides take precedence: if a DB override matches the same
// (protocol, attribute_id, wwn) as a config override, only the DB version is returned.
func (sr *scrutinyRepository) GetAllOverridesForDisplay(ctx context.Context) ([]models.AttributeOverride, error) {
	dbOverrides, err := sr.GetAttributeOverrides(ctx)
	if err != nil {
		return nil, err
	}

	configOverrides := overrides.ParseOverrides(sr.appConfig)

	// Build a set of (protocol|attribute_id|wwn) keys already covered by DB overrides.
	dbKeys := make(map[string]struct{}, len(dbOverrides))
	for i := range dbOverrides {
		key := dbOverrides[i].Protocol + "|" + dbOverrides[i].AttributeId + "|" + dbOverrides[i].WWN
		dbKeys[key] = struct{}{}
	}

	// Append config overrides that are not shadowed by a DB override.
	result := make([]models.AttributeOverride, 0, len(dbOverrides)+len(configOverrides))
	result = append(result, dbOverrides...)

	for _, co := range configOverrides {
		key := co.Protocol + "|" + co.AttributeId + "|" + co.WWN
		if _, exists := dbKeys[key]; exists {
			continue // DB override takes precedence; skip the config version
		}
		var warnAbove *int64
		var failAbove *int64
		if co.WarnAbove != nil {
			v := *co.WarnAbove
			warnAbove = &v
		}
		if co.FailAbove != nil {
			v := *co.FailAbove
			failAbove = &v
		}
		result = append(result, models.AttributeOverride{
			Protocol:    co.Protocol,
			AttributeId: co.AttributeId,
			WWN:         co.WWN,
			Action:      string(co.Action),
			Status:      co.Status,
			WarnAbove:   warnAbove,
			FailAbove:   failAbove,
			Source:      "config",
		})
	}

	return result, nil
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
