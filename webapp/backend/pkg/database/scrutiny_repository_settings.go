package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/mitchellh/mapstructure"
	"strings"
)

// settingsMu protects Viper appConfig access in LoadSettings/SaveSettings.
// Must be package-level because multiple scrutinyRepository instances share the same Viper instance.
var settingsMu sync.Mutex

// LoadSettings will retrieve settings from the database, store them in the AppConfig object, and return a Settings struct
func (sr *scrutinyRepository) LoadSettings(ctx context.Context) (*models.Settings, error) {
	settingsEntries := []models.SettingEntry{}
	if err := sr.gormClient.WithContext(ctx).Find(&settingsEntries).Error; err != nil {
		return nil, fmt.Errorf("Could not get settings from DB: %v", err)
	}

	settingsMu.Lock()
	defer settingsMu.Unlock()

	// store retrieved settings in the AppConfig obj
	for _, settingsEntry := range settingsEntries {
		configKey := fmt.Sprintf("%s.%s", config.DB_USER_SETTINGS_SUBKEY, settingsEntry.SettingKeyName)

		if settingsEntry.SettingDataType == "numeric" {
			sr.appConfig.SetDefault(configKey, settingsEntry.SettingValueNumeric)
		} else if settingsEntry.SettingDataType == "string" {
			sr.appConfig.SetDefault(configKey, settingsEntry.SettingValueString)
		} else if settingsEntry.SettingDataType == "bool" {
			sr.appConfig.SetDefault(configKey, settingsEntry.SettingValueBool)
		}
	}

	// unmarshal the dbsetting object data to a settings object.
	var settings models.Settings
	err := sr.appConfig.UnmarshalKey(config.DB_USER_SETTINGS_SUBKEY, &settings)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// GetSettingValue retrieves a single setting value by key name.
func (sr *scrutinyRepository) GetSettingValue(ctx context.Context, key string) (string, error) {
	var entry models.SettingEntry
	result := sr.gormClient.WithContext(ctx).Where("setting_key_name = ?", key).First(&entry)
	if result.Error != nil {
		return "", result.Error
	}
	if entry.SettingDataType == "string" {
		return entry.SettingValueString, nil
	}
	if entry.SettingDataType == "numeric" {
		return fmt.Sprintf("%d", entry.SettingValueNumeric), nil
	}
	if entry.SettingDataType == "bool" {
		return fmt.Sprintf("%t", entry.SettingValueBool), nil
	}
	return entry.SettingValueString, nil
}

// SetSettingValue sets a single setting value by key name (upsert).
func (sr *scrutinyRepository) SetSettingValue(ctx context.Context, key string, value string) error {
	var entry models.SettingEntry
	result := sr.gormClient.WithContext(ctx).Where("setting_key_name = ?", key).First(&entry)
	if result.Error != nil {
		// Entry doesn't exist, create it
		entry = models.SettingEntry{
			SettingKeyName:     key,
			SettingDataType:    "string",
			SettingValueString: value,
		}
		return sr.gormClient.WithContext(ctx).Create(&entry).Error
	}
	// Entry exists, update the string value
	entry.SettingValueString = value
	return sr.gormClient.WithContext(ctx).Model(&entry).Update("setting_value_string", value).Error
}

// testing
// curl -d '{"metrics": { "notify_level": 5, "status_filter_attributes": 5, "status_threshold": 5 }}' -H "Content-Type: application/json" -X POST http://localhost:9090/api/settings
// SaveSettings will update settings in AppConfig object, then save the settings to the database.
func (sr *scrutinyRepository) SaveSettings(ctx context.Context, settings models.Settings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	//save the entries to the appconfig
	settingsMap := &map[string]interface{}{}
	err := mapstructure.Decode(settings, &settingsMap)
	if err != nil {
		return err
	}
	settingsWrapperMap := map[string]interface{}{}
	settingsWrapperMap[config.DB_USER_SETTINGS_SUBKEY] = *settingsMap
	err = sr.appConfig.MergeConfigMap(settingsWrapperMap)
	if err != nil {
		return err
	}
	sr.logger.Debugf("after merge settings: %v", sr.appConfig.AllSettings())
	//retrieve current settings from the database
	settingsEntries := []models.SettingEntry{}
	if err := sr.gormClient.WithContext(ctx).Find(&settingsEntries).Error; err != nil {
		return fmt.Errorf("Could not get settings from DB: %v", err)
	}

	//update settingsEntries
	for ndx, settingsEntry := range settingsEntries {
		configKey := fmt.Sprintf("%s.%s", config.DB_USER_SETTINGS_SUBKEY, strings.ToLower(settingsEntry.SettingKeyName))

		if settingsEntry.SettingDataType == "numeric" {
			settingsEntries[ndx].SettingValueNumeric = sr.appConfig.GetInt(configKey)
		} else if settingsEntry.SettingDataType == "string" {
			settingsEntries[ndx].SettingValueString = sr.appConfig.GetString(configKey)
		} else if settingsEntry.SettingDataType == "bool" {
			settingsEntries[ndx].SettingValueBool = sr.appConfig.GetBool(configKey)
		}

		// store in database.
		//TODO: this should be `sr.gormClient.Updates(&settingsEntries).Error`
		err := sr.gormClient.Model(&models.SettingEntry{}).Where([]uint{settingsEntry.ID}).Select("setting_value_numeric", "setting_value_string", "setting_value_bool").Updates(settingsEntries[ndx]).Error
		if err != nil {
			return err
		}

	}
	return nil
}
