package database

import "gorm.io/gorm"

// migrateSettingsUniqueKey leaves one settings row per key and makes the key unique (#889).
// The migration history seeded line_stroke twice, and the settings table never had the unique
// key that models.SettingEntry declares.
//
// The row kept is the live row with the highest id, which is the value LoadSettings ends up
// using. A key with only soft-deleted rows keeps its highest id. The statements are portable, so
// the migration runs on SQLite and PostgreSQL alike; on PostgreSQL the index usually exists already.
func migrateSettingsUniqueKey(tx *gorm.DB) error {
	statements := []string{
		`DELETE FROM settings WHERE deleted_at IS NOT NULL
			AND setting_key_name IN (SELECT setting_key_name FROM settings WHERE deleted_at IS NULL)`,
		`DELETE FROM settings WHERE id NOT IN (SELECT MAX(id) FROM settings GROUP BY setting_key_name)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_setting_key_name ON settings (setting_key_name)`,
	}
	for _, stmt := range statements {
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
