package database

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260924000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

// schemaModels lists one model per relational table, in an order that satisfies no foreign keys
// (none are enforced). It is the current schema as Go types: a new PostgreSQL database is created
// from it, and the parity test checks it against the schema the SQLite migration history builds.
// A migration that adds a table must add its model here.
func schemaModels() []interface{} {
	return []interface{}{
		&models.Device{},
		&models.SettingEntry{},
		&models.AttributeOverride{},
		&models.ApiToken{},
		&models.NotifyUrl{},
		&models.ZFSPool{},
		&models.ZFSVdev{},
		&models.MDADMArray{},
		&models.FilesystemCapacity{},
		&models.FilesystemHostStatus{},
		&models.BtrfsFilesystem{},
		&models.BtrfsDevice{},
		&models.DeviceEnduranceOverride{},
		&models.DeviceSelfTest{},
		&models.NotificationOutbox{},
		&m20260924000000.SchedulerLease{},
	}
}
