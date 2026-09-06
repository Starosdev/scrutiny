package m20260906000000

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"gorm.io/gorm"
)

// Migrate adds the usable_used and usable_free columns to the zfs_pools table
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.ZFSPool{})
}
