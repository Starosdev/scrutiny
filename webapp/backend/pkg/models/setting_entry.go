package models

import (
	"gorm.io/gorm"
)

// SettingEntry matches a setting row in the database
type SettingEntry struct {
	gorm.Model
	SettingKeyName        string `json:"setting_key_name" gorm:"unique;not null"`
	SettingKeyDescription string `json:"setting_key_description"`
	SettingDataType       string `json:"setting_data_type"`
	SettingValueString    string `json:"setting_value_string"`
	SettingValueNumeric   int    `json:"setting_value_numeric"`
	SettingValueBool      bool   `json:"setting_value_bool"`
}

func (s SettingEntry) TableName() string {
	return "settings"
}
