package m20260910000000

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"gorm.io/gorm"
)

// Migrate adds timestamps used to distinguish missing pools from stale collectors.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.ZFSPool{})
}
