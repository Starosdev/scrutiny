package m20260411000000

// AttributeOverride is the migration-scoped struct after adding a unique
// constraint on (protocol, attribute_id, wwn). The actual migration uses raw
// SQL to remove any pre-existing duplicates before creating the unique index,
// which GORM AutoMigrate cannot do safely on its own.
// This struct is kept for reference only; the migration logic is in the
// registered Migrate function in scrutiny_repository_migrations.go.
